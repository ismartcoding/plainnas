package watcher

import (
	"context"
	"strings"

	"ismartcoding/plainnas/internal/graph"
	"ismartcoding/plainnas/internal/media"
	"ismartcoding/plainnas/internal/search"
)

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
				if v.Path != nil {
					continue
				}
				if v.MountPoint == nil {
					continue
				}
				mp := strings.TrimSpace(*v.MountPoint)
				if mp != "" {
					roots = append(roots, mp)
				}
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
