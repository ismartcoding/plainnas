package api

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

type gzipConfig struct {
	Level   int
	MinSize int
	Exclude []string
}

func defaultGzipConfig() gzipConfig {
	return gzipConfig{
		Level:   gzip.BestSpeed,
		MinSize: 1024,
		Exclude: []string{"/ws", "/fs", "/upload", "/upload_chunk"},
	}
}

func gzipMiddleware() gin.HandlerFunc {
	return gzipMiddlewareWithConfig(defaultGzipConfig())
}

func gzipMiddlewareWithConfig(cfg gzipConfig) gin.HandlerFunc {
	if cfg.MinSize <= 0 {
		cfg.MinSize = 1024
	}
	if cfg.Level == 0 {
		cfg.Level = gzip.BestSpeed
	}

	return func(c *gin.Context) {
		// 1) Client doesn't accept gzip.
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		// 2) Don't gzip upgrades / HEAD.
		if c.Request.Method == http.MethodHead ||
			strings.Contains(strings.ToLower(c.GetHeader("Connection")), "upgrade") ||
			strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
			c.Next()
			return
		}

		// 3) Path exclusions.
		for _, p := range cfg.Exclude {
			if p != "" && strings.HasPrefix(c.Request.URL.Path, p) {
				c.Next()
				return
			}
		}

		w := &gzipResponseWriter{
			ResponseWriter: c.Writer,
			cfg:            cfg,
			status:         http.StatusOK,
		}
		c.Writer = w
		defer w.Close()

		c.Next()
	}
}

var gzipPools sync.Map // map[int]*sync.Pool

func getGzipWriter(level int) *gzip.Writer {
	p, _ := gzipPools.LoadOrStore(level, &sync.Pool{New: func() any {
		w, _ := gzip.NewWriterLevel(nil, level)
		return w
	}})
	return p.(*sync.Pool).Get().(*gzip.Writer)
}

func putGzipWriter(level int, w *gzip.Writer) {
	if w == nil {
		return
	}
	if p, ok := gzipPools.Load(level); ok {
		p.(*sync.Pool).Put(w)
	}
}

type gzipResponseWriter struct {
	gin.ResponseWriter
	cfg gzipConfig

	status      int
	statusSet   bool
	wroteHdr    bool
	size        int
	decided     bool
	disabled    bool
	enabled     bool
	streaming   bool
	pending     bytes.Buffer
	gz          *gzip.Writer
	gzPoolLevel int
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.status = code
	w.statusSet = true
}

func (w *gzipResponseWriter) WriteHeaderNow() {
	if w.wroteHdr {
		return
	}

	// If a handler forces headers now, we cannot reliably decide gzip based on body size.
	// Fall back to passthrough to keep semantics (especially status codes) correct.
	if !w.decided {
		w.decided = true
		w.disableAndPassthrough()
	}

	// Ensure we write a status code (default 200 if none was set).
	w.writeHeaderIfNeeded()
}

func (w *gzipResponseWriter) Written() bool {
	return w.wroteHdr || w.size > 0
}

func (w *gzipResponseWriter) Status() int {
	if w.statusSet {
		return w.status
	}
	return w.ResponseWriter.Status()
}

func (w *gzipResponseWriter) Size() int {
	return w.size
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if w.streaming || (w.decided && w.disabled) {
		w.flushPendingRaw()
		w.writeHeaderIfNeeded()
		n, err := w.ResponseWriter.Write(data)
		w.size += n
		return n, err
	}

	if w.decided && w.enabled {
		n, err := w.gz.Write(data)
		w.size += n
		return n, err
	}

	// Not decided yet: buffer until MinSize, then decide.
	_, _ = w.pending.Write(data)
	if w.pending.Len() < w.cfg.MinSize {
		return len(data), nil
	}

	w.decide()
	if w.enabled {
		n, err := w.gz.Write(w.pending.Bytes())
		w.pending.Reset()
		if err != nil {
			w.disableAndPassthrough()
			return len(data), nil
		}
		w.size += n
		return len(data), nil
	}

	// Disabled: flush buffered raw.
	w.flushPendingRaw()
	return len(data), nil
}

func (w *gzipResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *gzipResponseWriter) Flush() {
	// Flush implies streaming; don't gzip.
	if !w.decided {
		w.streaming = true
		w.decided = true
		w.disableAndPassthrough()
		w.flushPendingRaw()
		w.writeHeaderIfNeeded()
		w.ResponseWriter.Flush()
		return
	}

	if w.enabled && w.gz != nil {
		_ = w.gz.Flush()
	}
	w.writeHeaderIfNeeded()
	w.ResponseWriter.Flush()
}

func (w *gzipResponseWriter) Close() {
	if !w.decided {
		// Small response: keep uncompressed.
		w.decided = true
		w.disableAndPassthrough()
	}

	if w.enabled {
		_ = w.gz.Close()
		putGzipWriter(w.gzPoolLevel, w.gz)
		w.gz = nil
	}

	w.flushPendingRaw()
	// If the handler only set a status (no body), make sure it reaches the client.
	w.writeHeaderIfNeeded()
}

func (w *gzipResponseWriter) decide() {
	if w.decided {
		return
	}
	w.decided = true

	// If handler already decided an encoding, don't touch it.
	if w.Header().Get("Content-Encoding") != "" {
		w.disableAndPassthrough()
		return
	}

	// Avoid breaking SSE.
	if strings.HasPrefix(strings.ToLower(w.Header().Get("Content-Type")), "text/event-stream") {
		w.disableAndPassthrough()
		return
	}

	// Only compress well-known compressible content types.
	ct := w.Header().Get("Content-Type")
	if !compressible(ct) {
		w.disableAndPassthrough()
		return
	}

	// Enable gzip.
	h := w.Header()
	h.Set("Content-Encoding", "gzip")
	h.Set("Vary", "Accept-Encoding")
	h.Del("Content-Length")

	w.writeHeaderIfNeeded()
	gz := getGzipWriter(w.cfg.Level)
	gz.Reset(w.ResponseWriter)
	w.gz = gz
	w.gzPoolLevel = w.cfg.Level
	w.enabled = true
}

func (w *gzipResponseWriter) disableAndPassthrough() {
	w.disabled = true
	w.enabled = false
	if w.gz != nil {
		_ = w.gz.Close()
		putGzipWriter(w.gzPoolLevel, w.gz)
		w.gz = nil
	}
}

func (w *gzipResponseWriter) writeHeaderIfNeeded() {
	if w.wroteHdr {
		return
	}

	status := http.StatusOK
	if w.statusSet {
		status = w.status
	}

	w.ResponseWriter.WriteHeader(status)
	w.wroteHdr = true
}

func (w *gzipResponseWriter) flushPendingRaw() {
	if w.pending.Len() == 0 {
		return
	}
	w.writeHeaderIfNeeded()
	n, _ := w.ResponseWriter.Write(w.pending.Bytes())
	w.size += n
	w.pending.Reset()
}

func compressible(ct string) bool {
	ct = strings.ToLower(ct)
	// Strip charset, etc.
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return strings.HasPrefix(ct, "text/") ||
		strings.HasPrefix(ct, "application/json") ||
		strings.HasPrefix(ct, "application/javascript") ||
		strings.HasPrefix(ct, "application/xml") ||
		strings.HasPrefix(ct, "image/svg+xml")
}

var _ http.Flusher = (*gzipResponseWriter)(nil)
