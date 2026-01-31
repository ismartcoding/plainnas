package api

import (
	"net/http"
	"os"
	"strings"

	"ismartcoding/plainnas/internal/dlna"

	"github.com/gin-gonic/gin"
)

// mediaHandler serves short, DLNA-friendly media URLs:
//
//	GET /media/<id>.<ext>
//
// These are generated internally for DLNA casting and intentionally avoid
// query parameters and special characters.
func mediaHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := strings.TrimSpace(c.Param("name"))
		if name == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		id := name
		if dot := strings.IndexByte(name, '.'); dot > 0 {
			id = name[:dot]
		}

		path, mime, ok := dlna.LookupMediaAlias(id)
		if !ok || path == "" {
			c.Status(http.StatusNotFound)
			return
		}

		fi, err := os.Stat(path)
		if err != nil || fi.IsDir() {
			c.Status(http.StatusNotFound)
			return
		}

		if mime != "" {
			c.Header("Content-Type", mime)
		}
		c.File(path)
	}
}
