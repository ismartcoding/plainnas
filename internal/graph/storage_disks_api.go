//go:build linux

package graph

import (
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/graph/model"
)

func isUserVisibleDiskName(name string) bool {
	n := strings.TrimSpace(name)
	if n == "" {
		return false
	}
	// Hide virtual/system devices that confuse users in a NAS UI.
	if strings.HasPrefix(n, "zram") || strings.HasPrefix(n, "loop") || strings.HasPrefix(n, "ram") {
		return false
	}
	return true
}

func ListStorageDisks() ([]*model.StorageDisk, error) {
	parsed, err := runLsblkJSON([]string{
		"NAME", "PATH", "TYPE", "MODEL", "SIZE", "RM",
	})
	if err != nil || parsed == nil {
		return []*model.StorageDisk{}, err
	}

	items := make([]*model.StorageDisk, 0, len(parsed.BlockDevices))
	for _, d := range parsed.BlockDevices {
		if strings.TrimSpace(d.Type) != "disk" {
			continue
		}
		if !isUserVisibleDiskName(d.Name) {
			continue
		}

		size := int64(0)
		if v, err := d.SizeBytes.Int64(); err == nil {
			size = v
		}

		removable := false
		if rm, ok := parseLsblkBoolish(d.RM); ok {
			removable = rm
		} else if strings.TrimSpace(d.Path) != "" {
			removable = isDeviceRemovable(strings.TrimSpace(d.Path))
		}

		name := strings.TrimSpace(d.Name)
		path := strings.TrimSpace(d.Path)
		if path == "" && name != "" {
			path = filepath.Join("/dev", name)
		}

		m := strings.TrimSpace(d.Model)
		if sysModel := readSysBlockModel(name); sysModel != nil {
			m = *sysModel
		}
		var modelPtr *string
		if strings.TrimSpace(m) != "" {
			mm := strings.TrimSpace(m)
			modelPtr = &mm
		}

		items = append(items, &model.StorageDisk{
			Name:      name,
			Path:      path,
			SizeBytes: size,
			Removable: removable,
			Model:     modelPtr,
		})
	}

	return items, nil
}
