package graph

import (
	"os"
	"path/filepath"
	"strings"
)

func readSysBlockModel(devName string) *string {
	devName = strings.TrimSpace(devName)
	if devName == "" {
		return nil
	}
	data, err := os.ReadFile("/sys/class/block/" + devName + "/device/model")
	if err != nil {
		return nil
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return nil
	}
	return &s
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
