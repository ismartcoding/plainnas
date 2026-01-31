package graph

import (
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/graph/model"
)

// listDiskPartitions returns partitions discovered from lsblk. Entries returned
// from here use an ID namespace prefix ("part:") to avoid colliding with mounted
// volume IDs.
func listDiskPartitions() ([]*model.StorageMount, error) {
	parsed, err := runLsblkJSON([]string{
		"NAME", "PATH", "TYPE", "SIZE", "FSTYPE", "UUID", "LABEL", "MOUNTPOINT", "PKNAME", "PARTN",
	})
	if err != nil || parsed == nil {
		return []*model.StorageMount{}, err
	}

	parts := make([]*model.StorageMount, 0, 32)
	for _, d := range parsed.BlockDevices {
		if strings.TrimSpace(d.Type) != "disk" {
			continue
		}
		if !isUserVisibleDiskName(strings.TrimSpace(d.Name)) {
			continue
		}
		diskID := diskIDFromName(strings.TrimSpace(d.Name))
		for _, c := range d.Children {
			if strings.TrimSpace(c.Type) != "part" {
				continue
			}
			// Hide LVM physical volumes (PV) and other non-user-facing partition roles.
			// In PlainNAS UI we only treat actual filesystems (or LVM LV) as Volumes.
			if strings.EqualFold(strings.TrimSpace(c.Fstype), "LVM2_member") {
				continue
			}

			// Hide boot/EFI partitions from novice-facing UI.
			mp := strings.TrimSpace(c.Mountpoint)
			if mp == "/boot" || mp == "/boot/efi" || mp == "/efi" || mp == "/boot/firmware" {
				continue
			}
			name := strings.TrimSpace(c.Name)
			path := strings.TrimSpace(c.Path)
			if path == "" && name != "" {
				path = filepath.Join("/dev", name)
			}
			if strings.TrimSpace(path) == "" {
				continue
			}

			sizeBytes := int64(0)
			if v, err := c.SizeBytes.Int64(); err == nil {
				sizeBytes = v
			}
			// Hide tiny partitions without a filesystem (common GPT/BIOS/metadata partitions).
			if strings.TrimSpace(c.Fstype) == "" && sizeBytes > 0 && sizeBytes < 32*1024*1024 {
				continue
			}

			var fsTypePtr *string
			if v := strings.TrimSpace(c.Fstype); v != "" {
				vv := v
				fsTypePtr = &vv
			}
			var mountPointPtr *string
			if v := strings.TrimSpace(c.Mountpoint); v != "" {
				vv := v
				mountPointPtr = &vv
			}
			var labelPtr *string
			if v := strings.TrimSpace(c.Label); v != "" {
				vv := v
				labelPtr = &vv
			}
			var uuidPtr *string
			if v := strings.TrimSpace(c.UUID); v != "" {
				vv := v
				uuidPtr = &vv
			}
			var diskIDPtr *string
			if diskID != "" {
				vv := diskID
				diskIDPtr = &vv
			}

			idSuffix := ""
			if uuidPtr != nil && strings.TrimSpace(*uuidPtr) != "" {
				idSuffix = strings.TrimSpace(*uuidPtr)
			} else {
				idSuffix = path
			}
			id := mountIDPartitionPrefix + idSuffix

			p := &model.StorageMount{
				ID:   id,
				Name: name,
				Path: func() *string { vv := path; return &vv }(),
				PartitionNum: func() *int {
					if n, ok := parseJSONInt(c.PartN); ok && n > 0 {
						nn := n
						return &nn
					}
					return nil
				}(),
				Label:      labelPtr,
				UUID:       uuidPtr,
				MountPoint: mountPointPtr,
				FsType:     fsTypePtr,
				TotalBytes: sizeBytes,
				Remote:     false,
				DiskID:     diskIDPtr,
			}
			parts = append(parts, p)
		}
	}

	return parts, nil
}
