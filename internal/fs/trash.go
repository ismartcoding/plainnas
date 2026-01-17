package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ismartcoding/plainnas/internal/pkg/shortid"
)

func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	name := base
	ext := ""
	if i := strings.LastIndex(base, "."); i > 0 {
		name = base[:i]
		ext = base[i:]
	}
	for i := 1; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}

func isEXDEV(err error) bool {
	if errors.Is(err, syscall.EXDEV) {
		return true
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}

func validateTrashablePath(p string) error {
	p = filepath.Clean(p)
	if strings.TrimSpace(p) == "" || p == "." {
		return fmt.Errorf("invalid path")
	}
	if p == string(filepath.Separator) {
		return fmt.Errorf("refuse to trash root")
	}
	// Refuse any nested trash locations.
	if isInNasTrash(p) {
		return fmt.Errorf("refuse to trash items inside .nas-trash")
	}
	return nil
}

// TrashPaths moves each provided filesystem path into its disk-local `.nas-trash`.
// Constraints:
// - Delete is rename-only (O(1))
// - No directory traversal
// - No cross-filesystem copy
// - All restore/GC logic uses Pebble metadata as the single source of truth
// Returns the new trashed paths (physical paths under `.nas-trash`).
func TrashPaths(paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))

	for _, raw := range paths {
		src := filepath.Clean(raw)
		if err := validateTrashablePath(src); err != nil {
			return nil, err
		}
		fi, err := os.Lstat(src)
		if err != nil {
			return nil, err
		}

		kind := "file"
		if fi.IsDir() {
			kind = "dir"
		}
		now := time.Now()
		deletedAt := now.Unix()
		id := shortid.New()

		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok || st == nil {
			return nil, fmt.Errorf("unsupported stat")
		}

		// Mountpoint resolution should be based on where the directory entry lives.
		// For symlinks, do not resolve the symlink target (it may be on another FS).
		mountProbe := src
		if fi.Mode()&os.ModeSymlink != 0 {
			mountProbe = filepath.Dir(src)
		}
		diskMount, err := resolveMountPoint(mountProbe)
		if err != nil {
			return nil, err
		}
		rel, err := computeBucketRelPath(kind, id, filepath.Base(src), now)
		if err != nil {
			return nil, err
		}
		dstAbs := trashAbsPath(diskMount, rel)

		l, err := lockDiskTrashRoot(diskMount)
		if err != nil {
			return nil, err
		}
		// Serialize rename + metadata updates per disk.
		if err := func() error {
			defer l.Unlock()

			if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
				return err
			}
			if err := os.Rename(src, dstAbs); err != nil {
				if isEXDEV(err) {
					return fmt.Errorf("cross-filesystem rename forbidden")
				}
				return err
			}

			item := &TrashItem{
				ID:           id,
				Type:         kind,
				OriginalPath: filepath.ToSlash(src),
				Disk:         diskMount,
				TrashRelPath: filepath.ToSlash(rel),
				DeletedAt:    deletedAt,
				UID:          int(st.Uid),
				GID:          int(st.Gid),
				Mode:         int(fi.Mode()),
				Size:         nil,
				EntryCount:   nil,
			}
			if err := storeTrashItem(item); err != nil {
				// Roll back rename so metadata stays the single truth.
				_ = os.Rename(dstAbs, src)
				return fmt.Errorf("failed to write trash metadata: %w", err)
			}
			return nil
		}(); err != nil {
			return nil, err
		}

		enqueueTrashStats(id)
		out = append(out, filepath.ToSlash(dstAbs))
	}
	return out, nil
}

func uniqueRestoredPath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	name := base
	ext := ""
	if i := strings.LastIndex(base, "."); i > 0 {
		name = base[:i]
		ext = base[i:]
	}
	// First attempt matches the spec wording.
	first := filepath.Join(dir, name+" (restored)"+ext)
	if _, err := os.Stat(first); os.IsNotExist(err) {
		return first
	}
	for i := 2; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s (restored %d)%s", name, i, ext))
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}

func validateRestorePath(p string) error {
	p = filepath.Clean(p)
	// Accept either a physical trash path (preferred for UI) or a raw trash item ID.
	if isInNasTrash(p) {
		if _, ok := parseTrashIDFromPath(p); !ok {
			return fmt.Errorf("invalid trash path")
		}
		return nil
	}
	if strings.TrimSpace(p) == "" {
		return fmt.Errorf("invalid trash id")
	}
	if strings.Contains(p, string(filepath.Separator)) || strings.Contains(p, "/") {
		return fmt.Errorf("invalid trash id")
	}
	return nil
}

// RestorePaths restores each provided trashed path back to its original location.
// The trashed path is treated as an identifier (f_<id>/d_<id>); restore always uses
// TrashItem metadata as the single source of truth.
// Returns the restored target paths.
func RestorePaths(trashedPaths []string) ([]string, error) {
	out := make([]string, 0, len(trashedPaths))
	for _, raw := range trashedPaths {
		src := filepath.Clean(raw)
		if err := validateRestorePath(src); err != nil {
			return nil, err
		}

		id, ok := parseTrashIDFromPath(src)
		if !ok {
			id = src
		}
		item, err := loadTrashItem(id)
		if err != nil {
			return nil, err
		}
		if item == nil {
			return nil, fmt.Errorf("trash item not found")
		}
		trashPath := trashAbsPath(item.Disk, item.TrashRelPath)
		target := filepath.Clean(item.OriginalPath)
		if strings.TrimSpace(target) == "" || target == "." || target == string(filepath.Separator) {
			return nil, fmt.Errorf("invalid original path")
		}
		// Enforce same-disk restore.
		diskRoot := filepath.Clean(item.Disk)
		if diskRoot == string(filepath.Separator) {
			// Root mount is a special case: diskRoot+sep would be "//", which never matches.
			// For /, any absolute path is on the same disk.
			if !filepath.IsAbs(target) {
				return nil, fmt.Errorf("restore target must be on original disk")
			}
		} else {
			if !(target == diskRoot || strings.HasPrefix(target, diskRoot+string(filepath.Separator))) {
				return nil, fmt.Errorf("restore target must be on original disk")
			}
		}

		l, err := lockDiskTrashRoot(item.Disk)
		if err != nil {
			return nil, err
		}
		if err := func() error {
			defer l.Unlock()
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if _, err := os.Stat(target); err == nil {
				target = uniqueRestoredPath(target)
			}
			if err := os.Rename(trashPath, target); err != nil {
				return err
			}
			_ = os.Chown(target, item.UID, item.GID)
			_ = os.Chmod(target, os.FileMode(item.Mode))
			if err := deleteTrashItemKeys(item); err != nil {
				// Keep metadata as the only truth: roll back physical restore on metadata failure.
				_ = os.Rename(target, trashPath)
				return err
			}
			return nil
		}(); err != nil {
			return nil, err
		}
		out = append(out, filepath.ToSlash(target))
	}
	return out, nil
}

// IsInNasTrash reports whether p is within a `.nas-trash` tree.
func IsInNasTrash(p string) bool {
	return isInNasTrash(p)
}

// ListTrash returns trash items from the Pebble index (newest first).
func ListTrash(offset, limit int, text string) ([]*TrashItem, error) {
	return listTrashItemsDirFirstNewestFirst(offset, limit, text)
}

// ListTrashOldestFirst returns trash items ordered by deletedAt ascending.
func ListTrashOldestFirst(offset, limit int, text string) ([]*TrashItem, error) {
	return listTrashItemsDirFirstOldestFirst(offset, limit, text)
}

// ListTrashByName returns trash items ordered by original base name (case-insensitive).
func ListTrashByName(offset, limit int, text string, desc bool) ([]*TrashItem, error) {
	return listTrashItemsByName(offset, limit, text, desc)
}

// ListTrashBySize returns trash items ordered by recorded size (dirs/missing size treated as 0).
func ListTrashBySize(offset, limit int, text string, desc bool) ([]*TrashItem, error) {
	return listTrashItemsBySize(offset, limit, text, desc)
}

// TrashCount returns the total number of trash items tracked in metadata.
func TrashCount() (int, error) {
	return countTrashItems()
}

// DeleteTrashByPath permanently deletes a trashed entry using metadata as truth.
func DeleteTrashByPath(trashedPath string) error {
	trashedPath = filepath.Clean(trashedPath)
	if !isInNasTrash(trashedPath) {
		return fmt.Errorf("not a trash path")
	}
	id, ok := parseTrashIDFromPath(trashedPath)
	if !ok {
		return fmt.Errorf("invalid trash path")
	}
	it, err := loadTrashItem(id)
	if err != nil {
		return err
	}
	if it == nil {
		// Best-effort fallback: remove physical path.
		return os.RemoveAll(trashedPath)
	}
	return deleteTrashItem(it)
}
