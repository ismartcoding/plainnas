package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ismartcoding/plainnas/internal/media/thumb"

	_ "golang.org/x/image/webp"
)

func webpSaveOptions(quality int) string {
	q := clamp(quality, 1, 100)
	// libvips webpsave: effort 0..6 (higher = slower, smaller).
	// Default to a fast setting to keep interactive thumbnail requests snappy.
	effort := 0
	if v := strings.TrimSpace(os.Getenv("PLAINNAS_WEBP_EFFORT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			effort = n
		}
	}
	effort = clamp(effort, 0, 6)
	return fmt.Sprintf("[Q=%d,effort=%d,strip]", q, effort)
}

func vipsCLIConcurrencyArgs() []string {
	// libvips thread pool size for the invoked process.
	// Default 1: generally best for latency; can be tuned via env.
	n := 1
	if v := strings.TrimSpace(os.Getenv("PLAINNAS_VIPS_CONCURRENCY")); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			n = i
		}
	}
	n = clamp(n, 1, 16)
	return []string{fmt.Sprintf("--vips-concurrency=%d", n)}
}

// MaxThumbBytes controls the maximum thumbnail size in bytes.
// Default 1MB; can be adjusted by callers at runtime.
var MaxThumbBytes int = 1024 * 1024

// ErrNoCover indicates the requested thumbnail source (audio/video) has no cover image.
// Callers may translate this to a 404 for "thumbnail not found".
var ErrNoCover = errors.New("media: no cover")

// ComputeTargetSize returns target width/height preserving aspect ratio.
func ComputeTargetSize(src image.Image, w, h int) (tw, th int) {
	bw := src.Bounds().Dx()
	bh := src.Bounds().Dy()
	return ComputeTargetSizeFromDims(bw, bh, w, h)
}

// ComputeTargetSizeFromDims returns target width/height preserving aspect ratio.
func ComputeTargetSizeFromDims(srcW, srcH, w, h int) (tw, th int) {
	tw, th = srcW, srcH
	if w > 0 && h > 0 {
		rw := float64(w) / float64(srcW)
		rh := float64(h) / float64(srcH)
		r := math.Min(rw, rh)
		if r < 1 {
			tw = int(float64(srcW) * r)
			th = int(float64(srcH) * r)
		}
	} else if w > 0 {
		if w < srcW {
			r := float64(w) / float64(srcW)
			tw = w
			th = int(float64(srcH) * r)
		}
	} else if h > 0 {
		if h < srcH {
			r := float64(h) / float64(srcH)
			th = h
			tw = int(float64(srcW) * r)
		}
	}
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}
	return
}

// GenerateThumbnail generates a thumbnail for path and always returns WEBP.
// For audio/video containers, thumbnails are sourced from:
//   - sidecar artwork (cover/folder/<stem>)
//   - embedded cover (MP3/FLAC/MP4)
//   - video frame extraction via ffmpeg (when no cover)
func GenerateThumbnail(path string, w, h, quality int, convertToJPEG bool) (data []byte, outFmt string, err error) {
	_ = convertToJPEG // legacy param; thumbnails are always WEBP.

	ext := strings.ToLower(filepath.Ext(path))
	isAudio := false
	isVideo := false
	switch ext {
	case ".mp3", ".wav", ".wma", ".ogg", ".m4a", ".opus", ".flac", ".aac":
		isAudio = true
	case ".mp4", ".mkv", ".webm", ".avi", ".3gp", ".mov", ".m4v", ".3gpp":
		isVideo = true
	}

	thumbPath := path
	if coverPath, cleanup, ok := maybeExtractCoverToTempImage(path); ok {
		if cleanup != nil {
			defer cleanup()
		}
		thumbPath = coverPath
	} else {
		if isAudio {
			return nil, "", ErrNoCover
		}
		if isVideo {
			b, err := generateVideoThumbnailWEBP(path, w, h, quality)
			if err != nil {
				return nil, "", err
			}
			if len(b) == 0 {
				return nil, "", ErrNoCover
			}
			return b, "webp", nil
		}
	}

	// GIF policy:
	// - If the (effective) thumbnail source is a small GIF (size < 5MB and max dimension < 600px),
	//   return the original GIF bytes as preview.
	// - Otherwise, fall through to WEBP thumbnail generation (which effectively uses the first frame).
	if shouldUseOriginalGIFForPreview(thumbPath) {
		b, err := os.ReadFile(thumbPath)
		if err != nil || len(b) == 0 {
			return nil, "", err
		}
		return b, "gif", nil
	}

	// Fast-path: if the source is already WEBP, no resize requested, and it's within cap, return as-is.
	if w <= 0 && h <= 0 && strings.EqualFold(filepath.Ext(thumbPath), ".webp") {
		if b, ok := returnRawWEBPIfCapped(thumbPath); ok {
			return b, "webp", nil
		}
	}

	if d, ok := generateImageThumbnailWEBPWithVipsSem(thumbPath, w, h, quality); ok {
		return d, "webp", nil
	}
	if d, ok := generateImageThumbnailWEBPWithVipsCLI(thumbPath, w, h, quality); ok {
		return d, "webp", nil
	}

	// WEBP-only policy: without vips tools we cannot fulfill thumbnail generation.
	return nil, "", fmt.Errorf("media: thumbnail generation requires vips tools for webp")
}

const (
	gifPreviewMaxBytes = 5 * 1024 * 1024
	gifPreviewMaxDim   = 600
)

// ThumbnailOutputFormat returns the expected output format used by GenerateThumbnail for `path`.
// This is used by the API layer to build stable cache keys and Content-Type without generating.
func ThumbnailOutputFormat(path string) string {
	refPath := ThumbnailCacheRefPath(path)
	if shouldUseOriginalGIFForPreview(refPath) {
		return "gif"
	}
	return "webp"
}

func shouldUseOriginalGIFForPreview(path string) bool {
	if !strings.EqualFold(filepath.Ext(path), ".gif") {
		return false
	}
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return false
	}
	if fi.Size() <= 0 || fi.Size() >= gifPreviewMaxBytes {
		return false
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return false
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return false
	}
	maxDim := cfg.Width
	if cfg.Height > maxDim {
		maxDim = cfg.Height
	}
	return maxDim < gifPreviewMaxDim
}

func returnRawWEBPIfCapped(path string) (data []byte, ok bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if fi.Size() > int64(MaxThumbBytes) {
		return nil, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return b, true
}

func generateImageThumbnailWEBPWithVipsCLI(path string, w, h, quality int) (data []byte, ok bool) {
	if envTruthy("PLAINNAS_DISABLE_VIPS") {
		return nil, false
	}

	if _, err := exec.LookPath("vips"); err != nil {
		// `libvips-tools` installs `vips` (and `vipsthumbnail`).
		return nil, false
	}

	tmpDir, err := os.MkdirTemp("", "plainnas-thumb-*")
	if err != nil {
		return nil, false
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	baseOutPath := filepath.Join(tmpDir, "thumb.webp")

	// Retry with lower quality to satisfy MaxThumbBytes.
	//
	// This path is used when we need to enforce a strict byte cap (default 200KB).
	// A naive step-down loop can end up spawning many `vips` processes for large targets
	// (e.g. 1024x1024), which can dominate request latency. Instead, adaptively drop
	// quality based on the previous output size to converge in fewer tries.
	q := clamp(quality, 1, 100)
	minQ := 10
	maxIters := 6
	for iter := 0; iter < maxIters; iter++ {
		outPath := baseOutPath
		outArg := outPath + webpSaveOptions(q)

		var args []string
		if w > 0 || h > 0 {
			args = []string{"thumbnail", path, outArg}
			// vips requires a width argument; use whichever is provided.
			width := w
			if width <= 0 {
				width = h
			}
			args = append(args, strconv.Itoa(width))
			if h > 0 {
				args = append(args, "--height", strconv.Itoa(h))
			}
		} else {
			// Conversion-only (no resize): use vips copy to transcode to WEBP.
			args = []string{"copy", path, outArg}
		}

		cmdArgs := append(vipsCLIConcurrencyArgs(), args...)
		cmd := exec.Command("vips", cmdArgs...)
		if outBytes, err := cmd.CombinedOutput(); err != nil {
			_ = outBytes
			return nil, false
		}

		fi, err := os.Stat(outPath)
		if err != nil {
			return nil, false
		}
		if fi.Size() <= int64(MaxThumbBytes) || q <= minQ {
			break
		}

		// Heuristic: shrink quality proportional to the overshoot.
		// For example, if output is 600KB and cap is 1MB, ratio=0.33 so q drops fast.
		ratio := float64(MaxThumbBytes) / float64(fi.Size())
		newQ := int(float64(q) * ratio * 0.92) // slight bias downward to avoid oscillation
		if newQ >= q {
			newQ = q - 10
		}
		if newQ < minQ {
			newQ = minQ
		}
		q = newQ
	}

	f, err := os.Open(baseOutPath)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, f); err != nil {
		return nil, false
	}

	return buf.Bytes(), true
}

// vipsthumbnail worker: batch jobs to amortize process startup costs.
var vipsWorker = thumb.NewVipsWorker(0)

func generateImageThumbnailWEBPWithVipsSem(path string, w, h, quality int) (data []byte, ok bool) {
	if envTruthy("PLAINNAS_DISABLE_VIPS") {
		return nil, false
	}
	if _, err := exec.LookPath("vipsthumbnail"); err != nil {
		return nil, false
	}
	if w <= 0 && h <= 0 {
		return nil, false
	}
	size := w
	if size <= 0 {
		size = h
	}
	q := clamp(quality, 1, 100)
	minQ := 10
	maxIters := 4

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for iter := 0; iter < maxIters; iter++ {
		b, err := vipsWorker.ResizeWEBPBytes(ctx, path, size, q)
		if err != nil {
			return nil, false
		}
		if len(b) <= MaxThumbBytes || q <= minQ {
			if len(b) > MaxThumbBytes {
				return nil, false
			}
			return b, true
		}

		// Adaptively drop quality based on overshoot (same idea as CLI fallback).
		ratio := float64(MaxThumbBytes) / float64(len(b))
		newQ := int(float64(q) * ratio * 0.92)
		if newQ >= q {
			newQ = q - 10
		}
		if newQ < minQ {
			newQ = minQ
		}
		q = newQ
	}

	return nil, false
}

func envTruthy(key string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func MimeFromFormat(format string) string {
	switch strings.ToLower(format) {
	case "jpeg", "jpg", "jfif":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// CacheKeyForThumbnail creates a cache key for a thumbnail using file metadata and params.
func CacheKeyForThumbnail(path string, modUnix int64, size int64, tw, th, q int, outFmt string) string {
	base := filepath.Clean(path)
	return base + "|" + outFmt + "|" + strconv.Itoa(tw) + "x" + strconv.Itoa(th) + "|q" + strconv.Itoa(q) + "|m" + strconv.FormatInt(modUnix, 10) + "|s" + strconv.FormatInt(size, 10)
}
