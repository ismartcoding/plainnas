package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/media"
)

type fileOpProgress struct {
	AddBytes func(n int64)
	AddItem  func()
}

func safeAddBytes(p *fileOpProgress, n int64) {
	if p != nil && p.AddBytes != nil && n > 0 {
		p.AddBytes(n)
	}
}

func safeAddItem(p *fileOpProgress) {
	if p != nil && p.AddItem != nil {
		p.AddItem()
	}
}

func copyFileOp(src string, dst string, overwrite bool) (bool, error) {
	return copyFileOpWithProgress(src, dst, overwrite, nil)
}

func copyFileOpWithProgress(src string, dst string, overwrite bool, progress *fileOpProgress) (bool, error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
		return false, fmt.Errorf("invalid arguments")
	}

	sfi, err := os.Stat(src)
	if err != nil {
		return false, err
	}

	// Destination resolution rules:
	// - If dst is an existing directory, copy src into dst/base(src).
	// - If src==dst ("duplicate folder"/"duplicate file"), copy to a sibling path with a unique name.
	resolvedDst := dst
	if dst == src {
		resolvedDst = filepath.Join(filepath.Dir(src), filepath.Base(src))
	} else if dfi, err := os.Stat(dst); err == nil && dfi.IsDir() {
		resolvedDst = filepath.Join(dst, filepath.Base(src))
	}

	if !overwrite {
		resolvedDst, err = makeUniquePathIfExists(resolvedDst, !sfi.IsDir())
		if err != nil {
			return false, err
		}
	}

	if sfi.IsDir() {
		srcAbs, err := filepath.Abs(src)
		if err != nil {
			return false, err
		}
		dstAbs, err := filepath.Abs(resolvedDst)
		if err != nil {
			return false, err
		}
		sep := string(os.PathSeparator)
		if strings.HasPrefix(dstAbs+sep, srcAbs+sep) {
			return false, fmt.Errorf("invalid destination: cannot copy a directory into itself")
		}
	}

	if sfi.IsDir() {
		// recursive copy
		err = filepath.WalkDir(src, func(p string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			rel, _ := filepath.Rel(src, p)
			target := filepath.Join(resolvedDst, rel)
			if d.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			if err := copyFileContentsWithProgress(p, target, func(n int64) {
				safeAddBytes(progress, n)
			}); err != nil {
				return err
			}
			safeAddItem(progress)
			return nil
		})
		if err != nil {
			return false, err
		}
	} else {
		if err := copyFileContentsWithProgress(src, resolvedDst, func(n int64) {
			safeAddBytes(progress, n)
		}); err != nil {
			return false, err
		}
		safeAddItem(progress)
	}

	_ = media.ScanFile(resolvedDst)
	return true, nil
}

func moveFileOp(src string, dst string, overwrite bool) (bool, error) {
	return moveFileOpWithProgress(src, dst, overwrite, nil)
}

func moveFileOpWithProgress(src string, dst string, overwrite bool, progress *fileOpProgress) (bool, error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	if strings.TrimSpace(src) == "" || strings.TrimSpace(dst) == "" {
		return false, fmt.Errorf("invalid arguments")
	}

	sfi, err := os.Stat(src)
	if err != nil {
		return false, err
	}

	// handle overwrite / unique
	if fi, err := os.Stat(dst); err == nil {
		if fi.IsDir() {
			dst = filepath.Join(dst, filepath.Base(src))
		}
		if !overwrite {
			base := filepath.Base(dst)
			dir := filepath.Dir(dst)
			name := base
			ext := ""
			if i := strings.LastIndex(base, "."); i > 0 {
				name = base[:i]
				ext = base[i:]
			}
			for i := 1; ; i++ {
				cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
				if _, e := os.Stat(cand); os.IsNotExist(e) {
					dst = cand
					break
				}
			}
		}
	}

	if err := os.Rename(src, dst); err != nil {
		// fallback cross-fs: copy then remove
		if sfi.IsDir() {
			// directories cannot be copied with copyFileContents; reuse copyFileOpWithProgress
			if _, err2 := copyFileOpWithProgress(src, dst, overwrite, progress); err2 != nil {
				return false, err
			}
		} else {
			if err2 := copyFileContentsWithProgress(src, dst, func(n int64) {
				safeAddBytes(progress, n)
			}); err2 != nil {
				return false, err
			}
			safeAddItem(progress)
		}
		_ = os.RemoveAll(src)
	} else {
		// rename succeeded; mark progress as completed for this entry
		if !sfi.IsDir() {
			safeAddBytes(progress, sfi.Size())
		}
		safeAddItem(progress)
	}

	_ = media.RemovePath(src)
	_ = media.ScanFile(dst)
	return true, nil
}
