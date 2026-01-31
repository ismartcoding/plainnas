package dlna

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/pkg/eventbus"
)

type rendererDiscoveryManager struct {
	mu      sync.Mutex
	running bool
	clients map[string]struct{}
}

type rendererCache struct {
	mu    sync.RWMutex
	byUDN map[string]discoveredDevice
}

var (
	discoveryMgr = &rendererDiscoveryManager{}
	cache        = &rendererCache{byUDN: map[string]discoveredDevice{}}
)

// StartRendererDiscovery starts (or joins) a background discovery task.
// The task runs for up to 1 minute and publishes incremental results to the
// requesting client via websocket events.
func StartRendererDiscovery(clientID string) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return
	}

	// Register client + immediately flush current cache.
	discoveryMgr.mu.Lock()
	if discoveryMgr.clients == nil {
		discoveryMgr.clients = make(map[string]struct{})
	}
	discoveryMgr.clients[clientID] = struct{}{}
	alreadyRunning := discoveryMgr.running
	if !discoveryMgr.running {
		discoveryMgr.running = true
	}
	discoveryMgr.mu.Unlock()

	flushCachedToClient(clientID)

	if alreadyRunning {
		return
	}

	go runRendererDiscoveryTask()
}

func flushCachedToClient(clientID string) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	for _, d := range cache.byUDN {
		if !d.HasAVTransport || d.UDN == "" {
			continue
		}
		eventbus.GetDefault().Publish(consts.EVENT_DLNA_RENDERER_FOUND, clientID, rendererPayload(d))
	}
}

func runRendererDiscoveryTask() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	onDevice := func(d discoveredDevice) {
		if !d.HasAVTransport || strings.TrimSpace(d.UDN) == "" {
			return
		}
		udn := strings.TrimSpace(d.UDN)

		// De-dup and cache.
		cache.mu.Lock()
		_, existed := cache.byUDN[udn]
		cache.byUDN[udn] = d
		cache.mu.Unlock()

		if existed {
			return
		}

		// Publish to all currently registered clients.
		discoveryMgr.mu.Lock()
		clients := make([]string, 0, len(discoveryMgr.clients))
		for cid := range discoveryMgr.clients {
			clients = append(clients, cid)
		}
		discoveryMgr.mu.Unlock()

		payload := rendererPayload(d)
		for _, cid := range clients {
			eventbus.GetDefault().Publish(consts.EVENT_DLNA_RENDERER_FOUND, cid, payload)
		}
	}

	// Keep scanning within the 1-minute window.
	for ctx.Err() == nil {
		roundCtx, cancelRound := context.WithTimeout(ctx, 10*time.Second)
		_, _ = discoverUPnPDevicesWithCallback(roundCtx, onDevice)
		cancelRound()

		select {
		case <-time.After(1200 * time.Millisecond):
		case <-ctx.Done():
		}
	}

	// Notify clients that the discovery window is done.
	discoveryMgr.mu.Lock()
	clients := make([]string, 0, len(discoveryMgr.clients))
	for cid := range discoveryMgr.clients {
		clients = append(clients, cid)
	}
	discoveryMgr.clients = nil
	discoveryMgr.running = false
	discoveryMgr.mu.Unlock()

	for _, cid := range clients {
		eventbus.GetDefault().Publish(consts.EVENT_DLNA_DISCOVERY_DONE, cid, map[string]any{"done": true})
	}
}

func rendererPayload(d discoveredDevice) map[string]any {
	return map[string]any{
		"udn":          d.UDN,
		"name":         d.FriendlyName,
		"manufacturer": d.Manufacturer,
		"modelName":    d.ModelName,
		"location":     d.Location,
	}
}

// CachedRenderers returns the latest cached renderers sorted by name.
func CachedRenderers() []Renderer {
	cache.mu.RLock()
	devs := make([]discoveredDevice, 0, len(cache.byUDN))
	for _, d := range cache.byUDN {
		devs = append(devs, d)
	}
	cache.mu.RUnlock()

	out := make([]Renderer, 0, len(devs))
	for _, d := range devs {
		if !d.HasAVTransport || d.UDN == "" || d.FriendlyName == "" {
			continue
		}
		out = append(out, Renderer{
			UDN:          d.UDN,
			Name:         d.FriendlyName,
			Manufacturer: d.Manufacturer,
			ModelName:    d.ModelName,
			Location:     d.Location,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func getCachedDeviceByUDN(udn string) (discoveredDevice, bool) {
	udn = strings.TrimSpace(udn)
	if udn == "" {
		return discoveredDevice{}, false
	}
	cache.mu.RLock()
	d, ok := cache.byUDN[udn]
	cache.mu.RUnlock()
	return d, ok
}
