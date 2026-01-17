//go:build linux

package media

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

var uuidNamespace = [16]byte{0x6d, 0x65, 0x64, 0x69, 0x61, 0x2d, 0x6e, 0x61, 0x73, 0x2d, 0x69, 0x64, 0x00, 0x00, 0x00, 0x00}

// mountCache maps mountpoint -> filesystem identifier (prefer FSUUID when available).
// It is optimized for extremely hot paths during scans.
var mountCache struct {
	mu        sync.RWMutex
	lastBuild time.Time
	// mountpoints sorted by length desc (best match first)
	mountpoints []string
	// mountpoint -> filesystem id (prefer FSUUID)
	fsIDByMount map[string]string
}

func decodeFstabEscapes(s string) string {
	// /proc/mounts uses octal escapes like \040 for space.
	// Keep this minimal; we only decode the most common ones.
	repl := map[string]string{
		"\\040": " ",
		"\\011": "\t",
		"\\012": "\n",
		"\\134": "\\",
	}
	out := s
	for k, v := range repl {
		out = strings.ReplaceAll(out, k, v)
	}
	return out
}

func buildResolvedDevPathToUUIDMap() map[string]string {
	m := map[string]string{}
	entries, err := os.ReadDir("/dev/disk/by-uuid")
	if err != nil {
		return m
	}
	for _, e := range entries {
		uuid := e.Name()
		if uuid == "" {
			continue
		}
		linkPath := filepath.Join("/dev/disk/by-uuid", uuid)
		resolved, err := filepath.EvalSymlinks(linkPath)
		if err != nil || resolved == "" {
			continue
		}
		m[resolved] = uuid
	}
	return m
}

func rebuildMountCacheLocked() {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		mountCache.mountpoints = []string{string(filepath.Separator)}
		mountCache.fsIDByMount = map[string]string{string(filepath.Separator): ""}
		mountCache.lastBuild = time.Now()
		return
	}
	defer file.Close()

	devToUUID := buildResolvedDevPathToUUIDMap()
	fsIDByMount := make(map[string]string, 64)
	mountSet := make(map[string]struct{}, 64)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		src := decodeFstabEscapes(parts[0])
		mp := decodeFstabEscapes(parts[1])
		if mp == "" {
			continue
		}
		mp = filepath.Clean(mp)
		if mp == "" {
			continue
		}
		// Prefer first-seen mapping for a mountpoint.
		if _, ok := fsIDByMount[mp]; ok {
			continue
		}

		fsid := ""
		if strings.HasPrefix(src, "/dev/") {
			devResolved, err := filepath.EvalSymlinks(src)
			if err != nil || devResolved == "" {
				devResolved = src
			}
			if uuid := devToUUID[devResolved]; uuid != "" {
				fsid = uuid
			} else {
				// Best-effort fallback when FSUUID cannot be resolved.
				fsid = devResolved
			}
		} else {
			// Remote/virtual mounts: keep a stable identifier.
			fsid = src
		}

		fsIDByMount[mp] = fsid
		mountSet[mp] = struct{}{}
	}

	// Ensure root mountpoint always exists.
	root := string(filepath.Separator)
	if _, ok := mountSet[root]; !ok {
		mountSet[root] = struct{}{}
		if _, ok := fsIDByMount[root]; !ok {
			fsIDByMount[root] = ""
		}
	}

	mounts := make([]string, 0, len(mountSet))
	for mp := range mountSet {
		mounts = append(mounts, mp)
	}
	sort.Slice(mounts, func(i, j int) bool { return len(mounts[i]) > len(mounts[j]) })

	mountCache.mountpoints = mounts
	mountCache.fsIDByMount = fsIDByMount
	mountCache.lastBuild = time.Now()
}

func filesystemIDForPath(path string) (string, error) {
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return "", fmt.Errorf("invalid path")
	}
	if !filepath.IsAbs(path) {
		p, err := filepath.Abs(path)
		if err == nil {
			path = filepath.Clean(p)
		}
	}

	// Refresh cache periodically to follow mount changes.
	const ttl = 30 * time.Second
	mountCache.mu.RLock()
	fresh := mountCache.fsIDByMount != nil && time.Since(mountCache.lastBuild) <= ttl
	if fresh {
		mounts := mountCache.mountpoints
		fsIDByMount := mountCache.fsIDByMount
		mountCache.mu.RUnlock()
		sep := string(filepath.Separator)
		for _, mp := range mounts {
			if mp == sep {
				return fsIDByMount[mp], nil
			}
			if path == mp || strings.HasPrefix(path, mp+sep) {
				return fsIDByMount[mp], nil
			}
		}
		// Should never happen because '/' is always present.
		return "", fmt.Errorf("unable to resolve filesystem id")
	}
	mountCache.mu.RUnlock()

	mountCache.mu.Lock()
	defer mountCache.mu.Unlock()
	// Another goroutine may have refreshed already.
	if mountCache.fsIDByMount == nil || time.Since(mountCache.lastBuild) > ttl {
		rebuildMountCacheLocked()
	}

	mounts := mountCache.mountpoints
	fsIDByMount := mountCache.fsIDByMount
	sep := string(filepath.Separator)
	for _, mp := range mounts {
		if mp == sep {
			return fsIDByMount[mp], nil
		}
		if path == mp || strings.HasPrefix(path, mp+sep) {
			return fsIDByMount[mp], nil
		}
	}
	return "", fmt.Errorf("unable to resolve filesystem id")
}

func fileIdentity(path string) (uint64, int64, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return 0, 0, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return 0, 0, fmt.Errorf("unsupported fileinfo stat on %s", path)
	}
	ino := uint64(st.Ino)
	ctimeSec := st.Ctim.Sec
	return ino, ctimeSec, nil
}

func uuidFromTriplet(fsUUID string, ino uint64, ctime int64) string {
	// fsUUID should be the filesystem UUID when available.
	s := fmt.Sprintf("%s:%d:%d", fsUUID, ino, ctime)
	h := sha1.New()
	_, _ = h.Write(uuidNamespace[:])
	_, _ = h.Write([]byte(s))
	sum := h.Sum(nil)
	b := make([]byte, 16)
	copy(b, sum)
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	x := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", x[0:8], x[8:12], x[12:16], x[16:20], x[20:32])
}

func GenerateUUIDFromPath(path string) (string, string, uint64, int64, error) {
	ino, ctime, err := fileIdentity(path)
	if err != nil {
		return "", "", 0, 0, err
	}
	fsid, err := filesystemIDForPath(path)
	if err != nil {
		return "", "", 0, 0, err
	}
	if fsid == "" {
		// As a last resort, use the mountpoint path; still stable within a running system.
		fsid = string(filepath.Separator)
	}
	id := uuidFromTriplet(fsid, ino, ctime)
	return id, fsid, ino, ctime, nil
}
