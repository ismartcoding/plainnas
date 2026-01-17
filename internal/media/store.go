package media

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"ismartcoding/plainnas/internal/db"

	"github.com/cockroachdb/pebble"
)

// ResetAllMediaData removes both index and all Pebble media data.
// This is used for complete rebuild operations.
func ResetAllMediaData() error {
	// First reset the media index
	if err := ResetMediaIndex(); err != nil {
		return err
	}

	// Then remove all media data from Pebble database
	peb := db.GetDefault()
	batch := make([][]byte, 0, 1000)

	// Remove all media:uuid: entries
	_ = peb.Iterate([]byte("media:uuid:"), func(key []byte, value []byte) error {
		batch = append(batch, append([]byte{}, key...))
		if len(batch) >= 1000 {
			_ = peb.BatchDelete(batch)
			batch = batch[:0]
		}
		return nil
	})

	// Remove all media:docid: entries
	_ = peb.Iterate([]byte("media:docid:"), func(key []byte, value []byte) error {
		batch = append(batch, append([]byte{}, key...))
		if len(batch) >= 1000 {
			_ = peb.BatchDelete(batch)
			batch = batch[:0]
		}
		return nil
	})

	// Remove all media:type: secondary index entries
	_ = peb.Iterate([]byte("media:type:"), func(key []byte, value []byte) error {
		batch = append(batch, append([]byte{}, key...))
		if len(batch) >= 1000 {
			_ = peb.BatchDelete(batch)
			batch = batch[:0]
		}
		return nil
	})

	// Remove all media:path: entries
	_ = peb.Iterate([]byte("media:path:"), func(key []byte, value []byte) error {
		batch = append(batch, append([]byte{}, key...))
		if len(batch) >= 1000 {
			_ = peb.BatchDelete(batch)
			batch = batch[:0]
		}
		return nil
	})

	// Remove all media:fid: entries
	_ = peb.Iterate([]byte("media:fid:"), func(key []byte, value []byte) error {
		batch = append(batch, append([]byte{}, key...))
		if len(batch) >= 1000 {
			_ = peb.BatchDelete(batch)
			batch = batch[:0]
		}
		return nil
	})

	// Final batch delete
	if len(batch) > 0 {
		_ = peb.BatchDelete(batch)
	}

	markTagRelationCleanupNeeded()

	return nil
}

// MediaIndexExists reports whether the media bleve index already exists on disk
func MediaIndexExists() bool {
	_, e1 := os.Stat(mediaNameDat())
	_, e2 := os.Stat(mediaNameIdx())
	_, e3 := os.Stat(mediaNameDict())
	_, e4 := os.Stat(mediaPathDat())
	_, e5 := os.Stat(mediaPathIdx())
	_, e6 := os.Stat(mediaPathDict())
	return e1 == nil && e2 == nil && e3 == nil && e4 == nil && e5 == nil && e6 == nil
}

func UpsertMedia(m *MediaFile) error {
	if m == nil {
		return nil
	}
	// Normalize path and name
	m.Path = filepath.ToSlash(m.Path)
	if m.FSUUID == "" && m.Path != "" {
		// Best-effort: fill missing FSUUID to avoid FID collisions.
		if fsid, err := filesystemIDForPath(m.Path); err == nil {
			m.FSUUID = fsid
		}
	}
	if m.FSUUID == "" {
		m.FSUUID = string(filepath.Separator)
	}
	if m.Name == "" {
		m.Name = filepath.Base(m.Path)
	}
	if m.Type == "" {
		m.Type = inferType(m.Name)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	peb := db.GetDefault()
	// Fast path: skip writes if content unchanged
	if old, _ := peb.Get(keyByUUID(m.UUID)); old != nil && bytes.Equal(old, b) {
		return nil
	} else if old != nil {
		var prev MediaFile
		_ = json.Unmarshal(old, &prev)
		if prev.Path != "" && prev.Path != m.Path {
			_ = peb.Delete(keyByPath(prev.Path))
		}
		// Remove secondary indexes if type/trash/mtime changed
		if prev.UUID != "" {
			_ = peb.Delete(keyTypeTrashUUID(prev.Type, prev.IsTrash, prev.UUID))
			_ = peb.Delete(keyTypeTrashMod(prev.Type, prev.IsTrash, prev.ModifiedAt, prev.UUID))
			_ = peb.Delete(keyTypeTrashModDesc(prev.Type, prev.IsTrash, prev.ModifiedAt, prev.UUID))
			_ = peb.Delete(keyTypeTrashName(prev.Type, prev.IsTrash, prev.Name, prev.UUID))
			_ = peb.Delete(keyTypeTrashNameDesc(prev.Type, prev.IsTrash, prev.Name, prev.UUID))
			_ = peb.Delete(keyTypeTrashSize(prev.Type, prev.IsTrash, prev.Size, prev.UUID))
			_ = peb.Delete(keyTypeTrashSizeDesc(prev.Type, prev.IsTrash, prev.Size, prev.UUID))
		}
	}

	if err := peb.Set(keyByUUID(m.UUID), b, &pebble.WriteOptions{Sync: false}); err != nil {
		return err
	}
	if err := peb.Set(keyByFID(m.FSUUID, m.Ino, m.Ctime), []byte(m.UUID), &pebble.WriteOptions{Sync: false}); err != nil {
		return err
	}
	_ = peb.Set(keyByPath(m.Path), []byte(m.UUID), &pebble.WriteOptions{Sync: false})
	// Secondary indexes (type + trash + modified time)
	_ = peb.Set(keyTypeTrashUUID(m.Type, m.IsTrash, m.UUID), []byte{}, &pebble.WriteOptions{Sync: false})
	_ = peb.Set(keyTypeTrashMod(m.Type, m.IsTrash, m.ModifiedAt, m.UUID), []byte{}, &pebble.WriteOptions{Sync: false})
	_ = peb.Set(keyTypeTrashModDesc(m.Type, m.IsTrash, m.ModifiedAt, m.UUID), []byte{}, &pebble.WriteOptions{Sync: false})
	_ = peb.Set(keyTypeTrashName(m.Type, m.IsTrash, m.Name, m.UUID), []byte{}, &pebble.WriteOptions{Sync: false})
	_ = peb.Set(keyTypeTrashNameDesc(m.Type, m.IsTrash, m.Name, m.UUID), []byte{}, &pebble.WriteOptions{Sync: false})
	_ = peb.Set(keyTypeTrashSize(m.Type, m.IsTrash, m.Size, m.UUID), []byte{}, &pebble.WriteOptions{Sync: false})
	_ = peb.Set(keyTypeTrashSizeDesc(m.Type, m.IsTrash, m.Size, m.UUID), []byte{}, &pebble.WriteOptions{Sync: false})

	// Index updates are handled by background rebuild; no runtime writes.
	return nil
}

// FlushMediaIndexBatch flushes any pending bleve index batch to disk.
func FlushMediaIndexBatch() error { return nil }

// DeleteMedia removes media by uuid
func DeleteMedia(uuid string) error {
	peb := db.GetDefault()
	// Tags are bound to media UUIDs; deleting a media item must remove tag relations.
	_ = db.DeleteTagRelationsByKeys([]string{uuid})
	var m MediaFile
	if err := peb.LoadJSON("media:uuid:"+uuid, &m); err != nil {
		return err
	}
	_ = peb.Delete([]byte("media:uuid:" + uuid))
	if m.UUID != "" {
		_ = peb.Delete(keyTypeTrashUUID(m.Type, m.IsTrash, m.UUID))
		_ = peb.Delete(keyTypeTrashMod(m.Type, m.IsTrash, m.ModifiedAt, m.UUID))
		_ = peb.Delete(keyTypeTrashModDesc(m.Type, m.IsTrash, m.ModifiedAt, m.UUID))
		_ = peb.Delete(keyTypeTrashName(m.Type, m.IsTrash, m.Name, m.UUID))
		_ = peb.Delete(keyTypeTrashNameDesc(m.Type, m.IsTrash, m.Name, m.UUID))
		_ = peb.Delete(keyTypeTrashSize(m.Type, m.IsTrash, m.Size, m.UUID))
		_ = peb.Delete(keyTypeTrashSizeDesc(m.Type, m.IsTrash, m.Size, m.UUID))
	}
	if m.Path != "" {
		_ = peb.Delete(keyByPath(m.Path))
	}
	if m.OriginalPath != "" && m.OriginalPath != m.Path {
		_ = peb.Delete(keyByPath(m.OriginalPath))
	}
	if m.FSUUID != "" && (m.Ino != 0 || m.Ctime != 0) {
		_ = peb.Delete(keyByFID(m.FSUUID, m.Ino, m.Ctime))
	}
	// No per-doc deletion from index; index rebuild handles removals.
	return nil
}

// GetFile returns a MediaFile by uuid
func GetFile(uuid string) (*MediaFile, error) {
	var m MediaFile
	if err := db.GetDefault().LoadJSON("media:uuid:"+uuid, &m); err != nil {
		return nil, err
	}
	if m.UUID == "" {
		return nil, nil
	}
	return &m, nil
}

// FindByPath returns uuid by path if exists
func FindByPath(path string) (string, error) {
	path = filepath.ToSlash(path)
	b, err := db.GetDefault().Get(keyByPath(path))
	if err != nil || b == nil {
		return "", err
	}
	return string(b), nil
}

// FindUUIDByFID returns uuid by dev/ino/ctime
func FindUUIDByFID(fsUUID string, ino uint64, ctime int64) (string, error) {
	b, err := db.GetDefault().Get(keyByFID(fsUUID, ino, ctime))
	if err != nil || b == nil {
		return "", err
	}
	return string(b), nil
}
