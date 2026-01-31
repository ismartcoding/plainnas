package api

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/fs"
	"ismartcoding/plainnas/internal/media"
	"ismartcoding/plainnas/internal/strutils"

	"github.com/gin-gonic/gin"
)

func resolveFSFile(c *gin.Context) (string, os.FileInfo, bool) {
	id := c.Query("id")
	if id == "" {
		c.Status(http.StatusBadRequest)
		return "", nil, false
	}

	path, err := fs.PathFromFileID(id)
	if err != nil {
		c.String(http.StatusForbidden, "File is expired or does not exist.")
		return "", nil, false
	}

	fi, err := os.Stat(path)
	if err != nil {
		c.Status(http.StatusNotFound)
		return "", nil, false
	}
	if fi.IsDir() {
		c.Status(http.StatusBadRequest)
		return "", nil, false
	}

	return path, fi, true
}

func normalizePreviewMode(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func resolveFSFileName(c *gin.Context, path string, preview string) string {
	fileName := c.Query("name")
	if fileName == "" {
		fileName = filepath.Base(path)
	}

	if preview == "pdf" {
		// Ensure a stable PDF name for browser viewers.
		if !strings.HasSuffix(strings.ToLower(fileName), ".pdf") {
			base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
			fileName = base + ".pdf"
		}
	}

	return fileName
}

func setContentDispositionHeaders(c *gin.Context, fileName string) {
	encodedName := url.QueryEscape(fileName)
	encodedName = strings.ReplaceAll(encodedName, "+", "%20")

	disp := "inline"
	if c.Query("dl") == "1" {
		disp = "attachment"
	}
	c.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"; filename*=utf-8''%s", disp, fileName, encodedName))
	c.Header("Access-Control-Expose-Headers", "Content-Disposition")
}

func servePDFPreview(c *gin.Context, path string, fi os.FileInfo) {
	pdfPath, err := fs.GetOrCreatePDFPreview(path, fi.ModTime().Unix(), fi.Size())
	if err != nil {
		switch err {
		case fs.ErrPreviewNotSupported:
			c.Status(http.StatusBadRequest)
		case fs.ErrPreviewToolMissing:
			c.String(http.StatusNotImplemented, "LibreOffice is required for DOC/DOCX preview.")
		default:
			c.String(http.StatusInternalServerError, "Failed to generate preview.")
		}
		return
	}

	db.AddRecentFile(path)
	c.File(pdfPath)
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func serveOriginalOrThumbnail(c *gin.Context, path string, fi os.FileInfo) {
	// Parse thumbnail params
	w := strutils.ParseInt(c.Query("w"))
	h := strutils.ParseInt(c.Query("h"))
	q := clampInt(strutils.ParseIntDefault(c.Query("q"), 75), 1, 100)
	// Back-compat: historically cc=1 requested JPEG conversion.
	// Thumbnails are now always WEBP; keep cc only as a "request thumbnail" signal.
	cc := c.Query("cc") == "1"

	// If no thumbnail params provided, serve original file
	if w <= 0 && h <= 0 && !cc {
		db.AddRecentFile(path)
		c.File(path)
		return
	}

	if w > 200 {
		db.AddRecentFile(path)
	}

	// Thumbnails are WEBP by default, except small GIFs which are served as original GIF previews.
	outFmt := media.ThumbnailOutputFormat(path)
	// Compute target size for cache key only (helpers recompute internally)
	tw, th := w, h

	// Cache invalidation source: sidecar cover (if present) should invalidate thumbnails.
	refPath := media.ThumbnailCacheRefPath(path)
	refFi := fi
	if refPath != path {
		if sfi, err := os.Stat(refPath); err == nil && !sfi.IsDir() {
			refFi = sfi
		}
	}

	keyBase := media.CacheKeyForThumbnail(path, refFi.ModTime().Unix(), refFi.Size(), tw, th, q, outFmt)
	cacheKey := "thumb:" + keyBase
	failKey := "thumbfail:" + keyBase

	if failed, _ := db.GetDefault().Get([]byte(failKey)); failed != nil {
		c.Status(http.StatusNoContent)
		return
	}

	if cached, _ := db.GetDefault().Get([]byte(cacheKey)); cached != nil {
		ct := media.MimeFromFormat(outFmt)
		c.Data(http.StatusOK, ct, cached)
		return
	}

	data, fmtUsed, err := media.GenerateThumbnail(path, w, h, q, cc)
	if err == media.ErrNoCover {
		_ = db.GetDefault().Set([]byte(failKey), []byte{1}, nil)
		c.Status(http.StatusNoContent)
		return
	}
	if err != nil || len(data) == 0 {
		c.File(path)
		return
	}

	_ = db.GetDefault().Set([]byte(cacheKey), data, nil)
	ct := media.MimeFromFormat(fmtUsed)
	c.Data(http.StatusOK, ct, data)
}

// fsHandler serves a file given an encrypted id.
// Supports thumbnail parameters: w (width), h (height), q (webp quality 1-100).
// Thumbnails are always generated and cached as WEBP to reduce space.
func fsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		path, fi, ok := resolveFSFile(c)
		if !ok {
			return
		}

		preview := normalizePreviewMode(c.Query("preview"))
		fileName := resolveFSFileName(c, path, preview)
		setContentDispositionHeaders(c, fileName)

		if preview == "pdf" {
			servePDFPreview(c, path, fi)
			return
		}

		serveOriginalOrThumbnail(c, path, fi)
	}
}

// mimeFromFormat moved to internal/media
