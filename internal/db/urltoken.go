package db

import (
	"crypto/rand"
	"encoding/base64"
	"sync"

	"github.com/cockroachdb/pebble"
)

var (
	urlTokenCache string
	urlTokenOnce  sync.Once
	urlTokenMu    sync.RWMutex
)

func ensureURLTokenLoaded() {
	// Try load existing
	if b, _ := GetDefault().Get([]byte("url_token")); b != nil {
		urlTokenCache = string(b)
		return
	}
	// Generate new 32-byte token (base64-encoded)
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	token := base64.StdEncoding.EncodeToString(buf)
	_ = GetDefault().Set([]byte("url_token"), []byte(token), &pebble.WriteOptions{Sync: true})
	urlTokenCache = token
}

// EnsureURLToken makes sure a global URL token exists in DB and cache.
func EnsureURLToken() {
	urlTokenOnce.Do(func() {
		ensureURLTokenLoaded()
	})
}

// GetURLToken returns the global URL token, loading it if needed.
func GetURLToken() string {
	urlTokenMu.RLock()
	v := urlTokenCache
	urlTokenMu.RUnlock()
	if v != "" {
		return v
	}
	urlTokenMu.Lock()
	defer urlTokenMu.Unlock()
	if urlTokenCache == "" {
		ensureURLTokenLoaded()
	}
	return urlTokenCache
}

// SetURLToken sets and persists the global URL token.
func SetURLToken(token string) error {
	if token == "" {
		return nil
	}
	if err := GetDefault().Set([]byte("url_token"), []byte(token), &pebble.WriteOptions{Sync: true}); err != nil {
		return err
	}
	urlTokenMu.Lock()
	urlTokenCache = token
	urlTokenMu.Unlock()
	return nil
}
