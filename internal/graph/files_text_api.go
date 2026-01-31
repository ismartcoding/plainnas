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

const maxWriteTextBytes = 2 * 1024 * 1024

func writeTextFileModel(ctx context.Context, path string, content string, overwrite bool) (*model.File, error) {
	p := filepath.Clean(path)
	if strings.TrimSpace(p) == "" {
		return nil, fmt.Errorf("path is empty")
	}
	if len(content) > maxWriteTextBytes {
		return nil, fmt.Errorf("content too large")
	}

	parent := filepath.Dir(p)
	pfi, err := os.Stat(parent)
	if err != nil {
		return nil, err
	}
	if !pfi.IsDir() {
		return nil, fmt.Errorf("parent is not a directory")
	}

	if fi, err := os.Stat(p); err == nil {
		if fi.IsDir() {
			return nil, fmt.Errorf("path is a directory")
		}
		if !overwrite {
			return nil, fmt.Errorf("target exists")
		}
	}

	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	_, werr := f.WriteString(content)
	cerr := f.Close()
	if werr != nil {
		return nil, werr
	}
	if cerr != nil {
		return nil, cerr
	}

	_ = media.ScanFile(p)

	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	return helpers.FileInfoToModel(p, info, true), nil
}
