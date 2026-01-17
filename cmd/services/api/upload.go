package api

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/media"
	"ismartcoding/plainnas/internal/pkg/log"
	"ismartcoding/plainnas/internal/strutils"

	"github.com/gin-gonic/gin"
)

// uploadHandler handles direct form-data uploads with an encrypted "info" part
// and a "file" part, similar to PlainApp behavior.
func uploadHandler() gin.HandlerFunc {
	type uploadInfo struct {
		Dir     string `json:"dir"`
		Replace bool   `json:"replace"`
	}

	// generate unique filename if file exists and replace is false
	makeUniquePath := func(path string) string {
		if _, err := os.Stat(path); err != nil {
			return path
		}
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		name := base
		ext := ""
		if i := strings.LastIndex(base, "."); i > 0 {
			name = base[:i]
			ext = base[i:]
		}
		for i := 1; ; i++ {
			candidate := filepath.Join(dir, name+" ("+strconv.Itoa(i)+")"+ext)
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				return candidate
			}
		}
	}

	return func(c *gin.Context) {
		clientID := c.GetHeader("c-id")
		if clientID == "" {
			c.String(http.StatusBadRequest, "c-id header is missing")
			return
		}
		session := db.GetSession(clientID)
		if session == nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		key, _ := base64.StdEncoding.DecodeString(session.Token)

		log.Debugf("[/upload] start clientID=%s ct=%s ua=%s", clientID, c.Request.Header.Get("Content-Type"), c.Request.UserAgent())

		mr, err := c.Request.MultipartReader()
		if err != nil {
			log.Errorf("[/upload] invalid multipart form: %v", err)
			c.String(http.StatusBadRequest, "invalid multipart form")
			return
		}

		var info uploadInfo
		haveInfo := false
		savedFileName := ""
		lastDestPath := ""

		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Errorf("[/upload] next part error: %v", err)
				c.String(http.StatusBadRequest, "read multipart error")
				return
			}

			name := part.FormName()
			switch name {
			case "info":
				b, _ := io.ReadAll(part)
				dec := strutils.ChaCha20Decrypt(key, b)
				if dec == nil {
					log.Errorf("[/upload] decrypt info failed")
					c.Status(http.StatusUnauthorized)
					part.Close()
					return
				}
				if err := json.Unmarshal(dec, &info); err != nil {
					log.Errorf("[/upload] bad info json: %v", err)
					c.String(http.StatusBadRequest, "bad info json")
					part.Close()
					return
				}
				log.Debugf("[/upload] info dir=%q replace=%v", info.Dir, info.Replace)
				haveInfo = true
			case "file":
				if !haveInfo {
					log.Errorf("[/upload] file part before info")
					c.String(http.StatusBadRequest, "info part missing before file")
					part.Close()
					return
				}
				fileName := part.FileName()
				if info.Dir == "" || fileName == "" {
					log.Errorf("[/upload] dir or filename missing dir=%q filename=%q", info.Dir, fileName)
					c.String(http.StatusBadRequest, "dir or filename missing")
					part.Close()
					return
				}
				destPath := filepath.Clean(filepath.Join(info.Dir, fileName))
				log.Debugf("[/upload] incoming file name=%q dest=%q", fileName, destPath)
				if fi, err := os.Stat(destPath); err == nil && !fi.IsDir() {
					if info.Replace {
						log.Debugf("[/upload] replacing existing file: %s", destPath)
						_ = os.Remove(destPath)
					} else {
						destPath = makeUniquePath(destPath)
						fileName = filepath.Base(destPath)
						log.Debugf("[/upload] target exists, using unique path: %s", destPath)
					}
				}
				if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
					log.Errorf("[/upload] cannot create dir %q: %v", filepath.Dir(destPath), err)
					c.String(http.StatusBadRequest, "cannot create dir")
					part.Close()
					return
				}
				f, err := os.Create(destPath)
				if err != nil {
					log.Errorf("[/upload] cannot create file %q: %v", destPath, err)
					c.String(http.StatusBadRequest, "cannot create file")
					part.Close()
					return
				}
				if _, err := io.Copy(f, part); err != nil {
					f.Close()
					log.Errorf("[/upload] write file error for %q: %v", destPath, err)
					c.String(http.StatusBadRequest, "write file error")
					part.Close()
					return
				}
				f.Close()
				savedFileName = fileName
				lastDestPath = destPath
			default:
				// ignore unknown parts
			}
			part.Close()
		}

		if savedFileName == "" {
			log.Errorf("[/upload] no file uploaded")
			c.String(http.StatusBadRequest, "no file uploaded")
			return
		}

		log.Infof("[/upload] saved file=%q path=%q", savedFileName, lastDestPath)
		// Index the uploaded file immediately
		if err := media.ScanFile(lastDestPath); err != nil {
			log.Errorf("[/upload] index file error for %q: %v", lastDestPath, err)
		}
		c.String(http.StatusCreated, savedFileName)
	}
}

// uploadChunkHandler handles chunk uploads: multipart with encrypted "info" and raw chunk "file"
func uploadChunkHandler() gin.HandlerFunc {
	type uploadChunkInfo struct {
		FileID string `json:"fileId"`
		Index  int    `json:"index"`
	}

	return func(c *gin.Context) {
		clientID := c.GetHeader("c-id")
		if clientID == "" {
			c.String(http.StatusBadRequest, "c-id header is missing")
			return
		}
		session := db.GetSession(clientID)
		if session == nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		key, _ := base64.StdEncoding.DecodeString(session.Token)

		log.Debugf("[/upload_chunk] start clientID=%s ct=%s ua=%s", clientID, c.Request.Header.Get("Content-Type"), c.Request.UserAgent())

		mr, err := c.Request.MultipartReader()
		if err != nil {
			log.Errorf("[/upload_chunk] invalid multipart form: %v", err)
			c.String(http.StatusBadRequest, "invalid multipart form")
			return
		}

		var info uploadChunkInfo
		haveInfo := false
		chunkSaved := false
		chunkPathSaved := ""

		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Errorf("[/upload_chunk] next part error: %v", err)
				c.String(http.StatusBadRequest, "read multipart error")
				return
			}
			switch part.FormName() {
			case "info":
				b, _ := io.ReadAll(part)
				dec := strutils.ChaCha20Decrypt(key, b)
				if dec == nil {
					log.Errorf("[/upload_chunk] decrypt info failed")
					c.Status(http.StatusUnauthorized)
					part.Close()
					return
				}
				if err := json.Unmarshal(dec, &info); err != nil {
					log.Errorf("[/upload_chunk] bad info json: %v", err)
					c.String(http.StatusBadRequest, "bad info json")
					part.Close()
					return
				}
				log.Debugf("[/upload_chunk] info fileId=%q index=%d", info.FileID, info.Index)
				haveInfo = true
			case "file":
				if !haveInfo || info.FileID == "" || info.Index < 0 {
					log.Errorf("[/upload_chunk] missing fileId or invalid index fileId=%q index=%d", info.FileID, info.Index)
					c.String(http.StatusBadRequest, "fileId or index is missing or invalid")
					part.Close()
					return
				}
				// Create chunk dir under data dir
				base := consts.DATA_DIR
				chunkDir := filepath.Join(base, "upload_tmp", info.FileID)
				if err := os.MkdirAll(chunkDir, 0o755); err != nil {
					log.Errorf("[/upload_chunk] cannot create chunk dir %q: %v", chunkDir, err)
					c.String(http.StatusBadRequest, "cannot create chunk dir")
					part.Close()
					return
				}
				chunkPath := filepath.Join(chunkDir, "chunk_"+strconv.Itoa(info.Index))
				log.Debugf("[/upload_chunk] saving chunk to %q", chunkPath)
				f, err := os.Create(chunkPath)
				if err != nil {
					log.Errorf("[/upload_chunk] cannot create chunk file %q: %v", chunkPath, err)
					c.String(http.StatusBadRequest, "cannot create chunk file")
					part.Close()
					return
				}
				if _, err := io.Copy(f, part); err != nil {
					f.Close()
					log.Errorf("[/upload_chunk] write chunk error for %q: %v", chunkPath, err)
					c.String(http.StatusBadRequest, "write chunk error")
					part.Close()
					return
				}
				f.Close()
				chunkSaved = true
				chunkPathSaved = chunkPath
			default:
				// ignore
			}
			part.Close()
		}

		if chunkSaved {
			log.Infof("[/upload_chunk] saved %q", chunkPathSaved)
			// Optionally index chunks is not useful; indexing only final assembled file is recommended.
			c.String(http.StatusCreated, "chunk_"+strconv.Itoa(info.Index))
		} else {
			log.Errorf("[/upload_chunk] chunk upload failed")
			c.String(http.StatusBadRequest, "chunk upload failed")
		}
	}
}
