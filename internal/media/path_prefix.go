package media

import (
	"path/filepath"
	"strings"
)

func pathHasDirPrefix(path string, dirPrefix string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	dirPrefix = filepath.ToSlash(filepath.Clean(dirPrefix))
	if dirPrefix == "" || dirPrefix == "." {
		return false
	}
	if dirPrefix == "/" {
		return true
	}
	if path == dirPrefix {
		return true
	}
	if !strings.HasPrefix(path, dirPrefix) {
		return false
	}
	if len(path) <= len(dirPrefix) {
		return true
	}
	return path[len(dirPrefix)] == '/'
}

// dirOverlapsAnyPrefix returns true when dir is within any allowed prefix, or is an ancestor
// of an allowed prefix (so walking should continue).
func dirOverlapsAnyPrefix(dir string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	dir = filepath.ToSlash(filepath.Clean(dir))
	for _, pre := range allowed {
		if pre == "" {
			continue
		}
		if pathHasDirPrefix(dir, pre) {
			return true
		}
		if pathHasDirPrefix(pre, dir) {
			return true
		}
	}
	return false
}

func pathInAnyPrefix(path string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, pre := range allowed {
		if pre == "" {
			continue
		}
		if pathHasDirPrefix(path, pre) {
			return true
		}
	}
	return false
}
