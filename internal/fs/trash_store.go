package fs

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"ismartcoding/plainnas/internal/db"
)

func trashItemKey(id string) string {
	return "trash:item:" + id
}

func trashDeletedAtIndexKey(deletedAt int64, id string) string {
	// Newest-first ordering using reversed timestamp.
	rev := int64(math.MaxInt64) - deletedAt
	return fmt.Sprintf("trash:by_deleted_at:%020d:%s", rev, id)
}

func trashDeletedAtIndexPrefix() []byte {
	return []byte("trash:by_deleted_at:")
}

func trashDirFlag(it *TrashItem) string {
	if it != nil && it.Type == "dir" {
		return "0"
	}
	return "1"
}

func trashDeletedAtDirFirstIndexKey(deletedAt int64, dirFlag string, id string) string {
	// Newest-first ordering within each group using reversed timestamp.
	rev := int64(math.MaxInt64) - deletedAt
	return fmt.Sprintf("trash:by_deleted_at_df:%s:%020d:%s", dirFlag, rev, id)
}

func trashDeletedAtDirFirstIndexPrefix(dirFlag string) []byte {
	if dirFlag != "" {
		return []byte("trash:by_deleted_at_df:" + dirFlag + ":")
	}
	return []byte("trash:by_deleted_at_df:")
}

func trashNameDirFirstIndexKey(dirFlag string, nameLower string, id string) string {
	// Stable ordering by name within each group; ID breaks ties.
	return "trash:by_name_df:" + dirFlag + ":" + nameLower + ":" + id
}

func trashNameDirFirstIndexPrefix(dirFlag string) []byte {
	if dirFlag != "" {
		return []byte("trash:by_name_df:" + dirFlag + ":")
	}
	return []byte("trash:by_name_df:")
}

func trashSizeDirFirstIndexKey(dirFlag string, size int64, id string) string {
	// Stable numeric ordering by size within each group; ID breaks ties.
	return fmt.Sprintf("trash:by_size_df:%s:%020d:%s", dirFlag, size, id)
}

func trashSizeDirFirstIndexPrefix(dirFlag string) []byte {
	if dirFlag != "" {
		return []byte("trash:by_size_df:" + dirFlag + ":")
	}
	return []byte("trash:by_size_df:")
}

func trashDisplayNameLower(it *TrashItem) string {
	if it == nil {
		return ""
	}
	base := filepath.Base(it.OriginalPath)
	return strings.ToLower(strings.TrimSpace(base))
}

func trashSortSize(it *TrashItem) int64 {
	if it == nil || it.Size == nil {
		return 0
	}
	return *it.Size
}

func loadTrashItem(id string) (*TrashItem, error) {
	var it TrashItem
	err := db.GetDefault().LoadJSON(trashItemKey(id), &it)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(it.ID) == "" {
		return nil, nil
	}
	return &it, nil
}

func storeTrashItem(it *TrashItem) error {
	if it == nil || strings.TrimSpace(it.ID) == "" {
		return fmt.Errorf("invalid trash item")
	}

	setIndexes := func(v *TrashItem) error {
		if err := db.GetDefault().Set([]byte(trashDeletedAtIndexKey(v.DeletedAt, v.ID)), []byte(v.ID), nil); err != nil {
			return err
		}
		dirFlag := trashDirFlag(v)
		nameKey := trashDisplayNameLower(v)
		sizeKey := trashSortSize(v)
		if err := db.GetDefault().Set([]byte(trashDeletedAtDirFirstIndexKey(v.DeletedAt, dirFlag, v.ID)), []byte(v.ID), nil); err != nil {
			return err
		}
		if err := db.GetDefault().Set([]byte(trashNameDirFirstIndexKey(dirFlag, nameKey, v.ID)), []byte(v.ID), nil); err != nil {
			return err
		}
		if err := db.GetDefault().Set([]byte(trashSizeDirFirstIndexKey(dirFlag, sizeKey, v.ID)), []byte(v.ID), nil); err != nil {
			return err
		}
		return nil
	}

	// Root-cause fix: Trash items can be updated (e.g. directory entry count, file size).
	// When the indexed fields change, old index keys must be removed, otherwise listings
	// (especially size/name) will return duplicates.
	old, _ := loadTrashItem(it.ID)
	if old != nil {
		_ = deleteTrashItemIndexes(old)
	}

	// Write item first (authoritative), then indexes for listing/GC.
	if err := db.GetDefault().StoreJSON(trashItemKey(it.ID), it); err != nil {
		// Best-effort restore old state.
		if old != nil {
			_ = setIndexes(old)
		}
		return err
	}

	if err := setIndexes(it); err != nil {
		// Roll back to old record if possible.
		_ = deleteTrashItemIndexes(it)
		_ = db.GetDefault().DeleteByKey(trashItemKey(it.ID))
		if old != nil {
			_ = db.GetDefault().StoreJSON(trashItemKey(old.ID), old)
			_ = setIndexes(old)
		}
		return err
	}
	return nil
}

func deleteTrashItemIndexes(it *TrashItem) error {
	if it == nil || it.ID == "" {
		return nil
	}
	if err := db.GetDefault().DeleteByKey(trashDeletedAtIndexKey(it.DeletedAt, it.ID)); err != nil {
		return err
	}
	dirFlag := trashDirFlag(it)
	nameKey := trashDisplayNameLower(it)
	sizeKey := trashSortSize(it)
	_ = db.GetDefault().DeleteByKey(trashDeletedAtDirFirstIndexKey(it.DeletedAt, dirFlag, it.ID))
	_ = db.GetDefault().DeleteByKey(trashNameDirFirstIndexKey(dirFlag, nameKey, it.ID))
	_ = db.GetDefault().DeleteByKey(trashSizeDirFirstIndexKey(dirFlag, sizeKey, it.ID))
	return nil
}

func deleteTrashItemKeys(it *TrashItem) error {
	if it == nil || it.ID == "" {
		return nil
	}
	if err := db.GetDefault().DeleteByKey(trashItemKey(it.ID)); err != nil {
		return err
	}
	return deleteTrashItemIndexes(it)
}

func countTrashItems() (int, error) {
	cnt := 0
	err := db.GetDefault().Iterate(trashDeletedAtIndexPrefix(), func(_ []byte, _ []byte) error {
		cnt++
		return nil
	})
	if err != nil {
		return 0, err
	}
	return cnt, nil
}

type trashIterFn func(prefix []byte, fn func(key []byte, value []byte) error) error

func listTrashItemsDirFirstFromIndex(dirPrefix []byte, dirIter trashIterFn, filePrefix []byte, fileIter trashIterFn, offset, limit int, text string) ([]*TrashItem, error) {
	if limit <= 0 {
		limit = 200
	}
	wantText := strings.ToLower(strings.TrimSpace(text))
	out := make([]*TrashItem, 0, limit)
	seen := 0

	collect := func(prefix []byte, iter trashIterFn) error {
		return iter(prefix, func(key []byte, _ []byte) error {
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
			if wantText != "" {
				p := strings.ToLower(it.OriginalPath)
				if !strings.Contains(p, wantText) && !strings.Contains(strings.ToLower(filepath.Base(p)), wantText) {
					return nil
				}
			}
			if seen < offset {
				seen++
				return nil
			}
			if len(out) >= limit {
				return db.ErrIterateStop
			}
			out = append(out, it)
			return nil
		})
	}

	if err := collect(dirPrefix, dirIter); err != nil {
		return nil, err
	}
	if len(out) >= limit {
		return out, nil
	}
	if err := collect(filePrefix, fileIter); err != nil {
		return nil, err
	}
	return out, nil
}

func listTrashItemsDirFirstNewestFirst(offset, limit int, text string) ([]*TrashItem, error) {
	return listTrashItemsDirFirstFromIndex(
		trashDeletedAtDirFirstIndexPrefix("0"),
		db.GetDefault().Iterate,
		trashDeletedAtDirFirstIndexPrefix("1"),
		db.GetDefault().Iterate,
		offset,
		limit,
		text,
	)
}

func listTrashItemsDirFirstOldestFirst(offset, limit int, text string) ([]*TrashItem, error) {
	// Index uses reversed timestamps, so reverse-iteration yields oldest-first.
	return listTrashItemsDirFirstFromIndex(
		trashDeletedAtDirFirstIndexPrefix("0"),
		db.GetDefault().IterateReverse,
		trashDeletedAtDirFirstIndexPrefix("1"),
		db.GetDefault().IterateReverse,
		offset,
		limit,
		text,
	)
}

func listTrashItemsByName(offset, limit int, text string, desc bool) ([]*TrashItem, error) {
	iter := db.GetDefault().Iterate
	if desc {
		iter = db.GetDefault().IterateReverse
	}
	return listTrashItemsDirFirstFromIndex(
		trashNameDirFirstIndexPrefix("0"),
		iter,
		trashNameDirFirstIndexPrefix("1"),
		iter,
		offset,
		limit,
		text,
	)
}

func listTrashItemsBySize(offset, limit int, text string, desc bool) ([]*TrashItem, error) {
	// Requirement: directories do NOT participate in size sorting.
	// Keep directories ordered by name (A-Z), while files are ordered by size.
	fileIter := db.GetDefault().Iterate
	if desc {
		fileIter = db.GetDefault().IterateReverse
	}
	return listTrashItemsDirFirstFromIndex(
		trashNameDirFirstIndexPrefix("0"),
		db.GetDefault().Iterate,
		trashSizeDirFirstIndexPrefix("1"),
		fileIter,
		offset,
		limit,
		text,
	)
}

func sanitizeTrashBaseName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "unknown"
	}
	// Disallow path separators just in case.
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, string(filepath.Separator), "_")
	return name
}

func computeBucketRelPath(kind string, id string, baseName string, now time.Time) (string, error) {
	if kind != "file" && kind != "dir" {
		return "", fmt.Errorf("invalid kind")
	}
	y := now.Format("2006")
	m := now.Format("01")
	prefix := "f_"
	if kind == "dir" {
		prefix = "d_"
	}
	baseName = sanitizeTrashBaseName(baseName)
	// Keep ID parseable from prefix, but embed basename for nicer UI.
	name := prefix + id + "__" + baseName
	return filepath.Join("data", y, m, name), nil
}

func trashAbsPath(diskMount string, rel string) string {
	return filepath.Join(diskMount, ".nas-trash", rel)
}

func parseTrashIDFromPath(p string) (id string, ok bool) {
	base := filepath.Base(filepath.Clean(p))
	if strings.HasPrefix(base, "f_") || strings.HasPrefix(base, "d_") {
		rest := strings.TrimSpace(base[2:])
		// Support f_<id>__<basename>
		if before, _, found := strings.Cut(rest, "__"); found {
			rest = before
		}
		id = strings.TrimSpace(rest)
		return id, id != ""
	}
	return "", false
}

func isInNasTrash(p string) bool {
	p = filepath.Clean(p)
	sep := string(filepath.Separator)
	needle := sep + ".nas-trash" + sep
	return strings.Contains(p, needle) || strings.HasSuffix(p, sep+".nas-trash")
}
