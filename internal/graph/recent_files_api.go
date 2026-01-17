package graph

import (
	"os"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/helpers"
	"ismartcoding/plainnas/internal/graph/model"
)

func recentFiles() ([]*model.File, error) {
	paths := db.GetRecentFiles(500)
	items := make([]*model.File, 0, len(paths))
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil {
			items = append(items, helpers.FileInfoToModel(p, info, info.IsDir()))
		}
	}
	return items, nil
}

func recentFilesCount() (int, error) {
	paths := db.GetRecentFiles(500)
	cnt := 0
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			cnt++
		}
	}
	return cnt, nil
}
