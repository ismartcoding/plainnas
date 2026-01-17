package media

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"ismartcoding/plainnas/internal/db"
)

func trashFlag(isTrash bool) int {
	if isTrash {
		return 1
	}
	return 0
}

const maxModKey int64 = (1 << 62) - 1
const maxSizeKey int64 = (1 << 62) - 1

func modKey(mod int64) string {
	if mod < 0 {
		mod = 0
	}
	return fmt.Sprintf("%020d", mod)
}

func modDescKey(mod int64) string {
	if mod < 0 {
		mod = 0
	}
	return fmt.Sprintf("%020d", maxModKey-mod)
}

func sizeKey(size int64) string {
	if size < 0 {
		size = 0
	}
	if size > maxSizeKey {
		size = maxSizeKey
	}
	return fmt.Sprintf("%020d", size)
}

func sizeDescKey(size int64) string {
	if size < 0 {
		size = 0
	}
	if size > maxSizeKey {
		size = maxSizeKey
	}
	return fmt.Sprintf("%020d", maxSizeKey-size)
}

func keyTypeTrashUUID(mediaType string, isTrash bool, uuid string) []byte {
	return []byte(fmt.Sprintf("media:type:%s:trash:%d:uuid:%s", strings.ToLower(mediaType), trashFlag(isTrash), uuid))
}

func keyTypeTrashMod(mediaType string, isTrash bool, mod int64, uuid string) []byte {
	return []byte(fmt.Sprintf("media:type:%s:trash:%d:mod:%s:%s", strings.ToLower(mediaType), trashFlag(isTrash), modKey(mod), uuid))
}

func keyTypeTrashModDesc(mediaType string, isTrash bool, mod int64, uuid string) []byte {
	return []byte(fmt.Sprintf("media:type:%s:trash:%d:moddesc:%s:%s", strings.ToLower(mediaType), trashFlag(isTrash), modDescKey(mod), uuid))
}

func normName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return strings.ToLower(name)
}

// nameDescKey inverts bytes so ascending iteration yields descending name order.
// This provides a fast NAME_DESC scan without requiring reverse iteration.
func nameDescKey(name string) string {
	b := []byte(normName(name))
	for i := range b {
		b[i] = 0xFF - b[i]
	}
	return string(b)
}

func keyTypeTrashName(mediaType string, isTrash bool, name string, uuid string) []byte {
	return []byte(fmt.Sprintf("media:type:%s:trash:%d:name:%s:%s", strings.ToLower(mediaType), trashFlag(isTrash), normName(name), uuid))
}

func keyTypeTrashNameDesc(mediaType string, isTrash bool, name string, uuid string) []byte {
	return []byte(fmt.Sprintf("media:type:%s:trash:%d:namedesc:%s:%s", strings.ToLower(mediaType), trashFlag(isTrash), nameDescKey(name), uuid))
}

func keyTypeTrashSize(mediaType string, isTrash bool, size int64, uuid string) []byte {
	return []byte(fmt.Sprintf("media:type:%s:trash:%d:size:%s:%s", strings.ToLower(mediaType), trashFlag(isTrash), sizeKey(size), uuid))
}

func keyTypeTrashSizeDesc(mediaType string, isTrash bool, size int64, uuid string) []byte {
	return []byte(fmt.Sprintf("media:type:%s:trash:%d:sizedesc:%s:%s", strings.ToLower(mediaType), trashFlag(isTrash), sizeDescKey(size), uuid))
}

// TypeIndexPrefix returns the Pebble prefix for iterating the type/trash indexes.
// indexKind must be one of: "uuid", "mod", "moddesc", "name", "namedesc", "size", "sizedesc".
func TypeIndexPrefix(mediaType string, isTrash bool, indexKind string) []byte {
	mediaType = strings.ToLower(mediaType)
	switch indexKind {
	case "uuid":
		return []byte(fmt.Sprintf("media:type:%s:trash:%d:uuid:", mediaType, trashFlag(isTrash)))
	case "mod":
		return []byte(fmt.Sprintf("media:type:%s:trash:%d:mod:", mediaType, trashFlag(isTrash)))
	case "moddesc":
		return []byte(fmt.Sprintf("media:type:%s:trash:%d:moddesc:", mediaType, trashFlag(isTrash)))
	case "name":
		return []byte(fmt.Sprintf("media:type:%s:trash:%d:name:", mediaType, trashFlag(isTrash)))
	case "namedesc":
		return []byte(fmt.Sprintf("media:type:%s:trash:%d:namedesc:", mediaType, trashFlag(isTrash)))
	case "size":
		return []byte(fmt.Sprintf("media:type:%s:trash:%d:size:", mediaType, trashFlag(isTrash)))
	case "sizedesc":
		return []byte(fmt.Sprintf("media:type:%s:trash:%d:sizedesc:", mediaType, trashFlag(isTrash)))
	default:
		return []byte("")
	}
}

// UUIDFromTypeIndexKey extracts the uuid suffix from a type index key.
func UUIDFromTypeIndexKey(key []byte) string {
	s := string(key)
	idx := strings.LastIndexByte(s, ':')
	if idx < 0 || idx+1 >= len(s) {
		return ""
	}
	return s[idx+1:]
}

// EnsureTypeIndexes ensures the type/trash secondary indexes exist; if missing, it rebuilds them.
// This project is currently in development, so we intentionally do not persist an index version key.
func EnsureTypeIndexes() error {
	peb := db.GetDefault()

	// Minimal but safe: ensure every index kind exists somewhere. This avoids
	// the "some indexes exist so we skip rebuild" bug when adding new kinds.
	need := map[string]bool{
		"uuid":     true,
		"mod":      true,
		"moddesc":  true,
		"name":     true,
		"namedesc": true,
		"size":     true,
		"sizedesc": true,
	}
	remaining := len(need)
	trashMarker := []byte(":trash:")
	iterErr := peb.Iterate([]byte("media:type:"), func(k []byte, _ []byte) error {
		p := bytes.Index(k, trashMarker)
		if p < 0 {
			return nil
		}
		p += len(trashMarker)
		// Expect ":trash:<digit>:<kind>:..."
		if p+2 >= len(k) {
			return nil
		}
		p += 2 // skip "<digit>:"
		q := bytes.IndexByte(k[p:], ':')
		if q <= 0 {
			return nil
		}
		kind := string(k[p : p+q])
		if need[kind] {
			need[kind] = false
			remaining--
			if remaining == 0 {
				return db.ErrIterateStop
			}
		}
		return nil
	})
	if iterErr != nil && iterErr != db.ErrIterateStop {
		return iterErr
	}
	if remaining == 0 {
		return nil
	}
	return RebuildTypeIndexes()
}

// RebuildTypeIndexes rebuilds the type/trash secondary indexes from the primary media:uuid: records.
func RebuildTypeIndexes() error {
	peb := db.GetDefault()
	// Clear previous type indexes to avoid duplicates/stale entries.
	var keysToDelete [][]byte
	_ = peb.Iterate([]byte("media:type:"), func(k []byte, _ []byte) error {
		keysToDelete = append(keysToDelete, k)
		if len(keysToDelete) >= 2000 {
			_ = peb.BatchDelete(keysToDelete)
			keysToDelete = keysToDelete[:0]
		}
		return nil
	})
	if len(keysToDelete) > 0 {
		_ = peb.BatchDelete(keysToDelete)
	}

	if err := peb.Iterate([]byte("media:uuid:"), func(_ []byte, value []byte) error {
		var m MediaFile
		if err := json.Unmarshal(value, &m); err != nil {
			return nil
		}
		if m.UUID == "" {
			return nil
		}
		// Best-effort writes, async.
		_ = peb.Set(keyTypeTrashUUID(m.Type, m.IsTrash, m.UUID), []byte{}, nil)
		_ = peb.Set(keyTypeTrashMod(m.Type, m.IsTrash, m.ModifiedAt, m.UUID), []byte{}, nil)
		_ = peb.Set(keyTypeTrashModDesc(m.Type, m.IsTrash, m.ModifiedAt, m.UUID), []byte{}, nil)
		_ = peb.Set(keyTypeTrashName(m.Type, m.IsTrash, m.Name, m.UUID), []byte{}, nil)
		_ = peb.Set(keyTypeTrashNameDesc(m.Type, m.IsTrash, m.Name, m.UUID), []byte{}, nil)
		_ = peb.Set(keyTypeTrashSize(m.Type, m.IsTrash, m.Size, m.UUID), []byte{}, nil)
		_ = peb.Set(keyTypeTrashSizeDesc(m.Type, m.IsTrash, m.Size, m.UUID), []byte{}, nil)
		return nil
	}); err != nil {
		return err
	}
	return nil
}
