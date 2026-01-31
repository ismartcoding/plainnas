package fs

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"ismartcoding/plainnas/internal/consts"
)

var (
	ErrPreviewNotSupported = errors.New("preview not supported")
	ErrPreviewToolMissing  = errors.New("preview tool missing")
)

var pdfPreviewLocks sync.Map // map[string]*sync.Mutex

func isOfficeDocLike(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx":
		return true
	default:
		return false
	}
}

func pdfPreviewCacheDir() string {
	return filepath.Join(consts.DATA_DIR, "preview_pdf")
}

func pdfPreviewKey(path string, modUnix int64, size int64) string {
	h := sha1.New()
	_, _ = h.Write([]byte(path))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(fmt.Sprintf("%d|%d", modUnix, size)))
	return hex.EncodeToString(h.Sum(nil))
}

func lockForKey(key string) *sync.Mutex {
	v, _ := pdfPreviewLocks.LoadOrStore(key, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func findLibreOfficeBinary() (string, error) {
	// Prefer PATH-based lookup first.
	if p, err := exec.LookPath("soffice"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("libreoffice"); err == nil {
		return p, nil
	}

	// systemd services may run with a minimal PATH; probe common locations.
	// (Ubuntu/Debian, Arch, Fedora, snap, etc.)
	candidates := []string{
		"/usr/bin/soffice",
		"/usr/bin/libreoffice",
		"/usr/lib/libreoffice/program/soffice",
		"/usr/lib64/libreoffice/program/soffice",
		"/snap/bin/libreoffice",
		"/var/lib/snapd/snap/bin/libreoffice",
		"/app/bin/libreoffice", // flatpak
		"/app/bin/soffice",     // flatpak
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}

	return "", fmt.Errorf("%w (searched PATH plus common locations)", ErrPreviewToolMissing)
}

func LibreOfficeAvailable() bool {
	_, err := findLibreOfficeBinary()
	return err == nil
}

// GetOrCreatePDFPreview converts an Office document into a cached PDF and returns the cached PDF path.
// Cache key includes source path + mtime + size, so updates invalidate automatically.
func GetOrCreatePDFPreview(srcPath string, modUnix int64, size int64) (string, error) {
	if !isOfficeDocLike(srcPath) {
		return "", ErrPreviewNotSupported
	}

	key := pdfPreviewKey(srcPath, modUnix, size)
	cacheDir := pdfPreviewCacheDir()
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}

	outPath := filepath.Join(cacheDir, key+".pdf")
	if fi, err := os.Stat(outPath); err == nil && !fi.IsDir() && fi.Size() > 0 {
		return outPath, nil
	}

	mu := lockForKey(key)
	mu.Lock()
	defer mu.Unlock()

	if fi, err := os.Stat(outPath); err == nil && !fi.IsDir() && fi.Size() > 0 {
		return outPath, nil
	}

	bin, err := findLibreOfficeBinary()
	if err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp(cacheDir, "conv-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	// LibreOffice writes the PDF into outdir named after the input basename.
	base := filepath.Base(srcPath)
	pdfName := strings.TrimSuffix(base, filepath.Ext(base)) + ".pdf"
	tmpPDF := filepath.Join(tmpDir, pdfName)

	cmd := exec.Command(bin,
		"--headless",
		"--nologo",
		"--nofirststartwizard",
		"--norestore",
		"--convert-to",
		"pdf",
		"--outdir",
		tmpDir,
		srcPath,
	)
	// Keep LO user profile isolated per run to avoid cross-request state and permission issues.
	cmd.Env = append(os.Environ(), "HOME="+tmpDir)

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("libreoffice convert failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	fi, err := os.Stat(tmpPDF)
	if err != nil || fi.IsDir() || fi.Size() == 0 {
		return "", fmt.Errorf("libreoffice produced no pdf")
	}

	if err := os.Rename(tmpPDF, outPath); err != nil {
		// Rename can fail across devices; fall back to copy.
		in, e1 := os.Open(tmpPDF)
		if e1 != nil {
			return "", err
		}
		defer in.Close()

		out, e2 := os.Create(outPath)
		if e2 != nil {
			return "", err
		}
		if _, e3 := io.Copy(out, in); e3 != nil {
			_ = out.Close()
			return "", e3
		}
		_ = out.Close()
	}

	return outPath, nil
}
