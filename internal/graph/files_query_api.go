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
		items, err := helpers.ListTrashFiles(offset, limit, q.Text, baseFilter, sortBy)
		if err != nil {
			return nil, err
		}
		return helpers.FilterFilesBySize(items, q.FileSizeOp, q.FileSizeVal), nil
	}

	base := helpers.BuildBaseDir(q.RootPath, q.RelativePath)

	// Use index search if we have text search OR file size filter
	if q.Text != "" || (q.FileSizeOp != "" && q.FileSizeVal > 0) {
		items, err := helpers.SearchIndexFiles(q.Text, base, offset, limit, q.ShowHidden, q.FileSizeOp, q.FileSizeVal)
		if err != nil {
			return nil, err
		}
		// SearchIndexFiles already applies size filter via index
		return items, nil
	}

	// No filters - just list files
	if sortBy == model.FileSortByNameAsc || sortBy == model.FileSortByNameDesc {
		items := helpers.ListFilesPaged(base, q.ShowHidden, offset, limit, sortBy)
		return items, nil
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
