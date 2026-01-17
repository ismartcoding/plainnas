package graph

import (
	"os"
	"strings"

	plainfs "ismartcoding/plainnas/internal/fs"
	"ismartcoding/plainnas/internal/media"
)

func restoreFiles(paths []string) (bool, error) {
	if len(paths) == 0 {
		return true, nil
	}
	restored, err := plainfs.RestorePaths(paths)
	if err != nil {
		return false, err
	}
	for _, p := range restored {
		if strings.TrimSpace(p) == "" {
			continue
		}
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			continue
		}
		_ = media.ScanFile(p)
	}
	return true, nil
}
