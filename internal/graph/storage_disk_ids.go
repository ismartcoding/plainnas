package graph

import (
	"os"
	"path/filepath"
	"strings"
)

const diskIDPrefix = "disk:"
const diskIDByIDPrefix = "diskbyid:"

// diskIDFromName returns a stable identifier for a disk.
//
// Prefer /dev/disk/by-id (stable across /dev/sdX renames) when available.
// Falls back to kernel disk name if no by-id link exists.
func diskIDFromName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	devPath := filepath.Join("/dev", name)
	if byID := bestByIDNameForDevPath(devPath); byID != "" {
		return diskIDByIDPrefix + byID
	}
	return diskIDPrefix + name
}

func bestByIDNameForDevPath(devPath string) string {
	devPath = strings.TrimSpace(devPath)
	if devPath == "" {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(devPath)
	if err != nil || resolved == "" {
		resolved = devPath
	}

	entries, err := os.ReadDir("/dev/disk/by-id")
	if err != nil {
		return ""
	}

	best := ""
	bestScore := -1
	for _, e := range entries {
		name := strings.TrimSpace(e.Name())
		if name == "" {
			continue
		}
		// Ignore partition-specific links.
		if strings.Contains(name, "-part") {
			continue
		}
		link := filepath.Join("/dev/disk/by-id", name)
		target, err := filepath.EvalSymlinks(link)
		if err != nil || target == "" {
			continue
		}
		if target != resolved {
			continue
		}

		score := scoreByIDName(name)
		if score > bestScore {
			bestScore = score
			best = name
		}
	}
	return best
}

func scoreByIDName(name string) int {
	// Prefer globally stable IDs.
	if strings.HasPrefix(name, "wwn-") {
		return 50
	}
	if strings.HasPrefix(name, "nvme-") {
		return 40
	}
	if strings.HasPrefix(name, "ata-") {
		return 30
	}
	if strings.HasPrefix(name, "scsi-") {
		return 20
	}
	if strings.HasPrefix(name, "usb-") {
		return 10
	}
	return 0
}
