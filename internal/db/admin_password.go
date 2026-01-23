package db

import (
	"crypto/sha512"
	"encoding/hex"
	"strings"
	"sync"

	"github.com/cockroachdb/pebble"
)

const adminPasswordHashKey = "auth:admin_password_sha512"

var (
	adminPwdHashCache string
	adminPwdOnce      sync.Once
	adminPwdMu        sync.RWMutex
)

func ensureAdminPasswordLoaded() {
	if b, _ := GetDefault().Get([]byte(adminPasswordHashKey)); b != nil {
		adminPwdHashCache = strings.TrimSpace(string(b))
		return
	}
	adminPwdHashCache = ""
}

// EnsureAdminPasswordLoaded warms the in-process cache from DB.
func EnsureAdminPasswordLoaded() {
	adminPwdOnce.Do(func() {
		ensureAdminPasswordLoaded()
	})
}

// GetAdminPasswordHash returns the SHA-512(hex) of the admin password.
// Empty string means password is not configured.
func GetAdminPasswordHash() string {
	adminPwdMu.RLock()
	v := adminPwdHashCache
	adminPwdMu.RUnlock()
	if v != "" {
		return v
	}
	adminPwdMu.Lock()
	defer adminPwdMu.Unlock()
	if adminPwdHashCache == "" {
		ensureAdminPasswordLoaded()
	}
	return adminPwdHashCache
}

// HasAdminPassword returns true if an admin password hash exists.
func HasAdminPassword() bool {
	return strings.TrimSpace(GetAdminPasswordHash()) != ""
}

func hashPasswordSHA512Hex(plain string) string {
	plain = strings.TrimSpace(plain)
	s := sha512.New()
	s.Write([]byte(plain))
	return hex.EncodeToString(s.Sum(nil))
}

// SetAdminPassword stores the admin password as SHA-512(hex) in DB.
func SetAdminPassword(plain string) error {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return nil
	}
	h := hashPasswordSHA512Hex(plain)
	if err := GetDefault().Set([]byte(adminPasswordHashKey), []byte(h), &pebble.WriteOptions{Sync: true}); err != nil {
		return err
	}
	adminPwdMu.Lock()
	adminPwdHashCache = h
	adminPwdMu.Unlock()
	return nil
}

// SetAdminPasswordHash stores a precomputed SHA-512(hex) admin password hash.
func SetAdminPasswordHash(hash string) error {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil
	}
	if err := GetDefault().Set([]byte(adminPasswordHashKey), []byte(hash), &pebble.WriteOptions{Sync: true}); err != nil {
		return err
	}
	adminPwdMu.Lock()
	adminPwdHashCache = hash
	adminPwdMu.Unlock()
	return nil
}
