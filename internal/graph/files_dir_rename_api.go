package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/graph/helpers"
	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/media"
)

func createDirModel(ctx context.Context, path string) (*model.File, error) {
	p := filepath.Clean(path)
	if strings.TrimSpace(p) == "" {
		return nil, fmt.Errorf("path is empty")
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		return nil, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	return helpers.FileInfoToModel(p, info, true), nil
}

func renameFileModel(ctx context.Context, path string, name string) (bool, error) {
	src := filepath.Clean(path)
	if strings.TrimSpace(src) == "" || strings.TrimSpace(name) == "" {
		return false, fmt.Errorf("invalid arguments")
	}
	dst := filepath.Join(filepath.Dir(src), name)
	if _, err := os.Stat(dst); err == nil {
		return false, fmt.Errorf("target exists")
	}
	if err := os.Rename(src, dst); err != nil {
		return false, err
	}
	_ = media.RemovePath(src)
	_ = media.ScanFile(dst)
	return true, nil
}
