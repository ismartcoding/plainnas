package graph

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/media"
)

func mergeChunks(fileID string, totalChunks int, path string, replace bool) (string, error) {
	if strings.TrimSpace(fileID) == "" || strings.TrimSpace(path) == "" || totalChunks <= 0 {
		return "", fmt.Errorf("invalid arguments")
	}

	base := filepath.Join(consts.DATA_DIR, "upload_tmp", fileID)
	// ensure destination path
	dest := filepath.Clean(path)
	if !replace {
		if _, err := os.Stat(dest); err == nil {
			dir := filepath.Dir(dest)
			baseName := filepath.Base(dest)
			name := baseName
			ext := ""
			if i := strings.LastIndex(baseName, "."); i > 0 {
				name = baseName[:i]
				ext = baseName[i:]
			}
			for i := 1; ; i++ {
				cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
				if _, e := os.Stat(cand); os.IsNotExist(e) {
					dest = cand
					break
				}
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	out, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer out.Close()

	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(base, fmt.Sprintf("chunk_%d", i))
		f, e := os.Open(chunkPath)
		if e != nil {
			return "", fmt.Errorf("missing chunk %d", i)
		}
		if _, e = io.Copy(out, f); e != nil {
			f.Close()
			return "", e
		}
		f.Close()
	}

	// cleanup chunk dir
	_ = os.RemoveAll(base)
	// index media
	_ = media.ScanFile(dest)
	return filepath.Base(dest), nil
}
