package media

import (
	"fmt"

	"github.com/cespare/xxhash/v2"
)

// Pebble key builders
func keyByUUID(uuid string) []byte { return []byte("media:uuid:" + uuid) }

func fidHash(fsUUID string) uint64 {
	return xxhash.Sum64String(fsUUID)
}

func keyByFID(fsUUID string, ino uint64, ctime int64) []byte {
	// fsUUID may contain ':' (e.g. remote mounts). Hash it so keys are safe and fixed-size.
	return []byte(fmt.Sprintf("media:fid:%016x:%d:%d", fidHash(fsUUID), ino, ctime))
}

func keyByPath(path string) []byte { return []byte("media:path:" + path) }

func keyByDocID(id uint64) []byte { return []byte(fmt.Sprintf("media:docid:%016x", id)) }
