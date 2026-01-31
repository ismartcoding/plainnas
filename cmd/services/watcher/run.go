package watcher

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"ismartcoding/plainnas/internal/graph"
	"ismartcoding/plainnas/internal/media"
	"ismartcoding/plainnas/internal/search"
)

func isPlainNASUSBMount(mp string) bool {
	mp = filepath.Clean(strings.TrimSpace(mp))
	if !strings.HasPrefix(mp, "/mnt/usb") {
		return false
	}
	n := strings.TrimPrefix(mp, "/mnt/usb")
	if n == "" {
		return false
	}
	// Must be /mnt/usb<positive int> (no extra path segments).
	if strings.Contains(n, "/") {
		return false
	}
	idx, err := strconv.Atoi(n)
	return err == nil && idx > 0
}

func Run(ctx context.Context) {
	// Build file index at startup from storage volumes in background
	go func() {
		// Build missing indexes at startup. If either index is missing, (re)build it.
		fileIdxExists := search.IndexExists()
		mediaIdxExists := media.MediaIndexExists()
		if fileIdxExists && mediaIdxExists {
			return
		}
		if vols, err := graph.ListMounts(); err == nil {
			roots := make([]string, 0, len(vols))
			for _, v := range vols {
				if v.MountPoint == nil {
					continue
				}
				mp := strings.TrimSpace(*v.MountPoint)
				if mp != "" && isPlainNASUSBMount(mp) {
					roots = append(roots, mp)
				}
			}
			if len(roots) == 0 {
				return
			}
			if !fileIdxExists {
				_ = search.IndexPaths(roots, false)
			}
			if !mediaIdxExists {
				for _, r := range roots {
					_ = media.ScanAndSync(r)
				}
				_ = media.BuildMediaIndex()
			}
		}
	}()
}
