package graph

import (
	"path/filepath"

	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/media"
	"ismartcoding/plainnas/internal/pkg/eventbus"
)

func stopMediaScan() {
	media.StopScan()
	indexed, total, _ := media.GetProgress()
	eventbus.GetDefault().Publish(consts.EVENT_MEDIA_SCAN_PROGRESS, map[string]any{
		"indexed": indexed,
		"pending": func() int64 {
			if total-indexed > 0 {
				return total - indexed
			}
			return 0
		}(),
		"total": total,
		"state": "stopped",
	})
}

func rebuildMediaIndex(root string) {
	media.StopScan()
	_ = media.ResetAllMediaData()
	media.ResumeScan()
	eventbus.GetDefault().Publish(consts.EVENT_MEDIA_SCAN_PROGRESS, map[string]any{
		"indexed": int64(0),
		"pending": int64(0),
		"total":   int64(0),
		"state":   "running",
		"root":    filepath.ToSlash(root),
	})
	go media.ScanAndSync(root)
}
