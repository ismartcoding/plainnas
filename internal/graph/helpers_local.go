package graph

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// copyFileContents copies a single file from src to dst.
func copyFileContents(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

type progressReader struct {
	r      io.Reader
	onRead func(n int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 && pr.onRead != nil {
		pr.onRead(int64(n))
	}
	return n, err
}

// copyFileContentsWithProgress copies a single file from src to dst and reports bytes copied.
// The callback is best-effort and may be called frequently.
func copyFileContentsWithProgress(src, dst string, onBytes func(n int64)) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	reader := &progressReader{r: in, onRead: onBytes}
	if _, err := io.Copy(out, reader); err != nil {
		return err
	}
	return out.Sync()
}

func makeUniquePathIfExists(dst string, treatAsFile bool) (string, error) {
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return dst, nil
	} else if err != nil {
		return "", err
	}
	base := filepath.Base(dst)
	dir := filepath.Dir(dst)
	name := base
	ext := ""
	if treatAsFile {
		if i := strings.LastIndex(base, "."); i > 0 {
			name = base[:i]
			ext = base[i:]
		}
	}
	for i := 1; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand, nil
		} else if err != nil {
			return "", err
		}
	}
}
