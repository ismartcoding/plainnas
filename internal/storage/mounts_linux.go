//go:build linux

package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/pkg/log"
)

const (
	plainnasMountRoot = "/mnt"
	usbPrefix         = "usb"
)

type lsblkOutput struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Name       string        `json:"name"`
	Path       string        `json:"path"`
	Type       string        `json:"type"`
	Fstype     string        `json:"fstype"`
	UUID       string        `json:"uuid"`
	SizeBytes  json.Number   `json:"size"`
	Mountpoint string        `json:"mountpoint"`
	Children   []lsblkDevice `json:"children"`
}

type blockFS struct {
	Path       string
	FSUUID     string
	FSType     string
	SizeBytes  uint64
	Mountpoint string
}

func EnsureMountedUSBVolumes(ctx context.Context) error {
	// Ensure mount root exists.
	if err := os.MkdirAll(plainnasMountRoot, 0755); err != nil {
		return err
	}

	// Load persisted mapping.
	persisted := db.GetFSUUIDSlotMap()

	// Discover filesystems.
	filesystems, err := scanFilesystems(ctx)
	if err != nil {
		return err
	}

	// Build mapping from currently mounted /mnt/usbX, and track used slots.
	mapping := map[string]int{}
	usedSlots := map[int]struct{}{}
	for _, fs := range filesystems {
		if fs.FSUUID == "" {
			continue
		}
		if slot, ok := parseUSBSlot(fs.Mountpoint); ok {
			mapping[fs.FSUUID] = slot
			usedSlots[slot] = struct{}{}
		}
	}

	// Mount any present-but-not-mounted filesystem by UUID into /mnt/usbX.
	for _, fs := range filesystems {
		if fs.FSUUID == "" {
			continue
		}
		if fs.Mountpoint != "" {
			// Already mounted (anywhere). If it's mounted to /mnt/usbX we already captured mapping.
			continue
		}

		// Choose slot: prefer persisted if it isn't currently occupied.
		slot, ok := persisted[fs.FSUUID]
		if ok {
			if slot <= 0 {
				ok = false
			}
			if _, occupied := usedSlots[slot]; occupied {
				ok = false
			}
		}
		if !ok {
			slot = nextFreeSlot(usedSlots)
		}
		usedSlots[slot] = struct{}{}
		mapping[fs.FSUUID] = slot

		target := filepath.Join(plainnasMountRoot, fmt.Sprintf("%s%d", usbPrefix, slot))
		if err := os.MkdirAll(target, 0755); err != nil {
			log.Errorf("mount: mkdir %s failed: %v", target, err)
			continue
		}
		// Avoid stacked mounts: if a previous device was mounted at /mnt/usbX and
		// never cleanly unmounted (e.g. hot-unplug), Linux can keep older layers.
		// Always clear /mnt/usbX before mounting a new filesystem there.
		if err := unmountAllAt(target); err != nil {
			log.Errorf("mount: cleanup %s failed: %v", target, err)
			continue
		}
		if err := mountByUUID(ctx, fs.FSUUID, target); err != nil {
			log.Errorf("mount: UUID %s -> %s failed: %v", fs.FSUUID, target, err)
			continue
		}
		log.Infof("mounted UUID %s at %s", fs.FSUUID, target)
	}

	// Persist mapping: includes already-mounted /mnt/usbX and newly mounted ones.
	return db.StoreFSUUIDSlotMap(mapping)
}

func scanFilesystems(ctx context.Context) ([]blockFS, error) {
	// lsblk provides filesystem UUID (UUID) and mountpoint; -b ensures size is bytes.
	cmd := exec.CommandContext(ctx, "lsblk", "-b", "-J", "-o", "NAME,PATH,TYPE,FSTYPE,UUID,SIZE,MOUNTPOINT")
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("lsblk failed: %v: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}

	var parsed lsblkOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}

	flat := make([]lsblkDevice, 0, 64)
	var walk func(d lsblkDevice)
	walk = func(d lsblkDevice) {
		flat = append(flat, d)
		for _, c := range d.Children {
			walk(c)
		}
	}
	for _, d := range parsed.BlockDevices {
		walk(d)
	}

	skipFSTypes := map[string]struct{}{
		"swap":        {},
		"crypto_LUKS": {},
		"LVM2_member": {},
	}

	items := make([]blockFS, 0, len(flat))
	seen := map[string]struct{}{}
	for _, d := range flat {
		uuid := strings.TrimSpace(d.UUID)
		fstype := strings.TrimSpace(d.Fstype)
		path := strings.TrimSpace(d.Path)
		mp := strings.TrimSpace(d.Mountpoint)
		if uuid == "" || fstype == "" || path == "" {
			continue
		}
		if _, skip := skipFSTypes[fstype]; skip {
			continue
		}
		if _, ok := seen[uuid]; ok {
			// Prefer first; lsblk may show the same UUID multiple times in edge cases.
			continue
		}

		var size uint64
		if d.SizeBytes != "" {
			if v, err := d.SizeBytes.Int64(); err == nil && v > 0 {
				size = uint64(v)
			} else if v, err := strconv.ParseUint(d.SizeBytes.String(), 10, 64); err == nil {
				size = v
			}
		}

		items = append(items, blockFS{
			Path:       path,
			FSUUID:     uuid,
			FSType:     fstype,
			SizeBytes:  size,
			Mountpoint: mp,
		})
		seen[uuid] = struct{}{}
	}

	// Deterministic order for logs and stable behavior.
	sort.Slice(items, func(i, j int) bool { return items[i].FSUUID < items[j].FSUUID })
	return items, nil
}

func mountByUUID(ctx context.Context, fsUUID string, target string) error {
	if fsUUID == "" {
		return fmt.Errorf("empty uuid")
	}
	// mount(8) supports -U for filesystem UUID.
	cmd := exec.CommandContext(ctx, "mount", "-U", fsUUID, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.TrimSpace(string(out))
		if s == "" {
			s = err.Error()
		}
		return fmt.Errorf("%s", s)
	}
	return nil
}

var usbSlotRe = regexp.MustCompile("^" + regexp.QuoteMeta(plainnasMountRoot) + "/" + usbPrefix + "([0-9]+)$")

func parseUSBSlot(mountpoint string) (int, bool) {
	if mountpoint == "" {
		return 0, false
	}
	m := usbSlotRe.FindStringSubmatch(mountpoint)
	if len(m) != 2 {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func nextFreeSlot(used map[int]struct{}) int {
	// Choose smallest positive slot (usb1, usb2, ...).
	for i := 1; ; i++ {
		if _, ok := used[i]; !ok {
			return i
		}
	}
}

func isMountpoint(path string) bool {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// mountinfo fields: see man 5 proc. The mount point is field 5.
		parts := strings.Split(line, " ")
		if len(parts) < 5 {
			continue
		}
		if parts[4] == path {
			return true
		}
	}
	return false
}

func unmountAllAt(path string) error {
	const maxLayers = 32
	for i := 0; i < maxLayers; i++ {
		if !isMountpoint(path) {
			return nil
		}
		// Try normal unmount first.
		if err := syscall.Unmount(path, 0); err != nil {
			// If busy, try a lazy detach so the system can drop it when unused.
			if err2 := syscall.Unmount(path, syscall.MNT_DETACH); err2 != nil {
				return err
			}
		}
	}
	return fmt.Errorf("too many mount layers at %s", path)
}
