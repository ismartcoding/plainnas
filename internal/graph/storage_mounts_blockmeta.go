package graph

import (
	"os"
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/graph/model"
)

type volumeDevMeta struct {
	PKName string
	Label  string
	UUID   string
}

func buildLsblkVolumeDevMetaMap() map[string]volumeDevMeta {
	// Best-effort: if lsblk isn't available or fails, we'll return empty mapping.
	m := map[string]volumeDevMeta{}

	parsed, err := runLsblkJSON([]string{"NAME", "PATH", "TYPE", "PKNAME", "PARTN", "LABEL", "UUID"})
	if err != nil || parsed == nil {
		return m
	}

	var walk func(d lsblkDevice)
	walk = func(d lsblkDevice) {
		path := strings.TrimSpace(d.Path)
		if path != "" {
			meta := volumeDevMeta{
				PKName: strings.TrimSpace(d.PKName),
				Label:  strings.TrimSpace(d.Label),
				UUID:   strings.TrimSpace(d.UUID),
			}
			m[path] = meta
			// Also index by resolved symlink target so callers can lookup via /dev/mapper/* or /dev/disk/by-*.
			if resolved, err := filepath.EvalSymlinks(path); err == nil && strings.TrimSpace(resolved) != "" {
				m[strings.TrimSpace(resolved)] = meta
			}
		}
		for _, c := range d.Children {
			walk(c)
		}
	}
	for _, d := range parsed.BlockDevices {
		walk(d)
	}

	return m
}

func applyVolumeBlockMeta(v *model.StorageMount, src string, meta map[string]volumeDevMeta) {
	if v == nil {
		return
	}
	// Resolve /dev/disk/by-uuid symlinks etc.
	resolved, err := filepath.EvalSymlinks(src)
	if err != nil || resolved == "" {
		resolved = src
	}
	if !strings.HasPrefix(resolved, "/dev/") {
		return
	}
	devName := strings.TrimPrefix(resolved, "/dev/")

	// Fill label from lsblk when available.
	if v.Label == nil {
		if m, ok := meta[resolved]; ok {
			lbl := strings.TrimSpace(m.Label)
			if lbl != "" {
				v.Label = &lbl
			}
		}
	}

	// Determine owning disk.
	diskName := ""
	if strings.HasPrefix(devName, "dm-") || strings.HasPrefix(devName, "md") {
		// Virtual devices: only set diskID when we can resolve a single underlying base disk.
		if pd, ok := resolveUnderlyingSingleBaseDisk(devName); ok {
			diskName = pd
		}
	} else if m, ok := meta[resolved]; ok {
		// Prefer lsblk PKNAME (e.g. partition -> disk); normalize any partition suffix.
		pk := strings.TrimSpace(m.PKName)
		if pk != "" {
			diskName = strings.TrimSpace(baseBlockDeviceName(pk))
		}
	}
	if diskName == "" {
		diskName = strings.TrimSpace(baseBlockDeviceName(devName))
	}

	if id := diskIDFromName(diskName); id != "" {
		vv := id
		v.DiskID = &vv
	}
}

func resolveUnderlyingSingleBaseDisk(devName string) (string, bool) {
	devName = strings.TrimSpace(devName)
	if devName == "" {
		return "", false
	}

	slavesDir := "/sys/class/block/" + devName + "/slaves"
	entries, err := os.ReadDir(slavesDir)
	if err != nil || len(entries) == 0 {
		return "", false
	}

	uniq := map[string]struct{}{}
	for _, e := range entries {
		n := strings.TrimSpace(e.Name())
		if n == "" {
			continue
		}

		// If the slave itself is another virtual device, try to resolve one level deeper.
		if strings.HasPrefix(n, "dm-") || strings.HasPrefix(n, "md") {
			if pd, ok := resolveUnderlyingSingleBaseDisk(n); ok {
				n = pd
			}
		}

		b := strings.TrimSpace(baseBlockDeviceName(n))
		if b == "" {
			b = n
		}
		uniq[b] = struct{}{}
	}

	if len(uniq) != 1 {
		return "", false
	}
	for k := range uniq {
		return k, true
	}
	return "", false
}
