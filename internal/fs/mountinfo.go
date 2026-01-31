//go:build linux

package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type mountInfoEntry struct {
	MountPoint string
}

func decodeMountEscapes(s string) string {
	// mountinfo uses octal escapes like \040 for space.
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

func readMountInfo() ([]mountInfoEntry, error) {
	b, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	out := make([]mountInfoEntry, 0, 64)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: https://www.kernel.org/doc/Documentation/filesystems/proc.txt
		// We only need the mount point (field 5).
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(parts[0])
		if len(fields) < 5 {
			continue
		}
		mp := decodeMountEscapes(fields[4])
		if mp == "" {
			continue
		}
		out = append(out, mountInfoEntry{MountPoint: mp})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no mountinfo entries")
	}
	return out, nil
}

func findBestMountPoint(entries []mountInfoEntry, absPath string) string {
	// `absPath` is expected to be cleaned and absolute.
	best := ""
	sep := string(filepath.Separator)
	for _, e := range entries {
		mp := filepath.Clean(e.MountPoint)
		if mp == "" {
			continue
		}

		matched := false
		if absPath == mp {
			matched = true
		} else if mp == sep {
			// Root mountpoint matches any absolute path.
			matched = filepath.IsAbs(absPath)
		} else {
			matched = strings.HasPrefix(absPath, mp+sep)
		}

		if matched {
			if len(mp) > len(best) {
				best = mp
			}
		}
	}
	return best
}

// resolveMountPoint returns the longest mountpoint prefix that contains absPath.
func resolveMountPoint(absPath string) (string, error) {
	absPath = filepath.Clean(absPath)
	if absPath == "" || absPath == "." {
		return "", fmt.Errorf("invalid path")
	}
	if !filepath.IsAbs(absPath) {
		p, err := filepath.Abs(absPath)
		if err == nil {
			absPath = filepath.Clean(p)
		}
	}
	entries, err := readMountInfo()
	if err != nil {
		return "", err
	}

	best := findBestMountPoint(entries, absPath)
	if best != "" {
		return best, nil
	}

	// Fallback: allow mountpoint resolution through symlinked path components
	// (e.g. `/DATA/...` where `/DATA` is a symlink to a real mount root).
	if realPath, err := filepath.EvalSymlinks(absPath); err == nil {
		realPath = filepath.Clean(realPath)
		if realPath != "" && realPath != "." && realPath != absPath {
			if best := findBestMountPoint(entries, realPath); best != "" {
				return best, nil
			}
		}
	}

	return "", fmt.Errorf("unable to resolve mountpoint")
}
