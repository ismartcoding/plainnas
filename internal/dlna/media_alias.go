package dlna

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type mediaAliasEntry struct {
	Path      string
	Mime      string
	ExpiresAt time.Time
}

var (
	mediaAliasMu   sync.RWMutex
	mediaAliasByID = map[string]mediaAliasEntry{}
	mediaAliasTTL  = 30 * time.Minute

	extRe = regexp.MustCompile(`^[a-z0-9]{1,16}$`)
)

func registerMediaAlias(path string, mime string) (id string, ext string) {
	n := time.Now().UnixNano()
	id = strconv.FormatInt(n, 36)
	if id == "" {
		id = strconv.FormatInt(time.Now().UnixNano(), 10)
	}

	ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if !extRe.MatchString(ext) {
		ext = "bin"
	}

	mediaAliasMu.Lock()
	mediaAliasByID[id] = mediaAliasEntry{
		Path:      path,
		Mime:      strings.TrimSpace(mime),
		ExpiresAt: time.Now().Add(mediaAliasTTL),
	}
	mediaAliasMu.Unlock()

	return id, ext
}

func lookupMediaAlias(id string) (path string, mime string, ok bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", "", false
	}

	mediaAliasMu.RLock()
	e, exists := mediaAliasByID[id]
	mediaAliasMu.RUnlock()
	if !exists {
		return "", "", false
	}
	if !e.ExpiresAt.IsZero() && time.Now().After(e.ExpiresAt) {
		mediaAliasMu.Lock()
		delete(mediaAliasByID, id)
		mediaAliasMu.Unlock()
		return "", "", false
	}
	return e.Path, e.Mime, true
}

// LookupMediaAlias resolves a short DLNA media id to a real file path.
func LookupMediaAlias(id string) (path string, mime string, ok bool) {
	return lookupMediaAlias(id)
}
