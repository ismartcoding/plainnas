package graph

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/version"
)

const (
	plainnasRepo           = "ismartcoding/plainnas"
	githubLatestReleaseURL = "https://api.github.com/repos/" + plainnasRepo + "/releases/latest"
	appUpdateCacheTTL      = 10 * time.Minute
)

type appUpdateCache struct {
	mu        sync.Mutex
	value     *model.AppUpdate
	fetchedAt time.Time
}

var globalAppUpdateCache appUpdateCache

type githubLatestRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func buildAppUpdate(ctx context.Context) (*model.AppUpdate, error) {
	current := normalizeVersion(version.Version)
	if current == "" {
		current = strings.TrimSpace(version.Version)
	}

	globalAppUpdateCache.mu.Lock()
	if globalAppUpdateCache.value != nil && time.Since(globalAppUpdateCache.fetchedAt) < appUpdateCacheTTL {
		cached := globalAppUpdateCache.value
		globalAppUpdateCache.mu.Unlock()
		return cached, nil
	}
	globalAppUpdateCache.mu.Unlock()

	res := &model.AppUpdate{
		CurrentVersion: current,
		HasUpdate:      false,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubLatestReleaseURL, nil)
	if err != nil {
		cacheAppUpdate(res)
		return res, nil
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "plainnas")

	client := &http.Client{Timeout: 5 * time.Second}
	httpRes, err := client.Do(req)
	if err != nil {
		cacheAppUpdate(res)
		return res, nil
	}
	defer httpRes.Body.Close()

	if httpRes.StatusCode != http.StatusOK {
		cacheAppUpdate(res)
		return res, nil
	}

	var gh githubLatestRelease
	if err := json.NewDecoder(httpRes.Body).Decode(&gh); err != nil {
		cacheAppUpdate(res)
		return res, nil
	}

	latest := normalizeVersion(gh.TagName)
	if latest == "" {
		latest = strings.TrimSpace(gh.TagName)
	}

	res.LatestVersion = optionalString(latest)
	res.URL = optionalString(strings.TrimSpace(gh.HTMLURL))
	res.HasUpdate = hasNewerVersion(current, latest)

	cacheAppUpdate(res)
	return res, nil
}

func cacheAppUpdate(v *model.AppUpdate) {
	globalAppUpdateCache.mu.Lock()
	globalAppUpdateCache.value = v
	globalAppUpdateCache.fetchedAt = time.Now()
	globalAppUpdateCache.mu.Unlock()
}

func optionalString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func normalizeVersion(v string) string {
	s := strings.TrimSpace(v)
	s = strings.TrimPrefix(s, "PlainNAS")
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimSpace(s)
	// Strip build metadata / prerelease if present.
	if i := strings.IndexAny(s, "+-"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func hasNewerVersion(current, latest string) bool {
	cur, okCur := parseSemver(normalizeVersion(current))
	lat, okLat := parseSemver(normalizeVersion(latest))
	if okCur && okLat {
		if lat[0] != cur[0] {
			return lat[0] > cur[0]
		}
		if lat[1] != cur[1] {
			return lat[1] > cur[1]
		}
		return lat[2] > cur[2]
	}

	// Fallback: if both are non-empty and different, assume update.
	c := strings.TrimSpace(current)
	l := strings.TrimSpace(latest)
	if c == "" || l == "" {
		return false
	}
	return normalizeVersion(c) != normalizeVersion(l)
}

func parseSemver(v string) ([3]int, bool) {
	var out [3]int
	s := normalizeVersion(v)
	parts := strings.Split(s, ".")
	if len(parts) < 3 {
		return out, false
	}
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
