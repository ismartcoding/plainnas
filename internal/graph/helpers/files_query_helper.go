package helpers

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	plainfs "ismartcoding/plainnas/internal/fs"
	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/pkg/log"
	"ismartcoding/plainnas/internal/search"
)

type FilesQuery struct {
	ShowHidden   bool
	Text         string
	RootPath     string
	RelativePath string
	TrashOnly    bool
	FileSizeOp   string
	FileSizeVal  int64
}

func ParseFilesQuery(q string) FilesQuery {
	fields := search.Parse(q)
	out := FilesQuery{}
	for _, f := range fields {
		switch f.Name {
		case "show_hidden":
			out.ShowHidden = f.Value == "true"
		case "text":
			out.Text = f.Value
		case "root_path":
			out.RootPath = f.Value
		case "relative_path":
			out.RelativePath = f.Value
		case "trash":
			out.TrashOnly = f.Value == "true"
		case "file_size":
			out.FileSizeOp = f.Op
			out.FileSizeVal = parseFileSize(f.Value)
			log.Debugf("[ParseFilesQuery] file_size detected - Op: %s, Value: %s, Parsed: %d bytes", f.Op, f.Value, out.FileSizeVal)
		}
	}
	if out.TrashOnly {
		// Trash view always includes hidden entries.
		out.ShowHidden = true
	}
	log.Debugf("[ParseFilesQuery] FileSizeOp=%s, FileSizeVal=%d", out.FileSizeOp, out.FileSizeVal)
	return out
}

// parseFileSize parses size strings like "1MB", "100KB", "1GB" to bytes
func parseFileSize(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0
	}

	multiplier := int64(1)
	if strings.HasSuffix(s, "GB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	} else if strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	} else if strings.HasSuffix(s, "KB") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	} else if strings.HasSuffix(s, "B") {
		s = strings.TrimSuffix(s, "B")
	}

	val, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return val * multiplier
}

func BuildBaseDir(rootPath, relativePath string) string {
	base := rootPath
	if relativePath != "" {
		if after, ok := strings.CutPrefix(relativePath, "/"); ok {
			relativePath = after
		}
		base = filepath.Join(base, filepath.FromSlash(relativePath))
	}
	return base
}

func normalizeSlashDir(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(filepath.Clean(filepath.FromSlash(p)))
	if p == "." {
		return ""
	}
	if p != "/" {
		p = strings.TrimRight(p, "/")
	}
	return p
}

func TrashBaseFilter(rootPath, relativePath string) string {
	// If not specified, keep the legacy behavior: return the flat trash listing.
	if strings.TrimSpace(rootPath) == "" && strings.TrimSpace(relativePath) == "" {
		return ""
	}
	return normalizeSlashDir(BuildBaseDir(rootPath, relativePath))
}

func trashItemToModel(it *plainfs.TrashItem) *model.File {
	if it == nil {
		return nil
	}
	p := filepath.ToSlash(filepath.Join(it.Disk, ".nas-trash", filepath.FromSlash(it.TrashRelPath)))
	isDir := it.Type == "dir"
	t := time.Unix(it.DeletedAt, 0)
	sz := int64(0)
	if it.Size != nil {
		sz = *it.Size
	}
	childCount := 0
	if isDir && it.EntryCount != nil {
		// entry_count includes the directory itself.
		if v := *it.EntryCount - 1; v > 0 {
			childCount = int(v)
		}
	}
	return &model.File{
		Path:       p,
		IsDir:      isDir,
		CreatedAt:  t,
		UpdatedAt:  t,
		Size:       sz,
		ChildCount: childCount,
	}
}

func trashDisplayNameFromPath(p string) string {
	name := filepath.Base(filepath.FromSlash(p))
	if strings.HasPrefix(name, "f_") || strings.HasPrefix(name, "d_") {
		if idx := strings.Index(name, "__"); idx >= 0 && idx+2 < len(name) {
			rest := name[idx+2:]
			if rest != "" {
				return rest
			}
		}
	}
	return name
}

func sortTrashFiles(items []*model.File, sortBy model.FileSortBy) {
	switch sortBy {
	case model.FileSortByNameAsc:
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].IsDir != items[j].IsDir {
				return items[i].IsDir && !items[j].IsDir
			}
			return strings.ToLower(trashDisplayNameFromPath(items[i].Path)) < strings.ToLower(trashDisplayNameFromPath(items[j].Path))
		})
	case model.FileSortByNameDesc:
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].IsDir != items[j].IsDir {
				return items[i].IsDir && !items[j].IsDir
			}
			return strings.ToLower(trashDisplayNameFromPath(items[i].Path)) > strings.ToLower(trashDisplayNameFromPath(items[j].Path))
		})
	default:
		SortFiles(items, sortBy)
	}
}

func ListTrashFiles(offset, limit int, text string, baseFilter string, sortBy model.FileSortBy) ([]*model.File, error) {
	baseFilter = normalizeSlashDir(baseFilter)
	if baseFilter == "" {
		var items []*plainfs.TrashItem
		var err error
		switch sortBy {
		case model.FileSortByDateAsc:
			items, err = plainfs.ListTrashOldestFirst(offset, limit, text)
		case model.FileSortByNameAsc:
			items, err = plainfs.ListTrashByName(offset, limit, text, false)
		case model.FileSortByNameDesc:
			items, err = plainfs.ListTrashByName(offset, limit, text, true)
		case model.FileSortBySizeAsc:
			items, err = plainfs.ListTrashBySize(offset, limit, text, false)
		case model.FileSortBySizeDesc:
			items, err = plainfs.ListTrashBySize(offset, limit, text, true)
		case model.FileSortByDateDesc:
			fallthrough
		default:
			items, err = plainfs.ListTrash(offset, limit, text)
		}
		if err != nil {
			return nil, err
		}
		out := make([]*model.File, 0, len(items))
		for _, it := range items {
			mf := trashItemToModel(it)
			if mf != nil {
				out = append(out, mf)
			}
		}
		return out, nil
	}

	// Browsing inside a trashed directory cannot be served from the trash index:
	// the Pebble metadata is stored per top-level trashed path (file/dir), not for
	// the directory's child entries. For a concrete baseFilter, list the physical
	// directory contents directly.
	dir := filepath.FromSlash(baseFilter)
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		items := ListFiles(dir, true)
		if strings.TrimSpace(text) != "" {
			needle := strings.ToLower(strings.TrimSpace(text))
			filtered := make([]*model.File, 0, len(items))
			for _, it := range items {
				if it == nil {
					continue
				}
				name := strings.ToLower(filepath.Base(filepath.FromSlash(it.Path)))
				if strings.Contains(name, needle) {
					filtered = append(filtered, it)
				}
			}
			items = filtered
		}
		sortTrashFiles(items, sortBy)
		if offset > 0 && offset < len(items) {
			items = items[offset:]
		} else if offset >= len(items) {
			return []*model.File{}, nil
		}
		if limit > 0 && limit < len(items) {
			items = items[:limit]
		}
		return items, nil
	}

	// Filtered trash browsing: iterate in pages and collect matches.
	// We keep this metadata-driven (no filesystem traversal), but we do need
	// to scan pages because the DB index is ordered by deletedAt, not by folder.
	effectiveLimit := limit
	if effectiveLimit <= 0 {
		effectiveLimit = 200 // matches internal/fs default
	}
	need := offset + effectiveLimit

	pageSize := 200
	if effectiveLimit > pageSize {
		pageSize = effectiveLimit
	}

	matches := make([]*model.File, 0, effectiveLimit)
	fetchOffset := 0
	for i := 0; i < 20 && len(matches) < need; i++ {
		page, err := plainfs.ListTrash(fetchOffset, pageSize, text)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		for _, it := range page {
			mf := trashItemToModel(it)
			if mf == nil {
				continue
			}
			parent := normalizeSlashDir(filepath.ToSlash(filepath.Dir(filepath.FromSlash(mf.Path))))
			if parent == baseFilter {
				matches = append(matches, mf)
			}
		}
		fetchOffset += len(page)
		if len(page) < pageSize {
			break
		}
	}

	sortTrashFiles(matches, sortBy)
	if offset > 0 && offset < len(matches) {
		matches = matches[offset:]
	} else if offset >= len(matches) {
		return []*model.File{}, nil
	}
	if limit > 0 && limit < len(matches) {
		matches = matches[:limit]
	}
	return matches, nil
}

func SearchIndexFiles(text string, base string, offset int, limit int, showHidden bool, sizeOp string, sizeBytes int64) ([]*model.File, error) {
	parent := normalizeSlashDir(base)

	// Use SearchIndex with size filter params
	paths, err := search.SearchIndex(text, parent, offset, limit, sizeOp, uint64(sizeBytes))

	if err != nil {
		return nil, err
	}

	if len(paths) > 0 {
		out := make([]*model.File, 0, len(paths))
		for _, p := range paths {
			if info, err := os.Stat(p); err == nil {
				out = append(out, FileInfoToModel(p, info, info.IsDir()))
			}
		}
		return out, nil
	}

	// If index yields nothing, only fall back to file walk if the user did NOT specify text or file size filter.
	// If either text or file size filter is set, never do this for a global search (parent == "") to avoid full scans.
	if strings.TrimSpace(text) != "" || sizeOp != "" {
		return []*model.File{}, nil
	}

	items := SearchFiles(text, filepath.FromSlash(parent), showHidden)
	if offset > 0 && offset < len(items) {
		items = items[offset:]
	} else if offset >= len(items) {
		return []*model.File{}, nil
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items, nil
}
