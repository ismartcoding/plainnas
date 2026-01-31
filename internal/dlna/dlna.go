package dlna

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"

	"ismartcoding/plainnas/internal/config"
	plainfs "ismartcoding/plainnas/internal/fs"
	"ismartcoding/plainnas/internal/pkg/log"
)

func dlnaSafeMediaURL(inputURL string, mime string) string {
	u, err := url.Parse(inputURL)
	if err != nil {
		return inputURL
	}

	// Only rewrite our own file-serving endpoint.
	if u.Path != "/fs" {
		return inputURL
	}
	id := strings.TrimSpace(u.Query().Get("id"))
	if id == "" {
		return inputURL
	}

	path, err := plainfs.PathFromFileID(id)
	if err != nil || path == "" {
		return inputURL
	}

	aliasID, ext := registerMediaAlias(path, mime)
	if aliasID == "" {
		return inputURL
	}

	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return inputURL
	}

	httpPort := strings.TrimSpace(config.GetDefault().GetString("server.http_port"))
	if httpPort == "" && !strings.EqualFold(u.Scheme, "http") {
		return inputURL
	}
	hostPort := ""
	if httpPort != "" {
		hostPort = net.JoinHostPort(host, httpPort)
	} else if strings.EqualFold(u.Scheme, "http") && u.Host != "" {
		hostPort = u.Host
	} else {
		// Fallback: best-effort (IPv6 without port needs brackets).
		if strings.Contains(host, ":") {
			hostPort = "[" + host + "]"
		} else {
			hostPort = host
		}
	}

	return "http://" + hostPort + "/media/" + aliasID + "." + ext
}

func DiscoverRenderers(ctx context.Context) ([]Renderer, error) {
	// PlainAPP approach: send SSDP M-SEARCH with 2 empty lines and parse responses.
	// Many TVs are picky about request formatting.
	devs, err := discoverUPnPDevices(ctx)
	if err != nil {
		return nil, err
	}
	log.Infof("[DLNA] discover: found %d UPnP devices (pre-filter)", len(devs))

	byUDN := make(map[string]Renderer)
	for _, d := range devs {
		if d.UDN == "" || d.FriendlyName == "" {
			continue
		}
		if !d.HasAVTransport {
			continue
		}

		byUDN[d.UDN] = Renderer{
			UDN:          d.UDN,
			Name:         d.FriendlyName,
			Manufacturer: d.Manufacturer,
			ModelName:    d.ModelName,
			Location:     d.Location,
		}
	}

	out := make([]Renderer, 0, len(byUDN))
	for _, r := range byUDN {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func Cast(ctx context.Context, rendererUDN string, mediaURL string, title string, mime string, mediaType MediaType) error {
	rendererUDN = strings.TrimSpace(rendererUDN)
	if rendererUDN == "" {
		return fmt.Errorf("renderer UDN is empty")
	}
	mediaURL = dlnaSafeMediaURL(mediaURL, mime)
	if _, err := url.ParseRequestURI(mediaURL); err != nil {
		return fmt.Errorf("invalid url")
	}
	if dlnaDebugEnabled() {
		log.Infof("[DLNA] cast: udn=%s mediaURL=%s mime=%s type=%s title=%s", rendererUDN, mediaURL, strings.TrimSpace(mime), mediaType, strings.TrimSpace(title))
	}

	meta, err := didlLiteMetadata(mediaURL, title, mime, mediaType)
	if err != nil {
		return err
	}

	dev, err := findUPnPDeviceByUDN(ctx, rendererUDN)
	if err != nil {
		return err
	}
	if !dev.HasAVTransport || dev.AVTransport.ServiceType == "" {
		return fmt.Errorf("renderer has no AVTransport")
	}
	if dlnaDebugEnabled() {
		log.Infof("[DLNA] cast: renderer=%s (%s) avTransport.controlURL=%s location=%s", dev.FriendlyName, dev.UDN, dev.AVTransport.ControlURL, dev.Location)
	}

	// Some renderers behave better if we Stop first.
	_ = soapAVTransport(dev, "Stop", "<InstanceID>0</InstanceID>")

	// Set URI + metadata, then Play.
	setBody := "<InstanceID>0</InstanceID>" +
		"<CurrentURI>" + xmlEscape(mediaURL) + "</CurrentURI>" +
		"<CurrentURIMetaData>" + xmlEscape(meta) + "</CurrentURIMetaData>"
	if err := soapAVTransport(dev, "SetAVTransportURI", setBody); err != nil {
		return err
	}

	if err := soapAVTransport(dev, "Play", "<InstanceID>0</InstanceID><Speed>1</Speed>"); err != nil {
		return err
	}
	return nil
}
