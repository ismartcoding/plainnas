package api

import (
	"context"
	"embed"
	"io/fs"
	"ismartcoding/plainnas/internal/config"
	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/pkg/log"
	"ismartcoding/plainnas/internal/pkg/tls"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

//go:embed dist
var webFS embed.FS

func fileFromFS(fsys http.FileSystem, name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		f, err := fsys.Open(name)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		http.ServeContent(c.Writer, c.Request, name, stat.ModTime(), f)
	}
}

// Run run api server
func Run(ctx context.Context) {
	gin.SetMode(gin.ReleaseMode)

	if _, err := os.Stat(consts.ETC_TLS_SERVER_PEM); err != nil {
		tls.MakeCert(consts.ETC_TLS_SERVER_PEM, consts.ETC_TLS_SERVER_KEY)
	}

	r := gin.Default()
	r.Use(gzipMiddleware())
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, c-id")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})
	r.GET("/health_check", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/auth", authHandler())
	r.POST("/auth/status", authStatusHandler())
	r.POST("/auth/setup", authSetupHandler())
	r.POST("/graphql", requireAuth(), graphqlHandler())
	r.POST("/upload", uploadHandler())
	r.POST("/upload_chunk", uploadChunkHandler())
	r.GET("/ws", wsHandler())
	r.GET("/fs", fsHandler())
	// Serve embedded frontend assets from web/dist
	distFS, err := fs.Sub(webFS, "dist")
	if err != nil {
		log.Errorf("failed to load embedded web/dist: %v", err)
	} else {
		assetsFS, _ := fs.Sub(distFS, "assets")
		ficonsFS, _ := fs.Sub(distFS, "ficons")
		iconsFS, _ := fs.Sub(distFS, "icons")

		r.StaticFS("/assets", http.FS(assetsFS))
		r.StaticFS("/ficons", http.FS(ficonsFS))
		r.StaticFS("/icons", http.FS(iconsFS))

		// Single-file endpoints
		distHTTP := http.FS(distFS)
		r.GET("/broken-image.png", fileFromFS(distHTTP, "broken-image.png"))
		r.GET("/favicon.ico", fileFromFS(distHTTP, "favicon.ico"))
		r.GET("/logo.svg", fileFromFS(distHTTP, "logo.svg"))
		r.GET("/manifest.json", fileFromFS(distHTTP, "manifest.json"))
		r.GET("/sw.js", fileFromFS(distHTTP, "sw.js"))

		// SPA fallback
		r.NoRoute(func(c *gin.Context) {
			f, err := distHTTP.Open("index.html")
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			defer f.Close()
			stat, err := f.Stat()
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), f)
		})
	}

	// Log resolved ports to aid debugging visibility
	httpsPort := config.GetDefault().GetString("server.https_port")
	if httpsPort != "" {
		go func() {
			log.Infof("Starting HTTPS server at https://0.0.0.0:%s", httpsPort)
			if err := r.RunTLS(":"+httpsPort, consts.ETC_TLS_SERVER_PEM, consts.ETC_TLS_SERVER_KEY); err != nil {
				log.Panic(err)
			}
		}()
	}

	httpPort := config.GetDefault().GetString("server.http_port")
	if httpPort != "" {
		go func() {
			log.Infof("Starting HTTP server at http://0.0.0.0:%s", httpPort)
			if err := r.Run(":" + httpPort); err != nil {
				log.Panic(err)
			}
		}()
	}

}
