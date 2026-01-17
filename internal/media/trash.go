package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ismartcoding/plainnas/internal/db"
	plainfs "ismartcoding/plainnas/internal/fs"
)

// TrashUUID moves a file to trash and updates metadata/index.
func TrashUUID(uuid string) error {
	mf, err := GetFile(uuid)
	if err != nil || mf == nil {
		return fmt.Errorf("not found")
	}
	if mf.IsTrash {
		return nil
	}
	src := mf.Path
	if _, err := os.Stat(src); err != nil {
		return err
	}
	trashed, err := plainfs.TrashPaths([]string{src})
	if err != nil {
		return err
	}
	if len(trashed) != 1 {
		return fmt.Errorf("trash failed")
	}
	mf.Path = filepath.ToSlash(trashed[0])
	mf.TrashPath = mf.Path
	if mf.OriginalPath == "" {
		mf.OriginalPath = filepath.ToSlash(src)
	}
	mf.IsTrash = true
	mf.DeletedAt = time.Now().Unix()
	// When an item is moved to trash, tag relations must be removed.
	_ = db.DeleteTagRelationsByKeys([]string{uuid})
	return UpsertMedia(mf)
}

// RestoreUUID restores a file from trash back to its original path.
func RestoreUUID(uuid string) error {
	mf, err := GetFile(uuid)
	if err != nil || mf == nil {
		return fmt.Errorf("not found")
	}
	if !mf.IsTrash {
		return nil
	}
	restored, err := plainfs.RestorePaths([]string{mf.Path})
	if err != nil {
		return err
	}
	if len(restored) != 1 {
		return fmt.Errorf("restore failed")
	}
	mf.Path = filepath.ToSlash(restored[0])
	mf.IsTrash = false
	mf.TrashPath = ""
	mf.DeletedAt = 0
	return UpsertMedia(mf)
}

// DeleteUUIDPermanently removes the physical file and all metadata/index.
func DeleteUUIDPermanently(uuid string) error {
	mf, err := GetFile(uuid)
	if err != nil || mf == nil {
		return fmt.Errorf("not found")
	}
	// If it's a trashed item, delete via trash metadata; otherwise remove directly.
	if p := mf.Path; strings.TrimSpace(p) != "" {
		if plainfs.IsInNasTrash(p) {
			_ = plainfs.DeleteTrashByPath(p)
		} else {
			_ = os.Remove(p)
		}
	}
	return DeleteMedia(uuid)
}
