package graph

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/model"
)

// listMountedVolumes enumerates mounted storage volumes from /proc/mounts.
func listMountedVolumes() ([]*model.StorageMount, error) {
	volumes := []*model.StorageMount{}

	// Load alias mapping once
	aliasMap := db.GetVolumeAliasMap()

	devMeta := buildLsblkVolumeDevMetaMap()

	file, err := os.Open("/proc/mounts")
	if err != nil {
		return volumes, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	seenSrc := map[string]struct{}{}
	byTarget := map[string]*model.StorageMount{}
	targetOrder := make([]string, 0, 16)
	skipTypes := map[string]struct{}{
		"proc": {}, "sysfs": {}, "devpts": {}, "tmpfs": {}, "devtmpfs": {}, "securityfs": {},
		"cgroup": {}, "cgroup2": {}, "pstore": {}, "bpf": {}, "overlay": {}, "squashfs": {},
		"autofs": {}, "mqueue": {}, "hugetlbfs": {}, "ramfs": {}, "fusectl": {}, "debugfs": {},
		"tracefs": {}, "configfs": {}, "efivarfs": {},
	}

	// System mountpoints to hide from the user-facing "Volumes" concept.
	skipMountPoints := map[string]struct{}{
		"/boot":          {},
		"/boot/efi":      {},
		"/efi":           {},
		"/boot/firmware": {},
	}

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		src := parts[0]
		target := parts[1]
		fstype := parts[2]
		opts := ""
		if len(parts) > 3 {
			opts = parts[3]
		}

		if _, skip := skipTypes[fstype]; skip {
			continue
		}
		_, targetSeen := byTarget[target]

		// Hide system mountpoints that add noise for NAS users.
		if _, skip := skipMountPoints[target]; skip {
			continue
		}

		// Skip bind mounts: they are not standalone storage volumes
		if opts != "" && strings.Contains(opts, "bind") {
			continue
		}

		isBlock := strings.HasPrefix(src, "/dev/")
		isRemote := strings.Contains(src, ":") || strings.HasPrefix(fstype, "nfs") || strings.Contains(fstype, "cifs") || strings.Contains(fstype, "smb") || strings.Contains(fstype, "sshfs")
		if !isBlock && !isRemote {
			continue
		}

		// Ignore RAM-backed or loop devices which often represent ephemeral/system mounts
		if strings.HasPrefix(src, "/dev/zram") || strings.HasPrefix(src, "/dev/loop") {
			continue
		}

		// Dedupe multiple mount points of the same block device (e.g., bind mounts)
		if isBlock {
			if _, ok := seenSrc[src]; ok {
				continue
			}
		}

		var stat syscall.Statfs_t
		if err := syscall.Statfs(target, &stat); err != nil {
			continue
		}
		bsize := uint64(stat.Bsize)
		total := uint64(stat.Blocks) * bsize
		free := uint64(stat.Bfree) * bsize
		used := total - free

		// Stable ID
		id := ""
		fsUUID := ""
		devResolved := ""
		if isRemote {
			id = volumeIDRemotePrefix + src
		} else if strings.HasPrefix(src, "/dev/") {
			resolved, err := filepath.EvalSymlinks(src)
			if err != nil || resolved == "" {
				resolved = src
			}
			devResolved = resolved
			if m, ok := devMeta[devResolved]; ok {
				fsUUID = strings.TrimSpace(m.UUID)
			}
			if fsUUID != "" {
				id = volumeIDFSUUIDPrefix + fsUUID
			} else {
				id = volumeIDDevPrefix + devResolved
			}
		}
		// Fallback: still return something non-empty even if UUID resolution fails.
		if id == "" {
			id = src
		}

		mp := target
		ft := fstype
		used64 := int64(used)
		free64 := int64(free)
		dt := strings.TrimSpace(getDriveType(src, isRemote))
		var dtPtr *string
		if dt != "" {
			dtt := dt
			dtPtr = &dtt
		}

		v := &model.StorageMount{
			ID:         id,
			Name:       filepath.Base(target),
			MountPoint: &mp,
			FsType:     &ft,
			TotalBytes: int64(total),
			UsedBytes:  &used64,
			FreeBytes:  &free64,
			Remote:     isRemote,
			DriveType:  dtPtr,
		}
		if fsUUID != "" {
			u := fsUUID
			v.UUID = &u
		}

		if isBlock {
			applyVolumeBlockMeta(v, src, devMeta)
		}

		if a, ok := aliasMap[v.ID]; ok && strings.TrimSpace(a) != "" {
			val := a
			v.Alias = &val
		}

		// Keep the last mount for a given mountpoint. This matters when a mountpoint
		// is reused and older mounts are hidden but still listed in /proc/mounts.
		if !targetSeen {
			targetOrder = append(targetOrder, target)
		}
		byTarget[target] = v
		if isBlock {
			seenSrc[src] = struct{}{}
		}
	}

	// Preserve first-seen mountpoint ordering, but with the latest mount info.
	for _, target := range targetOrder {
		if v := byTarget[target]; v != nil {
			volumes = append(volumes, v)
		}
	}

	return volumes, nil
}
