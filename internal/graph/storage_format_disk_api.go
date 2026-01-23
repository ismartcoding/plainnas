//go:build linux

package graph

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/storage"
)

func FormatDiskSinglePartition(diskPath string, onUnmount func(mountpoint string)) error {
	diskPath = strings.TrimSpace(diskPath)
	if diskPath == "" {
		return fmt.Errorf("disk path is required")
	}
	if !strings.HasPrefix(diskPath, "/dev/") {
		return fmt.Errorf("invalid disk path: %q", diskPath)
	}

	// Prevent PlainNAS auto-mount reconciliation from racing with this formatting
	// operation (it can remount a filesystem right after we unmount it).
	releaseAutoMount := storage.InhibitAutoMount()
	defer releaseAutoMount()

	parsed, err := runLsblkJSON([]string{"NAME", "PATH", "TYPE", "MOUNTPOINT"})
	if err != nil {
		return err
	}
	if parsed == nil {
		return fmt.Errorf("lsblk returned no data")
	}

	disk := findLsblkDiskByPath(parsed.BlockDevices, diskPath)
	if disk == nil {
		return fmt.Errorf("disk not found: %s", diskPath)
	}
	if strings.TrimSpace(disk.Type) != "disk" {
		return fmt.Errorf("device is not a disk: %s", diskPath)
	}

	// Never allow formatting the system/root disk.
	if hasMountpoint(disk, "/") {
		return fmt.Errorf("refusing to format system disk: %s", diskPath)
	}

	// Ensure required tools exist.
	for _, tool := range []string{"wipefs", "sfdisk", "mkfs.ext4", "umount"} {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("required tool not found in PATH: %s", tool)
		}
	}

	// If the disk has mounted descendants, unmount them first.
	// This supports the common UX of formatting removable storage that the OS auto-mounted.
	if mps := listMountedMountpoints(disk); len(mps) > 0 {
		if err := unmountMountpoints(mps, onUnmount); err != nil {
			return err
		}

		// Re-check mount state from fresh lsblk output.
		parsed2, err := runLsblkJSON([]string{"NAME", "PATH", "TYPE", "MOUNTPOINT"})
		if err != nil {
			return err
		}
		if parsed2 == nil {
			return fmt.Errorf("lsblk returned no data")
		}
		disk2 := findLsblkDiskByPath(parsed2.BlockDevices, diskPath)
		if disk2 == nil {
			return fmt.Errorf("disk not found: %s", diskPath)
		}
		if hasMountpoint(disk2, "/") {
			return fmt.Errorf("refusing to format system disk: %s", diskPath)
		}
		if still := listMountedMountpoints(disk2); len(still) > 0 {
			return fmt.Errorf("failed to unmount disk %s (still mounted: %s)", diskPath, strings.Join(still, ", "))
		}
	}

	// Wipe existing signatures.
	if out, err := exec.Command("wipefs", "-a", diskPath).CombinedOutput(); err != nil {
		return fmt.Errorf("wipefs failed: %s", strings.TrimSpace(string(out)))
	}

	// Create a new GPT partition table with a single Linux partition.
	// Use named-fields format (documented in sfdisk(8)).
	// - start defaults to a 1MiB-aligned offset
	// - size=+ fills all remaining space
	// - type=linux is the GPT Linux filesystem type
	script := "label: gpt\nsize=+, type=linux\n"
	cmd := exec.Command("sfdisk", "--wipe", "always", "--wipe-partitions", "always", "--force", diskPath)
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sfdisk failed: %s", strings.TrimSpace(string(out)))
	}

	// Best-effort: notify kernel and wait for the partition node.
	if _, err := exec.LookPath("partprobe"); err == nil {
		_ = exec.Command("partprobe", diskPath).Run()
	}
	if _, err := exec.LookPath("udevadm"); err == nil {
		_ = exec.Command("udevadm", "settle").Run()
	}

	partPath := firstPartitionPath(diskPath)
	if err := waitForPath(partPath, 3*time.Second); err != nil {
		return fmt.Errorf("partition device not found after partitioning: %s", partPath)
	}

	// Format as ext4 (common default on Linux).
	if out, err := exec.Command("mkfs.ext4", "-F", "-L", "plainnas", partPath).CombinedOutput(); err != nil {
		return fmt.Errorf("mkfs.ext4 failed: %s", strings.TrimSpace(string(out)))
	}

	// Now that formatting is complete, do a best-effort reconciliation so the new
	// filesystem gets mounted into a stable /mnt/usbX slot.
	// Temporarily lift inhibition for this single call.
	releaseAutoMount()
	releaseAutoMount = func() {}
	enCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	_ = storage.EnsureMountedUSBVolumes(enCtx)
	cancel()

	return nil
}

func unmountMountpoints(mountpoints []string, onUnmount func(mountpoint string)) error {
	mps := make([]string, 0, len(mountpoints))
	seen := map[string]struct{}{}
	for _, mp := range mountpoints {
		mp = strings.TrimSpace(mp)
		if mp == "" {
			continue
		}
		if _, ok := seen[mp]; ok {
			continue
		}
		seen[mp] = struct{}{}
		mps = append(mps, mp)
	}

	// Unmount deepest paths first to avoid issues with nested mounts.
	sort.Slice(mps, func(i, j int) bool {
		if len(mps[i]) != len(mps[j]) {
			return len(mps[i]) > len(mps[j])
		}
		return mps[i] > mps[j]
	})

	for _, mp := range mps {
		out, err := exec.Command("umount", mp).CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("failed to unmount %s: %s", mp, msg)
		}
		if onUnmount != nil {
			onUnmount(mp)
		}
	}

	return nil
}

func findLsblkDiskByPath(devs []lsblkDevice, diskPath string) *lsblkDevice {
	var found *lsblkDevice
	var walk func(d lsblkDevice)
	walk = func(d lsblkDevice) {
		if found != nil {
			return
		}
		p := strings.TrimSpace(d.Path)
		n := strings.TrimSpace(d.Name)
		if p == diskPath || (p == "" && n != "" && filepath.Join("/dev", n) == diskPath) {
			dd := d
			found = &dd
			return
		}
		for _, c := range d.Children {
			walk(c)
		}
	}
	for _, d := range devs {
		walk(d)
	}
	return found
}

func hasMountpoint(d *lsblkDevice, mountpoint string) bool {
	if d == nil {
		return false
	}
	if strings.TrimSpace(d.Mountpoint) == mountpoint {
		return true
	}
	for i := range d.Children {
		c := d.Children[i]
		if hasMountpoint(&c, mountpoint) {
			return true
		}
	}
	return false
}

func listMountedMountpoints(d *lsblkDevice) []string {
	if d == nil {
		return nil
	}
	mps := []string{}
	seen := map[string]struct{}{}
	var walk func(x *lsblkDevice)
	walk = func(x *lsblkDevice) {
		if x == nil {
			return
		}
		mp := strings.TrimSpace(x.Mountpoint)
		if mp != "" {
			if _, ok := seen[mp]; !ok {
				seen[mp] = struct{}{}
				mps = append(mps, mp)
			}
		}
		for i := range x.Children {
			c := x.Children[i]
			walk(&c)
		}
	}
	walk(d)
	return mps
}

func firstPartitionPath(diskPath string) string {
	// nvme0n1 -> nvme0n1p1, mmcblk0 -> mmcblk0p1, sda -> sda1
	base := strings.TrimSpace(diskPath)
	if base == "" {
		return ""
	}
	last := base[len(base)-1]
	if last >= '0' && last <= '9' {
		return base + "p1"
	}
	return base + "1"
}

func waitForPath(path string, timeout time.Duration) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is empty")
	}
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout")
		}
		time.Sleep(80 * time.Millisecond)
	}
}

func eventErrSummary(err error) string {
	if err == nil {
		return ""
	}
	// Keep event messages short and single-line.
	s := strings.TrimSpace(err.Error())
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	const maxLen = 200
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

func formatDisk(ctx context.Context, path string) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return false, fmt.Errorf("path is empty")
	}
	clientID, _ := ctx.Value(ContextKeyClientID).(string)
	clientID = strings.TrimSpace(clientID)
	onUnmount := func(mountpoint string) {
		db.AddEvent("unmount", mountpoint, clientID)
	}
	if err := FormatDiskSinglePartition(path, onUnmount); err != nil {
		db.AddEvent("format_disk_failed", fmt.Sprintf("%s: %s", path, eventErrSummary(err)), clientID)
		return false, err
	}
	db.AddEvent("format_disk", path, clientID)
	return true, nil
}
