package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/db"
	plainfs "ismartcoding/plainnas/internal/fs"
	"ismartcoding/plainnas/internal/media"
)

func purgeMediaIndexByPathPrefix(p string) {
	prefix := filepath.ToSlash(p)
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if prefix == "" {
		return
	}
	_ = db.GetDefault().Iterate([]byte("media:path:"+prefix), func(_ []byte, value []byte) error {
		if uuid := string(value); uuid != "" {
			_ = media.DeleteMedia(uuid)
		}
		return nil
	})
}

func deleteFiles(paths []string) (bool, error) {
	if len(paths) == 0 {
		return true, nil
	}

	for _, raw := range paths {
		p := filepath.Clean(raw)
		if strings.TrimSpace(p) == "" || p == "." {
			return false, fmt.Errorf("invalid path")
		}
		// Safety guard: never allow deleting filesystem root.
		if p == string(os.PathSeparator) {
			return false, fmt.Errorf("refusing to delete root")
		}

		fi, err := os.Lstat(p)
		if err != nil {
			if plainfs.IsInNasTrash(p) {
				_ = plainfs.DeleteTrashByPath(p)
			}
			// If the file doesn't exist on disk, still try to remove any stale index entries.
			_ = media.RemovePath(p)
			// Also try to purge children by prefix (covers stale directory trees).
			purgeMediaIndexByPathPrefix(p)
			continue
		}

		if plainfs.IsInNasTrash(p) {
			if err := plainfs.DeleteTrashByPath(p); err != nil {
				return false, err
			}
			continue
		}

		if fi.IsDir() {
			// Delete physical directory tree first.
			if err := os.RemoveAll(p); err != nil {
				return false, err
			}
			// Purge any media index entries under this directory.
			purgeMediaIndexByPathPrefix(p)
			continue
		}

		// Regular file (or symlink): delete and remove from index.
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		_ = media.RemovePath(p)
	}

	return true, nil
}
