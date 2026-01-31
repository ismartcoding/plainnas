package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"ismartcoding/plainnas/internal/db"
	plainfs "ismartcoding/plainnas/internal/fs"
	"ismartcoding/plainnas/internal/graph/helpers"
	"ismartcoding/plainnas/internal/graph/model"

	"github.com/gin-gonic/gin"
)

type zipFilesRequest struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Query string `json:"query"`
	Name  string `json:"name"`
}

type zipPathItem struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

func zipDirHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Query("id"))
		if id == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		folderPath, err := plainfs.PathFromFileID(id)
		if err != nil {
			c.Status(http.StatusForbidden)
			return
		}
		folderPath = strings.TrimSpace(folderPath)
		if folderPath == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		fi, err := os.Stat(folderPath)
		if err != nil || !fi.IsDir() {
			c.Status(http.StatusNotFound)
			return
		}

		baseName := fi.Name()
		fileName := strings.TrimSpace(c.Query("name"))
		if fileName == "" {
			fileName = baseName + ".zip"
		}
		if !strings.HasSuffix(strings.ToLower(fileName), ".zip") {
			fileName += ".zip"
		}
		setZipDownloadHeaders(c, fileName)

		zipw := zip.NewWriter(c.Writer)
		defer zipw.Close()

		_ = zipFolderToWriter(zipw, folderPath, baseName)
	}
}

func zipFilesHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Query("id"))
		if id == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		plain, err := plainfs.PathFromFileID(id)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		var req zipFilesRequest
		if err := json.Unmarshal([]byte(plain), &req); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		typeStr := strings.ToUpper(strings.TrimSpace(req.Type))
		if typeStr == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		fileName := strings.TrimSpace(req.Name)
		if fileName == "" {
			fileName = "download.zip"
		}
		if !strings.HasSuffix(strings.ToLower(fileName), ".zip") {
			fileName += ".zip"
		}
		setZipDownloadHeaders(c, fileName)

		zipw := zip.NewWriter(c.Writer)
		defer zipw.Close()

		var items []zipPathItem
		switch typeStr {
		case "AUDIO":
			list, _ := helpers.ScanAudios(0, 10000, req.Query, model.FileSortByDateDesc)
			items = make([]zipPathItem, 0, len(list))
			for _, it := range list {
				if it == nil {
					continue
				}
				items = append(items, zipPathItem{Path: filepath.FromSlash(it.Path)})
			}
		case "VIDEO":
			list, _ := helpers.ScanVideos(0, 10000, req.Query, model.FileSortByDateDesc)
			items = make([]zipPathItem, 0, len(list))
			for _, it := range list {
				if it == nil {
					continue
				}
				items = append(items, zipPathItem{Path: filepath.FromSlash(it.Path)})
			}
		case "IMAGE":
			list, _ := helpers.ScanImages(0, 10000, req.Query, model.FileSortByDateDesc)
			items = make([]zipPathItem, 0, len(list))
			for _, it := range list {
				if it == nil {
					continue
				}
				items = append(items, zipPathItem{Path: filepath.FromSlash(it.Path)})
			}
		case "FILE":
			tmpKey := strings.TrimSpace(req.ID)
			if tmpKey == "" {
				c.Status(http.StatusBadRequest)
				return
			}
			b, _ := db.GetDefault().Get([]byte("temp:" + tmpKey))
			if b == nil {
				c.Status(http.StatusNotFound)
				return
			}
			_ = db.GetDefault().Delete([]byte("temp:" + tmpKey))
			if err := json.Unmarshal(b, &items); err != nil {
				c.Status(http.StatusBadRequest)
				return
			}
		default:
			c.Status(http.StatusBadRequest)
			return
		}

		// Filter to existing paths and normalize.
		items = filterExistingZipItems(items)
		items = dropItemsInsideSelectedDirs(items)

		for _, it := range items {
			p := strings.TrimSpace(it.Path)
			if p == "" {
				continue
			}

			fi, err := os.Stat(p)
			if err != nil {
				continue
			}

			entryName := strings.TrimSpace(it.Name)
			if entryName == "" {
				entryName = filepath.Base(p)
			}
			entryName = safeZipEntryName(entryName)
			if entryName == "" {
				entryName = filepath.Base(p)
				entryName = safeZipEntryName(entryName)
			}
			if entryName == "" {
				continue
			}

			if fi.IsDir() {
				_ = zipFolderToWriter(zipw, p, entryName)
				continue
			}
			_ = zipAddFile(zipw, p, entryName, fi)
		}
	}
}

func setZipDownloadHeaders(c *gin.Context, fileName string) {
	encodedName := url.QueryEscape(fileName)
	encodedName = strings.ReplaceAll(encodedName, "+", "%20")

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=utf-8''%s", fileName, encodedName))
	c.Header("Access-Control-Expose-Headers", "Content-Disposition")
	c.Header("Content-Type", "application/zip")
}

func safeZipEntryName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Clean(name)
	name = strings.TrimPrefix(name, "/")
	if name == "." || name == ".." || strings.HasPrefix(name, "../") {
		return ""
	}
	return name
}

func zipFolderToWriter(zipw *zip.Writer, folderPath string, prefix string) error {
	folderPath = strings.TrimSpace(folderPath)
	if folderPath == "" {
		return nil
	}
	prefix = safeZipEntryName(prefix)
	if prefix == "" {
		prefix = safeZipEntryName(filepath.Base(folderPath))
	}
	if prefix == "" {
		return nil
	}

	// Ensure top-level directory entry exists.
	_ = zipAddDir(zipw, prefix+"/")

	return filepath.WalkDir(folderPath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p == folderPath {
			return nil
		}

		rel, err := filepath.Rel(folderPath, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		rel = safeZipEntryName(rel)
		if rel == "" {
			return nil
		}

		zipName := prefix + "/" + rel
		if d.IsDir() {
			return zipAddDir(zipw, zipName+"/")
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		return zipAddFile(zipw, p, zipName, info)
	})
}

func zipAddDir(zipw *zip.Writer, zipName string) error {
	zipName = safeZipEntryName(zipName)
	if zipName == "" {
		return nil
	}
	if !strings.HasSuffix(zipName, "/") {
		zipName += "/"
	}

	h := &zip.FileHeader{Name: zipName, Method: zip.Store}
	_, err := zipw.CreateHeader(h)
	return err
}

func zipAddFile(zipw *zip.Writer, filePath string, zipName string, info os.FileInfo) error {
	zipName = safeZipEntryName(zipName)
	if zipName == "" {
		return nil
	}

	h, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	h.Name = zipName
	// Deflate is fine for most content; ZIP already stores meta for streaming.
	h.Method = zip.Deflate

	w, err := zipw.CreateHeader(h)
	if err != nil {
		return err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(w, f)
	return err
}

func filterExistingZipItems(items []zipPathItem) []zipPathItem {
	out := make([]zipPathItem, 0, len(items))
	seen := map[string]struct{}{}
	for _, it := range items {
		p := strings.TrimSpace(it.Path)
		if p == "" {
			continue
		}
		p = filepath.Clean(p)
		if _, ok := seen[p]; ok {
			continue
		}
		if fi, err := os.Stat(p); err == nil && fi != nil {
			seen[p] = struct{}{}
			out = append(out, zipPathItem{Path: p, Name: strings.TrimSpace(it.Name)})
		}
	}
	return out
}

func dropItemsInsideSelectedDirs(items []zipPathItem) []zipPathItem {
	dirs := make([]string, 0)
	for _, it := range items {
		p := strings.TrimSpace(it.Path)
		if p == "" {
			continue
		}
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			dirs = append(dirs, filepath.Clean(p))
		}
	}
	if len(dirs) == 0 {
		return items
	}

	sort.Slice(dirs, func(i, j int) bool {
		// Shorter first.
		if len(dirs[i]) != len(dirs[j]) {
			return len(dirs[i]) < len(dirs[j])
		}
		return dirs[i] < dirs[j]
	})

	isInsideAnyDir := func(p string) bool {
		p = filepath.Clean(p)
		for _, d := range dirs {
			if p == d {
				continue
			}
			if strings.HasPrefix(p, d+string(os.PathSeparator)) {
				return true
			}
		}
		return false
	}

	out := make([]zipPathItem, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.Path) == "" {
			continue
		}
		if isInsideAnyDir(it.Path) {
			continue
		}
		out = append(out, it)
	}
	return out
}
