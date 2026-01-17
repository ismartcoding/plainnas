package db

import (
	"path/filepath"
	"strconv"
	"strings"
)

type SambaShareAccess string

const (
	SambaShareAccessAnyoneWrite SambaShareAccess = "ANYONE_WRITE" // guest + read-write
	SambaShareAccessAnyoneRead  SambaShareAccess = "ANYONE_READ"  // guest + read-only
	SambaShareAccessPassword    SambaShareAccess = "PASSWORD"     // password + read-write (legacy)
)

// New model (multi shares)

type SambaShareAuth string

const (
	SambaShareAuthGuest    SambaShareAuth = "GUEST"
	SambaShareAuthPassword SambaShareAuth = "PASSWORD"
)

type SambaShare struct {
	Name     string         `json:"name"`
	SharePath string        `json:"share_path"`
	Auth     SambaShareAuth `json:"auth"`
	ReadOnly bool           `json:"read_only"`
}

type SambaSettings struct {
	Enabled     bool        `json:"enabled"`
	Shares      []SambaShare `json:"shares"`
	HasPassword bool        `json:"has_password"`
}

func sambaSettingsKey() string {
	return "settings:samba"
}

func GetSambaSettings() SambaSettings {
	var s SambaSettings
	_ = GetDefault().LoadJSON(sambaSettingsKey(), &s)
	return normalizeSambaSettings(s)
}

func StoreSambaSettings(s SambaSettings) error {
	s = normalizeSambaSettings(s)
	return GetDefault().StoreJSON(sambaSettingsKey(), s)
}

func normalizeSambaSettings(s SambaSettings) SambaSettings {
	// Normalize shares
	seen := map[string]int{}
	out := make([]SambaShare, 0, len(s.Shares))
	for _, sh := range s.Shares {
		sh.Name = strings.TrimSpace(sh.Name)
		sh.SharePath = normalizeAbsPath(sh.SharePath)
		sh.Auth = normalizeSambaShareAuthV2(sh.Auth)
		// Default to read-only for safety.
		// (UI can explicitly set read-write.)
		// Keep sh.ReadOnly as-is.
		if sh.SharePath == "" {
			continue
		}
		if sh.Name == "" {
			sh.Name = filepath.Base(sh.SharePath)
		}
		key := strings.ToLower(sh.Name)
		if n, ok := seen[key]; ok {
			n++
			seen[key] = n
			sh.Name = sh.Name + "-" + strconv.Itoa(n)
		} else {
			seen[key] = 1
		}
		out = append(out, sh)
	}
	s.Shares = out
	if len(s.Shares) == 0 {
		s.Enabled = false
	}
	if !s.HasPassword {
		// Keep HasPassword false unless explicitly set by password mutation.
	}
	return s
}

func normalizeSambaShareAuthV2(a SambaShareAuth) SambaShareAuth {
	switch a {
	case SambaShareAuthGuest, SambaShareAuthPassword:
		return a
	default:
		return SambaShareAuthGuest
	}
}

func normalizeAbsPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(filepath.Clean(p))
	if p == "." {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		return ""
	}
	return p
}

func normalizeSambaShareAccess(a SambaShareAccess) SambaShareAccess {
	switch a {
	case SambaShareAccessAnyoneWrite, SambaShareAccessAnyoneRead, SambaShareAccessPassword:
		return a
	default:
		return SambaShareAccessAnyoneRead
	}
}
