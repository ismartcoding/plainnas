package graph

import (
	"os"
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/graph/model"
)

func pathStatModel(path string) (*model.PathStat, error) {
	p := filepath.Clean(path)
	if strings.TrimSpace(p) == "" || p == "." {
		return &model.PathStat{Exists: false, IsDir: false}, nil
	}

	fi, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.PathStat{Exists: false, IsDir: false}, nil
		}
		return nil, err
	}

	return &model.PathStat{Exists: true, IsDir: fi.IsDir()}, nil
}

func pathStatsModel(paths []string) ([]*model.PathStatResult, error) {
	out := make([]*model.PathStatResult, 0, len(paths))
	for _, raw := range paths {
		p := filepath.Clean(raw)
		if strings.TrimSpace(p) == "" || p == "." {
			out = append(out, &model.PathStatResult{Path: raw, Exists: false, IsDir: false})
			continue
		}
		fi, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				out = append(out, &model.PathStatResult{Path: raw, Exists: false, IsDir: false})
				continue
			}
			return nil, err
		}
		out = append(out, &model.PathStatResult{Path: raw, Exists: true, IsDir: fi.IsDir()})
	}
	return out, nil
}
