package helpers

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/pkg/log"
)

const (
	// dirChildrenThreshold is a hard cap for directory Children count.
	// If the real count exceeds this value, we return this value.
	dirChildrenThreshold = 10000
	// getdents buffer size (8KB-32KB tends to be a good range).
	getdentsBufferSize = 32768
)

// countDirEntriesFast counts directory entries using getdents(2).
// If the real count exceeds threshold, it returns (threshold, true, nil).
// It skips "." and "..".
func countDirEntriesFast(path string, threshold int) (count int, fuzzy bool, err error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return 0, false, err
	}
	defer syscall.Close(fd)

	buf := make([]byte, getdentsBufferSize)
	count = 0
	for {
		n, e := syscall.Getdents(fd, buf)
		if e != nil {
			return 0, false, e
		}
		if n == 0 {
			break
		}

		pos := 0
		for pos < n {
			dirent := (*syscall.Dirent)(unsafe.Pointer(&buf[pos]))
			reclen := int(dirent.Reclen)
			if reclen <= 0 {
				// Defensive: should never happen, but avoids infinite loops on corrupted data.
				return count, false, syscall.EIO
			}
			pos += reclen

			if dirent.Ino == 0 {
				continue
			}
			// Fast skip for "." and ".." without string allocation.
			if dirent.Name[0] == '.' {
				if dirent.Name[1] == 0 || (dirent.Name[1] == '.' && dirent.Name[2] == 0) {
					continue
				}
			}

			count++
			if count > threshold {
				return threshold, true, nil
			}
		}
	}

	return count, false, nil
}

func ListFiles(dir string, showHidden bool) []*model.File {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []*model.File{}
	}
	files := make([]*model.File, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(dir, name)
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfoToModel(full, info, e.IsDir()))
	}
	return files
}

func SearchFiles(text string, root string, showHidden bool) []*model.File {
	lower := strings.ToLower(text)
	var out []*model.File
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if text == "" || strings.Contains(strings.ToLower(name), lower) {
			if info, e := d.Info(); e == nil {
				out = append(out, FileInfoToModel(path, info, d.IsDir()))
			}
		}
		return nil
	})
	return out
}

func FileInfoToModel(path string, info os.FileInfo, isDir bool) *model.File {
	childCount := 0
	if isDir {
		if c, _, err := countDirEntriesFast(path, dirChildrenThreshold); err == nil {
			childCount = c
		}
	}
	return &model.File{
		Path:       filepath.ToSlash(path),
		IsDir:      isDir,
		CreatedAt:  info.ModTime(),
		UpdatedAt:  info.ModTime(),
		Size:       info.Size(),
		ChildCount: childCount,
	}
}

func SortFiles(items []*model.File, sortBy model.FileSortBy) {
	switch sortBy {
	case model.FileSortByDateAsc:
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].IsDir != items[j].IsDir {
				return items[i].IsDir && !items[j].IsDir
			}
			return items[i].UpdatedAt.Before(items[j].UpdatedAt)
		})
	case model.FileSortByDateDesc:
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].IsDir != items[j].IsDir {
				return items[i].IsDir && !items[j].IsDir
			}
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		})
	case model.FileSortBySizeAsc:
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].IsDir != items[j].IsDir {
				return items[i].IsDir && !items[j].IsDir
			}
			if items[i].IsDir {
				return strings.ToLower(filepath.Base(items[i].Path)) < strings.ToLower(filepath.Base(items[j].Path))
			}
			if items[i].Size != items[j].Size {
				return items[i].Size < items[j].Size
			}
			return strings.ToLower(filepath.Base(items[i].Path)) < strings.ToLower(filepath.Base(items[j].Path))
		})
	case model.FileSortBySizeDesc:
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].IsDir != items[j].IsDir {
				return items[i].IsDir && !items[j].IsDir
			}
			if items[i].IsDir {
				return strings.ToLower(filepath.Base(items[i].Path)) < strings.ToLower(filepath.Base(items[j].Path))
			}
			if items[i].Size != items[j].Size {
				return items[i].Size > items[j].Size
			}
			return strings.ToLower(filepath.Base(items[i].Path)) < strings.ToLower(filepath.Base(items[j].Path))
		})
	case model.FileSortByNameAsc:
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].IsDir != items[j].IsDir {
				return items[i].IsDir && !items[j].IsDir
			}
			return strings.ToLower(filepath.Base(items[i].Path)) < strings.ToLower(filepath.Base(items[j].Path))
		})
	case model.FileSortByNameDesc:
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].IsDir != items[j].IsDir {
				return items[i].IsDir && !items[j].IsDir
			}
			return strings.ToLower(filepath.Base(items[i].Path)) > strings.ToLower(filepath.Base(items[j].Path))
		})
	default:
	}
}

// FilterFilesBySize filters files based on size criteria
// When fileSize filter is active, directories are excluded from results
func FilterFilesBySize(files []*model.File, op string, filterSize int64) []*model.File {
	log.Debugf("[FilterFilesBySize] called: op=%s, filterSize=%d, inputCount=%d", op, filterSize, len(files))
	if filterSize == 0 || op == "" {
		log.Debugf("[FilterFilesBySize] no filtering (filterSize=%d, op=%s)", filterSize, op)
		return files
	}

	filtered := make([]*model.File, 0, len(files))
	for _, f := range files {
		if f == nil {
			continue
		}
		// When filtering by size, exclude directories (only show files)
		if f.IsDir {
			continue
		}

		if matchesFileSize(f.Size, op, filterSize) {
			filtered = append(filtered, f)
		}
	}
	log.Debugf("[FilterFilesBySize] result: filtered %d -> %d files (directories excluded)", len(files), len(filtered))
	return filtered
}

// matchesFileSize checks if a file size matches the filter
func matchesFileSize(size int64, op string, filterSize int64) bool {
	switch op {
	case ">":
		return size > filterSize
	case ">=":
		return size >= filterSize
	case "<":
		return size < filterSize
	case "<=":
		return size <= filterSize
	case "=", "":
		return size == filterSize
	case "!=":
		return size != filterSize
	default:
		return true
	}
}
