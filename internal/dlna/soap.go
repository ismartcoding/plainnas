package dlna

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ismartcoding/plainnas/internal/pkg/log"
)

func soapAVTransport(dev discoveredDevice, action string, paramsXML string) error {
	svc := dev.AVTransport
	st := strings.TrimSpace(svc.ServiceType)
	if st == "" {
		return errors.New("missing AVTransport serviceType")
	}
	endpoint, err := resolveServiceEndpoint(dev.Location, svc.ControlURL)
	if err != nil {
		return err
	}
	start := time.Now()

	body := "<u:" + action + " xmlns:u=\"" + st + "\">" + paramsXML + "</u:" + action + ">"
	envelope := "<?xml version=\"1.0\" encoding=\"utf-8\"?>" +
		"<s:Envelope s:encodingStyle=\"http://schemas.xmlsoap.org/soap/encoding/\" xmlns:s=\"http://schemas.xmlsoap.org/soap/envelope/\">" +
		"<s:Body>" + body + "</s:Body>" +
		"</s:Envelope>"

	// IMPORTANT: do not bind a caller context to the HTTP request.
	// Some renderers are slow to respond; rely on http.Client timeouts instead.
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(envelope))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")
	req.Header.Set("SOAPAction", "\""+st+"#"+action+"\"")
	req.Header.Set("Connection", "close")

	client := &http.Client{Timeout: 3 * time.Second}
	if dlnaDebugEnabled() {
		log.Infof("[DLNA] soap: tx action=%s endpoint=%s bytes=%d", action, endpoint, len(envelope))
		if dlnaDebugPayloadEnabled() {
			log.Infof("[DLNA] soap: txBody action=%s\n%s", action, truncateForLog(envelope, 4096))
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return fmt.Errorf("soap %s timeout after %s: %w", action, client.Timeout, err)
		}
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if dlnaDebugEnabled() {
		log.Infof("[DLNA] soap: rx action=%s status=%d bytes=%d dur=%s", action, resp.StatusCode, len(respBody), time.Since(start))
		if dlnaDebugPayloadEnabled() {
			log.Infof("[DLNA] soap: rxBody action=%s\n%s", action, truncateForLog(string(respBody), 4096))
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("soap %s http %d", action, resp.StatusCode)
	}
	return nil
}

func resolveServiceEndpoint(location string, controlURL string) (string, error) {
	loc, err := url.Parse(strings.TrimSpace(location))
	if err != nil {
		return "", err
	}
	refStr := strings.TrimSpace(controlURL)
	if refStr == "" {
		return "", errors.New("empty controlURL")
	}
	ref, err := url.Parse(refStr)
	if err != nil {
		return "", err
	}
	// If device returns host-less absolute path, ResolveReference will do the right thing.
	end := loc.ResolveReference(ref)
	if end.Host == "" {
		return "", errors.New("invalid resolved endpoint")
	}
	// Normalize default ports if URL parser leaves them empty.
	if end.Port() == "" {
		// Some devices omit port in LOCATION but still reachable on default.
		// Keep as-is; http.Client will handle it.
	}
	return end.String(), nil
}
