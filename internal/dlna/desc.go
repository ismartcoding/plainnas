package dlna

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ismartcoding/plainnas/internal/pkg/log"
)

type upnpService struct {
	ServiceType string `xml:"serviceType"`
	ServiceID   string `xml:"serviceId"`
	ControlURL  string `xml:"controlURL"`
	EventSubURL string `xml:"eventSubURL"`
	SCPDURL     string `xml:"SCPDURL"`
}

type upnpDeviceDesc struct {
	FriendlyName string        `xml:"friendlyName"`
	Manufacturer string        `xml:"manufacturer"`
	ModelName    string        `xml:"modelName"`
	UDN          string        `xml:"UDN"`
	Services     []upnpService `xml:"serviceList>service"`
}

type upnpRootDesc struct {
	Device upnpDeviceDesc `xml:"device"`
}

type discoveredDevice struct {
	UDN            string
	FriendlyName   string
	Manufacturer   string
	ModelName      string
	Location       string
	HasAVTransport bool
	AVTransport    upnpService
}

func discoverUPnPDevices(ctx context.Context) ([]discoveredDevice, error) {
	return discoverUPnPDevicesWithCallback(ctx, nil)
}

func discoverUPnPDevicesWithCallback(ctx context.Context, onDevice func(discoveredDevice)) ([]discoveredDevice, error) {
	// PlainAPP sends a single ST=ssdp:all and waits ~5s.
	// Some devices respond only when the query is minimal, so we try that first
	// and only fall back to broader targets if needed.
	primarySTs := []string{"ssdp:all"}
	fallbackSTs := []string{
		"ssdp:all",
		"upnp:rootdevice",
		"urn:schemas-upnp-org:device:MediaRenderer:1",
		"urn:schemas-upnp-org:service:AVTransport:1",
	}
	if dlnaDebugEnabled() {
		log.Infof("[DLNA] ssdp: starting search ST=%s", strings.Join(primarySTs, ","))
	}
	responses, err := ssdpSearch(ctx, primarySTs)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		if dlnaDebugEnabled() {
			log.Infof("[DLNA] ssdp: no responses for primary ST; retrying with fallback ST=%s", strings.Join(fallbackSTs, ","))
		}
		responses, err = ssdpSearch(ctx, fallbackSTs)
		if err != nil {
			return nil, err
		}
	}
	if dlnaDebugEnabled() {
		log.Infof("[DLNA] ssdp: got %d unique responses", len(responses))
	}

	// Fetch and parse descriptions.
	byLocation := make(map[string]discoveredDevice)
	client := &http.Client{Timeout: 2 * time.Second}
	for _, r := range responses {
		loc := strings.TrimSpace(r.Location)
		if loc == "" {
			continue
		}
		if _, ok := byLocation[loc]; ok {
			continue
		}
		if dlnaDebugEnabled() {
			log.Infof("[DLNA] desc: fetching location=%s usn=%s st=%s", loc, strings.TrimSpace(r.USN), strings.TrimSpace(r.ST))
		}
		d, err := fetchAndParseDevice(ctx, client, loc)
		if err != nil {
			if dlnaDebugEnabled() {
				log.Infof("[DLNA] desc: fetch failed location=%s err=%v", loc, err)
			}
			continue
		}
		if d.UDN == "" {
			d.UDN = parseUDNFromUSN(r.USN)
		}
		if d.UDN == "" {
			continue
		}
		d.Location = loc
		if onDevice != nil {
			onDevice(d)
		}
		byLocation[loc] = d
	}

	out := make([]discoveredDevice, 0, len(byLocation))
	for _, d := range byLocation {
		out = append(out, d)
	}
	if dlnaDebugEnabled() {
		avc := 0
		for _, d := range out {
			if d.HasAVTransport {
				avc++
			}
		}
		log.Infof("[DLNA] discover: parsed %d device descriptions (%d with AVTransport)", len(out), avc)
	}
	return out, nil
}

func findUPnPDeviceByUDN(ctx context.Context, udn string) (discoveredDevice, error) {
	udn = strings.TrimSpace(udn)
	if udn == "" {
		return discoveredDevice{}, fmt.Errorf("udn is empty")
	}
	if d, ok := getCachedDeviceByUDN(udn); ok {
		return d, nil
	}
	devs, err := discoverUPnPDevices(ctx)
	if err != nil {
		return discoveredDevice{}, err
	}
	for _, d := range devs {
		if d.UDN == udn {
			return d, nil
		}
	}
	return discoveredDevice{}, fmt.Errorf("renderer not found")
}

func fetchAndParseDevice(ctx context.Context, client *http.Client, location string) (discoveredDevice, error) {
	start := time.Now()
	// IMPORTANT: do not bind the caller context to the HTTP request.
	// Discovery may run under a short/consumed ctx budget (e.g. UI-triggered polling);
	// relying on http.Client timeouts makes description fetches more reliable.
	req, err := http.NewRequest(http.MethodGet, location, nil)
	if err != nil {
		return discoveredDevice{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return discoveredDevice{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return discoveredDevice{}, fmt.Errorf("device description http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return discoveredDevice{}, err
	}
	if dlnaDebugEnabled() {
		log.Infof("[DLNA] desc: ok location=%s status=%d bytes=%d dur=%s", location, resp.StatusCode, len(b), time.Since(start))
		if dlnaDebugPayloadEnabled() {
			log.Infof("[DLNA] desc: body location=%s\n%s", location, truncateForLog(string(b), 4096))
		}
	}

	var root upnpRootDesc
	if err := xml.Unmarshal(b, &root); err != nil {
		return discoveredDevice{}, err
	}

	dd := root.Device
	name := strings.TrimSpace(dd.FriendlyName)
	if name == "" {
		name = strings.TrimSpace(dd.UDN)
	}

	out := discoveredDevice{
		UDN:            strings.TrimSpace(dd.UDN),
		FriendlyName:   name,
		Manufacturer:   strings.TrimSpace(dd.Manufacturer),
		ModelName:      strings.TrimSpace(dd.ModelName),
		HasAVTransport: false,
	}

	for _, s := range dd.Services {
		st := strings.TrimSpace(s.ServiceType)
		sid := strings.TrimSpace(s.ServiceID)
		if st == "urn:schemas-upnp-org:service:AVTransport:1" || sid == "urn:upnp-org:serviceId:AVTransport" {
			out.HasAVTransport = true
			out.AVTransport = s
			out.AVTransport.ServiceType = st
			out.AVTransport.ServiceID = sid
			out.AVTransport.ControlURL = strings.TrimSpace(s.ControlURL)
			out.AVTransport.EventSubURL = strings.TrimSpace(s.EventSubURL)
			out.AVTransport.SCPDURL = strings.TrimSpace(s.SCPDURL)
			break
		}
	}

	return out, nil
}
