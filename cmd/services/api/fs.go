package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/media"
	"ismartcoding/plainnas/internal/strutils"

	"github.com/gin-gonic/gin"
)

// fsHandler serves a file given an encrypted id.
// Supports thumbnail parameters: w (width), h (height), q (webp quality 1-100).
// Thumbnails are always generated and cached as WEBP to reduce space.
func fsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Query("id")
		if id == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		ciphertext, err := base64.StdEncoding.DecodeString(id)
		if err != nil {
			c.String(http.StatusForbidden, "File is expired or does not exist.")
			return
		}

		// Decrypt using global URL token
		path := ""
		if token := db.GetURLToken(); token != "" {
			key, _ := base64.StdEncoding.DecodeString(token)
			if plain := strutils.ChaCha20Decrypt(key, ciphertext); plain != nil {
				path = string(plain)
			}
		}

		if path == "" {
			c.String(http.StatusForbidden, "File is expired or does not exist.")
			return
		}

		fi, err := os.Stat(path)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		if fi.IsDir() {
			c.Status(http.StatusBadRequest)
			return
		}

		// Content-Disposition
		fileName := c.Query("name")
		if fileName == "" {
			fileName = filepath.Base(path)
		}
		encodedName := url.QueryEscape(fileName)
		encodedName = strings.ReplaceAll(encodedName, "+", "%20")
		disp := "inline"
		if c.Query("dl") == "1" {
			disp = "attachment"
		}
		c.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"; filename*=utf-8''%s", disp, fileName, encodedName))
		c.Header("Access-Control-Expose-Headers", "Content-Disposition")

		// Parse thumbnail params
		w := strutils.ParseInt(c.Query("w"))
		h := strutils.ParseInt(c.Query("h"))
		q := strutils.ParseIntDefault(c.Query("q"), 75)
		if q < 1 {
			q = 1
		} else if q > 100 {
			q = 100
		}
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

		cacheKey := "thumb:" + media.CacheKeyForThumbnail(path, refFi.ModTime().Unix(), refFi.Size(), tw, th, q, outFmt)
		failKey := "thumbfail:" + media.CacheKeyForThumbnail(path, refFi.ModTime().Unix(), refFi.Size(), tw, th, q, outFmt)

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
}

// mimeFromFormat moved to internal/media
