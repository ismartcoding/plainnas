package media

import (
	"os"
	"path/filepath"
	"strings"
)

// maybeExtractCoverToTempImage attempts to extract an embedded cover image suitable
// for thumbnailing from common audio/video containers.
//
// Pure-Go only: we do not decode video frames. For video, this supports containers
// that can carry embedded artwork (e.g. MP4/MOV/M4A via `covr`).
//
// It returns a path to a temporary image file and a cleanup function.
// ok=false means "no cover extracted" and callers should fall back to treating the
// original path as an image.
func maybeExtractCoverToTempImage(path string) (imgPath string, cleanup func(), ok bool) {
	// Priority: sidecar > embedded.
	if p, ok := findSidecarCoverPath(path); ok {
		return p, nil, true
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	var coverBytes []byte
	var mime string

	switch ext {
	case "mp3":
		coverBytes, mime, ok = extractEmbeddedCoverMP3(path)
	case "flac":
		coverBytes, mime, ok = extractEmbeddedCoverFLAC(path)
	case "m4a", "mp4", "m4v", "mov", "3gp", "3gpp":
		coverBytes, mime, ok = extractEmbeddedCoverMP4(path)
	default:
		return "", nil, false
	}

	if !ok || len(coverBytes) == 0 {
		return "", nil, false
	}

	return writeTempCoverImage(coverBytes, mime)
}

// ThumbnailCacheRefPath returns the file path whose metadata should be used to
// invalidate thumbnail caches for `path`.
//
// When a sidecar cover exists, that sidecar is used; otherwise it returns `path`.
func ThumbnailCacheRefPath(path string) string {
	if p, ok := findSidecarCoverPath(path); ok {
		return p
	}
	return path
}

func findSidecarCoverPath(path string) (coverPath string, ok bool) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, filepath.Ext(base))

	// Common sidecar naming conventions.
	candidates := []string{
		stem + ".jpg",
		stem + ".jpeg",
		stem + ".webp",
		stem + ".png",
		stem + ".gif",
		"cover.jpg",
		"cover.jpeg",
		"cover.webp",
		"cover.png",
		"cover.gif",
		"folder.jpg",
		"folder.jpeg",
		"folder.webp",
		"folder.png",
		"folder.gif",
	}

	for _, name := range candidates {
		p := filepath.Join(dir, name)
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() || fi.Size() <= 0 {
			continue
		}
		return p, true
	}
	return "", false
}

func writeTempCoverImage(b []byte, mime string) (imgPath string, cleanup func(), ok bool) {
	// Guardrail: avoid writing unbounded blobs to disk.
	if len(b) == 0 || len(b) > 25*1024*1024 {
		return "", nil, false
	}

	tmpDir, err := os.MkdirTemp("", "plainnas-cover-*")
	if err != nil {
		return "", nil, false
	}
	cleanup = func() { _ = os.RemoveAll(tmpDir) }

	ext := ".img"
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	case "image/webp":
		ext = ".webp"
	case "image/png":
		ext = ".png"
	case "image/gif":
		ext = ".gif"
	}

	imgPath = filepath.Join(tmpDir, "cover"+ext)
	if err := os.WriteFile(imgPath, b, 0o600); err != nil {
		cleanup()
		return "", nil, false
	}
	fi, err := os.Stat(imgPath)
	if err != nil || fi.Size() <= 0 {
		cleanup()
		return "", nil, false
	}
	return imgPath, cleanup, true
}
