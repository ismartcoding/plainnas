package media

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Public API

func DeleteFile(uuid string) error { return DeleteMedia(uuid) }

func SearchFiles(query string, filters map[string]string, offset int, limit int) ([]MediaFile, error) {
	return Search(query, filters, offset, limit)
}

func GetFileByUUID(uuid string) (*MediaFile, error) { return GetFile(uuid) }

// UpsertPath creates/updates a single file entry (used by watcher or uploads)
func UpsertPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return fmt.Errorf("unsupported fileinfo stat on %s", path)
	}
	ino := uint64(st.Ino)
	ctime := st.Ctim.Sec
	fsuuid, err := filesystemIDForPath(path)
	if err != nil {
		return err
	}
	if fsuuid == "" {
		fsuuid = string(filepath.Separator)
	}
	id := uuidFromTriplet(fsuuid, ino, ctime)
	name := filepath.Base(path)
	m := &MediaFile{
		UUID:         id,
		FSUUID:       fsuuid,
		Ino:          ino,
		Ctime:        ctime,
		Path:         filepath.ToSlash(path),
		OriginalPath: filepath.ToSlash(path),
		Name:         name,
		Size:         info.Size(),
		ModifiedAt:   info.ModTime().Unix(),
		Type:         inferType(name),
	}
	if ex, _ := FindUUIDByFID(fsuuid, ino, ctime); ex != "" && ex != id {
		m.UUID = ex
	}
	return UpsertMedia(m)
}

// ScanFile indexes a single file immediately (similar to Android's MediaScannerConnection.scanFile)
func ScanFile(path string) error {
	if err := UpsertPath(path); err != nil {
		return err
	}
	return FlushMediaIndexBatch()
}

// ScanFiles indexes multiple files immediately and flushes once at the end
func ScanFiles(paths []string) error {
	for _, p := range paths {
		_ = UpsertPath(p)
	}
	return FlushMediaIndexBatch()
}

// RemovePath deletes a file entry by path if exists.
func RemovePath(path string) error {
	if uuid, _ := FindByPath(filepath.ToSlash(path)); uuid != "" {
		return DeleteMedia(uuid)
	}
	return fmt.Errorf("not found")
}
