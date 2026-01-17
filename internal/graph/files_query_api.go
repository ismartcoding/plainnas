package graph

import (
	"ismartcoding/plainnas/internal/graph/helpers"
	"ismartcoding/plainnas/internal/graph/model"
)

func files(offset int, limit int, query string, sortBy model.FileSortBy) ([]*model.File, error) {
	q := helpers.ParseFilesQuery(query)
	if q.TrashOnly {
		// Metadata-driven trash listing (with optional directory browsing).
		baseFilter := helpers.TrashBaseFilter(q.RootPath, q.RelativePath)
		return helpers.ListTrashFiles(offset, limit, q.Text, baseFilter, sortBy)
	}

	base := helpers.BuildBaseDir(q.RootPath, q.RelativePath)
	if q.Text != "" {
		return helpers.SearchIndexFiles(q.Text, base, offset, limit, q.ShowHidden)
	}

	items := helpers.ListFiles(base, q.ShowHidden)
	helpers.SortFiles(items, sortBy)
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
