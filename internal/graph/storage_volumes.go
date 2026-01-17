package graph

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/model"
)

const volumeIDRemotePrefix = "remote:"

// ListStorageVolumes enumerates mounted storage volumes and enriches them with
// physical device capacity when possible to avoid misleading totals (e.g. LVM).
func ListStorageVolumes() ([]*model.StorageVolume, error) {
	volumes := []*model.StorageVolume{}

	// Load alias mapping once
	aliasMap := db.GetVolumeAliasMap()

	devToUUID := buildResolvedDevPathToUUIDMap()

	file, err := os.Open("/proc/mounts")
	if err != nil {
		return volumes, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	seenSrc := map[string]struct{}{}
	byTarget := map[string]*model.StorageVolume{}
	targetOrder := make([]string, 0, 16)
	skipTypes := map[string]struct{}{
		"proc": {}, "sysfs": {}, "devpts": {}, "tmpfs": {}, "devtmpfs": {}, "securityfs": {},
		"cgroup": {}, "cgroup2": {}, "pstore": {}, "bpf": {}, "overlay": {}, "squashfs": {},
		"autofs": {}, "mqueue": {}, "hugetlbfs": {}, "ramfs": {}, "fusectl": {}, "debugfs": {},
		"tracefs": {}, "configfs": {}, "efivarfs": {},
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

		// Skip dedicated boot partition to avoid tiny duplicate volume entries
		if target == "/boot" {
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

		removable := false
		if isBlock {
			removable = isDeviceRemovable(src)
		}

		// Stable ID
		id := ""
		if isRemote {
			id = volumeIDRemotePrefix + src
		} else if strings.HasPrefix(src, "/dev/") {
			devResolved, err := filepath.EvalSymlinks(src)
			if err != nil || devResolved == "" {
				devResolved = src
			}
			if uuid := devToUUID[devResolved]; uuid != "" {
				id = uuid
			}
		}
		// Fallback: still return something non-empty even if UUID resolution fails.
		if id == "" {
			id = src
		}

		v := &model.StorageVolume{
			ID:         id,
			Name:       filepath.Base(target),
			MountPoint: target,
			FsType:     fstype,
			TotalBytes: int64(total),
			UsedBytes:  int64(used),
			FreeBytes:  int64(free),
			Removable:  removable,
			Remote:     isRemote,
			DriveType:  getDriveType(src, isRemote),
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

// getUnderlyingPhysicalCapacityBytes resolves a block device path to its
// underlying physical partition(s) and returns their capacity in bytes. For
// device-mapper nodes (e.g. LVM), it sums the capacity of slave partitions.
func getUnderlyingPhysicalCapacityBytes(src string) uint64 {
	realPath, err := filepath.EvalSymlinks(src)
	if err != nil || realPath == "" {
		realPath = src
	}
	name := filepath.Base(realPath)

	// Device-mapper (e.g., /dev/dm-0) with underlying slaves
	if strings.HasPrefix(name, "dm-") {
		slavesDir := "/sys/class/block/" + name + "/slaves"
		entries, err := os.ReadDir(slavesDir)
		if err != nil {
			// Fallback to the dm-X size
			return readSysBlockSizeBytes(name)
		}
		var sum uint64
		for _, e := range entries {
			if !e.IsDir() {
				// entries in slaves are symlinks; use their names
				dev := e.Name()
				sum += readSysBlockSizeBytes(dev)
			}
		}
		if sum > 0 {
			return sum
		}
		return readSysBlockSizeBytes(name)
	}

	// Regular block or partition (e.g., sda, sda3, nvme0n1p3)
	return readSysBlockSizeBytes(name)
}

func readSysBlockSizeBytes(devName string) uint64 {
	// Size is reported in 512-byte sectors
	data, err := os.ReadFile("/sys/class/block/" + devName + "/size")
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(data))
	sectors, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return sectors * 512
}

// readSysRotational returns true if the device is rotational (HDD), false if SSD/NVMe.
func readSysRotational(devName string) (bool, bool) {
	data, err := os.ReadFile("/sys/class/block/" + devName + "/queue/rotational")
	if err != nil {
		return false, false
	}
	s := strings.TrimSpace(string(data))
	return s == "1", true
}

func baseBlockDeviceName(name string) string {
	if strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "mmcblk") || strings.HasPrefix(name, "md") {
		if i := strings.LastIndex(name, "p"); i > 0 && allDigits(name[i+1:]) {
			return name[:i]
		}
	}
	j := len(name) - 1
	for j >= 0 {
		c := name[j]
		if c < '0' || c > '9' {
			break
		}
		j--
	}
	if j >= 0 && j < len(name)-1 {
		return name[:j+1]
	}
	return name
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// getDriveType determines the high-level drive technology for the given source.
// Returns one of: "Remote", "HDD", "SSD", or "Unknown".
func getDriveType(src string, isRemote bool) string {
	if isRemote {
		return "Remote"
	}
	realPath, err := filepath.EvalSymlinks(src)
	if err != nil || realPath == "" {
		realPath = src
	}
	name := filepath.Base(realPath)

	// Device-mapper or MD RAID devices expose slaves; infer type from slaves.
	if strings.HasPrefix(name, "dm-") || strings.HasPrefix(name, "md") {
		slavesDir := "/sys/class/block/" + name + "/slaves"
		entries, err := os.ReadDir(slavesDir)
		if err == nil && len(entries) > 0 {
			anyRotational := false
			allKnown := true
			for _, e := range entries {
				if e.IsDir() { // entries are symlinks/files; use name regardless
					continue
				}
				rot, ok := readSysRotational(e.Name())
				if !ok {
					allKnown = false
					continue
				}
				if rot {
					anyRotational = true
				}
			}
			if anyRotational {
				return "HDD"
			}
			if allKnown {
				return "SSD"
			}
		}
		// Fallback to dm/md itself
		if rot, ok := readSysRotational(name); ok {
			if rot {
				return "HDD"
			}
			return "SSD"
		}
		return "Unknown"
	}

	// Regular block or partition device
	if rot, ok := readSysRotational(name); ok {
		if rot {
			return "HDD"
		}
		return "SSD"
	}

	// Try parent device when name refers to a partition
	parent := baseBlockDeviceName(name)
	if parent != name {
		if rot, ok := readSysRotational(parent); ok {
			if rot {
				return "HDD"
			}
			return "SSD"
		}
	}

	// Heuristic: NVMe implies SSD
	if strings.HasPrefix(name, "nvme") {
		return "SSD"
	}
	return "Unknown"
}

func buildResolvedDevPathToUUIDMap() map[string]string {
	m := map[string]string{}
	entries, err := os.ReadDir("/dev/disk/by-uuid")
	if err != nil {
		return m
	}
	for _, e := range entries {
		uuid := e.Name()
		if uuid == "" {
			continue
		}
		linkPath := filepath.Join("/dev/disk/by-uuid", uuid)
		resolved, err := filepath.EvalSymlinks(linkPath)
		if err != nil || resolved == "" {
			continue
		}
		m[resolved] = uuid
	}
	return m
}
