package fs

import (
	"fmt"
	"os"
	"strings"
	"time"

	"ismartcoding/plainnas/internal/db"
)

func deleteTrashItem(it *TrashItem) error {
	if it == nil || strings.TrimSpace(it.ID) == "" {
		return fmt.Errorf("invalid trash item")
	}
	l, err := lockDiskTrashRoot(it.Disk)
	if err != nil {
		return err
	}
	defer l.Unlock()
	// DB delete first (meta is truth), then physical delete.
	if err := deleteTrashItemKeys(it); err != nil {
		return err
	}
	if err := os.RemoveAll(trashAbsPath(it.Disk, it.TrashRelPath)); err != nil {
		// Best-effort: re-create metadata if physical delete fails.
		_ = storeTrashItem(it)
		return err
	}
	return nil
}

// PurgeTrashOlderThan removes items older than the given number of days.
// Returns the number of items purged.
func PurgeTrashOlderThan(days int) (int, error) {
	if days <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	purged := 0

	// Oldest-first: iterate reverse over the deleted_at index.
	err := db.GetDefault().IterateReverse(trashDeletedAtIndexPrefix(), func(key []byte, _ []byte) error {
		k := string(key)
		idx := strings.LastIndex(k, ":")
		if idx < 0 || idx+1 >= len(k) {
			return nil
		}
		id := k[idx+1:]
		it, err := loadTrashItem(id)
		if err != nil || it == nil {
			return nil
		}
		if it.DeletedAt >= cutoff {
			// Remaining items are newer; stop early.
			return db.ErrIterateStop
		}
		if err := deleteTrashItem(it); err == nil {
			purged++
		}
		return nil
	})
	return purged, err
}

// PurgeAllTrash removes all trash items (oldest-first).
func PurgeAllTrash() (int, error) {
	purged := 0
	err := db.GetDefault().IterateReverse(trashDeletedAtIndexPrefix(), func(key []byte, _ []byte) error {
		k := string(key)
		idx := strings.LastIndex(k, ":")
		if idx < 0 || idx+1 >= len(k) {
			return nil
		}
		id := k[idx+1:]
		it, err := loadTrashItem(id)
		if err != nil || it == nil {
			return nil
		}
		if err := deleteTrashItem(it); err == nil {
			purged++
		}
		return nil
	})
	return purged, err
}

// PurgeTrashOverBytes purges oldest items until the total known trash size is <= maxBytes.
// Unknown sizes (nil) are treated as 0 until the async worker fills them.
func PurgeTrashOverBytes(maxBytes int64) (int, error) {
	if maxBytes <= 0 {
		return 0, nil
	}
	var total int64
	_ = db.GetDefault().Iterate(trashDeletedAtIndexPrefix(), func(key []byte, _ []byte) error {
		k := string(key)
		idx := strings.LastIndex(k, ":")
		if idx < 0 || idx+1 >= len(k) {
			return nil
		}
		id := k[idx+1:]
		it, err := loadTrashItem(id)
		if err != nil || it == nil {
			return nil
		}
		if it.Size != nil {
			total += *it.Size
		}
		return nil
	})
	if total <= maxBytes {
		return 0, nil
	}

	purged := 0
	// Oldest-first removal.
	err := db.GetDefault().IterateReverse(trashDeletedAtIndexPrefix(), func(key []byte, _ []byte) error {
		if total <= maxBytes {
			return db.ErrIterateStop
		}
		k := string(key)
		idx := strings.LastIndex(k, ":")
		if idx < 0 || idx+1 >= len(k) {
			return nil
		}
		id := k[idx+1:]
		it, err := loadTrashItem(id)
		if err != nil || it == nil {
			return nil
		}
		if err := deleteTrashItem(it); err == nil {
			purged++
			if it.Size != nil {
				total -= *it.Size
			}
		}
		return nil
	})
	return purged, err
}
