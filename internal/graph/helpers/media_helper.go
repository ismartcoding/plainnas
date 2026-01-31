package helpers

import (
	"encoding/base64"
	"encoding/json"
	"hash/fnv"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/media"
	"ismartcoding/plainnas/internal/search"
	"ismartcoding/plainnas/internal/strutils"
)

type mediaFileItem struct {
	id       string
	path     string
	size     int64
	mod      int64
	name     string
	bucketID string
	duration int
	artist   string
	title    string
}

type mediaQueryFilters struct {
	text       string
	basePath   string
	trashOnly  bool
	ids        string
	bucketID   string
	showHidden bool
}

func bucketDirFromPath(path string) string {
	p := filepath.ToSlash(filepath.Clean(path))
	d := filepath.Dir(p)
	if d == "." {
		return ""
	}
	return d
}

func bucketIDFromDir(dir string) string {
	dir = filepath.ToSlash(filepath.Clean(dir))
	h := fnv.New32a()
	_, _ = h.Write([]byte(dir))
	return strconv.FormatUint(uint64(h.Sum32()), 10)
}

func BucketIDFromPath(path string) (dir string, bucketID string) {
	dir = bucketDirFromPath(path)
	return dir, bucketIDFromDir(dir)
}

func BucketNameFromDir(dir string) string {
	base := filepath.Base(filepath.ToSlash(filepath.Clean(dir)))
	if base == "." {
		return ""
	}
	return base
}

func parseMediaQueryFilters(query string) mediaQueryFilters {
	filterFields := search.ParseWithTagMapping(query, func(tagIDs []string) []string {
		var allKeys []string
		for _, tagID := range tagIDs {
			if keys, err := TagHelperInstance.GetKeysByTagId(tagID); err == nil {
				allKeys = append(allKeys, keys...)
			}
		}
		return allKeys
	})

	f := mediaQueryFilters{}
	rootPath := ""
	relativePath := ""
	for _, it := range filterFields {
		switch it.Name {
		case "show_hidden":
			f.showHidden = it.Value == "true"
		case "text":
			f.text = it.Value
		case "bucket_id":
			f.bucketID = it.Value
		case "root_path":
			rootPath = it.Value
		case "relative_path":
			relativePath = it.Value
		case "trash":
			f.trashOnly = strings.ToLower(it.Value) == "true"
		case "ids":
			f.ids = it.Value
		}
	}

	base := rootPath
	if relativePath != "" {
		if after, ok := strings.CutPrefix(relativePath, "/"); ok {
			relativePath = after
		}
		base = filepath.Join(rootPath, relativePath)
	}
	f.basePath = base
	return f
}

// generateEncryptedFileID creates an encrypted file ID from a file path using the global URL token
func GenerateEncryptedFileID(path string) string {
	token := db.GetURLToken()
	if token == "" {
		return ""
	}
	key, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return ""
	}
	if enc := strutils.ChaCha20Encrypt(key, []byte(path)); enc != nil {
		return base64.StdEncoding.EncodeToString(enc)
	}
	return ""
}

func scanMedia(offset int, limit int, query string, sortBy model.FileSortBy, mediaType string) ([]mediaFileItem, error) {
	q := parseMediaQueryFilters(query)
	_ = q.showHidden
	text := q.text
	base := q.basePath
	trashOnly := q.trashOnly
	ids := q.ids
	bucketID := q.bucketID

	var results []media.MediaFile
	if ids != "" {
		// Use ids-based search when tag filtering is applied
		searchQuery := "ids:" + ids
		filters := map[string]string{"type": mediaType}
		if trashOnly {
			filters["trash"] = "true"
		} else {
			filters["trash"] = "false"
		}
		if base != "" {
			filters["path_prefix"] = filepath.ToSlash(filepath.Clean(base))
		}
		results, _ = media.Search(searchQuery, filters, 0, 10000)
	} else if text == "" {
		// Fast path: empty query, no ids. If we can satisfy ordering via media type indexes,
		// avoid scanning and unmarshalling the full Pebble media corpus.
		if limit > 0 && base == "" && mediaType != "" && (sortBy == model.FileSortByDateAsc || sortBy == model.FileSortByDateDesc || sortBy == model.FileSortByNameAsc || sortBy == model.FileSortByNameDesc || sortBy == model.FileSortBySizeAsc || sortBy == model.FileSortBySizeDesc) {
			idxKind := ""
			switch sortBy {
			case model.FileSortByDateAsc:
				idxKind = "mod"
			case model.FileSortByDateDesc:
				idxKind = "moddesc"
			case model.FileSortByNameAsc:
				idxKind = "name"
			case model.FileSortByNameDesc:
				idxKind = "namedesc"
			case model.FileSortBySizeAsc:
				idxKind = "size"
			case model.FileSortBySizeDesc:
				idxKind = "sizedesc"
			}
			prefix := media.TypeIndexPrefix(mediaType, trashOnly, idxKind)
			if len(prefix) > 0 {
				files := make([]mediaFileItem, 0, limit)
				skipped := 0
				iterErr := db.GetDefault().Iterate(prefix, func(key []byte, _ []byte) error {
					uuid := media.UUIDFromTypeIndexKey(key)
					if uuid == "" {
						return nil
					}
					mf, err := media.GetFile(uuid)
					if err != nil || mf == nil {
						return nil
					}
					// Best-effort: only probe duration for the items we actually return.
					if (mediaType == "audio" || mediaType == "video") && mf.DurationSec <= 0 {
						_, _ = media.EnsureDuration(mf)
					}
					// Best-effort: only probe artist for the items we actually return.
					if mediaType == "audio" && mf.Artist == "" {
						_, _ = media.EnsureArtist(mf)
					}
					// Best-effort: only probe title for the items we actually return.
					if mediaType == "audio" && mf.Title == "" {
						_, _ = media.EnsureTitle(mf)
					}
					// Enforce trash filter defensively (should be implied by index).
					if trashOnly && !mf.IsTrash {
						return nil
					}
					if !trashOnly && mf.IsTrash {
						return nil
					}
					// Bucket mapping uses original path if available (so trash items keep their origin bucket).
					pathForBucket := mf.Path
					if mf.OriginalPath != "" {
						pathForBucket = mf.OriginalPath
					}
					_, bID := BucketIDFromPath(pathForBucket)
					if bucketID != "" && bID != bucketID {
						return nil
					}
					if skipped < offset {
						skipped++
						return nil
					}
					files = append(files, mediaFileItem{id: mf.UUID, path: filepath.ToSlash(mf.Path), size: mf.Size, mod: mf.ModifiedAt, name: mf.Name, bucketID: bID, duration: mf.DurationSec, artist: mf.Artist, title: mf.Title})
					if len(files) >= limit {
						return db.ErrIterateStop
					}
					return nil
				})
				if iterErr == db.ErrIterateStop {
					iterErr = nil
				}
				return files, iterErr
			}
		}

		// Fallback: when no query text or ids, list all media via Pebble and apply filters
		var all []media.MediaFile
		_ = db.GetDefault().Iterate([]byte("media:uuid:"), func(key []byte, value []byte) error {
			var mf media.MediaFile
			if err := json.Unmarshal(value, &mf); err != nil {
				return nil
			}
			// Apply type filter
			if mediaType != "" && !strings.EqualFold(mf.Type, mediaType) {
				return nil
			}
			// Apply trash filter
			if trashOnly && !mf.IsTrash {
				return nil
			}
			if !trashOnly && mf.IsTrash {
				return nil
			}
			// Apply base path and source directory whitelist filters.
			if base != "" {
				if !strings.HasPrefix(filepath.ToSlash(mf.Path), filepath.ToSlash(base)) {
					return nil
				}
			}
			// Apply bucket filter
			if bucketID != "" {
				pathForBucket := mf.Path
				if mf.OriginalPath != "" {
					pathForBucket = mf.OriginalPath
				}
				_, bID := BucketIDFromPath(pathForBucket)
				if bID != bucketID {
					return nil
				}
			}
			all = append(all, mf)
			return nil
		})
		results = all
	} else {
		// Use regular text search
		filters := map[string]string{"type": mediaType}
		if trashOnly {
			filters["trash"] = "true"
		} else {
			filters["trash"] = "false"
		}
		if base != "" {
			filters["path_prefix"] = filepath.ToSlash(filepath.Clean(base))
		}
		results, _ = media.Search(text, filters, 0, 10000)
	}

	files := make([]mediaFileItem, 0, len(results))
	for _, it := range results {
		pathForBucket := it.Path
		if it.OriginalPath != "" {
			pathForBucket = it.OriginalPath
		}
		_, bID := BucketIDFromPath(pathForBucket)
		if bucketID != "" && bID != bucketID {
			continue
		}
		if base != "" {
			if !strings.HasPrefix(filepath.ToSlash(it.Path), filepath.ToSlash(base)) {
				continue
			}
		}
		files = append(files, mediaFileItem{id: it.UUID, path: filepath.ToSlash(it.Path), size: it.Size, mod: it.ModifiedAt, name: it.Name, bucketID: bID, duration: it.DurationSec, artist: it.Artist, title: it.Title})
	}

	switch sortBy {
	case model.FileSortByDateAsc:
		sort.Slice(files, func(i, j int) bool { return files[i].mod < files[j].mod })
	case model.FileSortByDateDesc:
		sort.Slice(files, func(i, j int) bool { return files[i].mod > files[j].mod })
	case model.FileSortBySizeAsc:
		sort.Slice(files, func(i, j int) bool { return files[i].size < files[j].size })
	case model.FileSortBySizeDesc:
		sort.Slice(files, func(i, j int) bool { return files[i].size > files[j].size })
	case model.FileSortByNameAsc:
		sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].name) < strings.ToLower(files[j].name) })
	case model.FileSortByNameDesc:
		sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].name) > strings.ToLower(files[j].name) })
	}

	if offset > 0 && offset < len(files) {
		files = files[offset:]
	} else if offset >= len(files) {
		files = nil
	}
	if limit > 0 && limit < len(files) {
		files = files[:limit]
	}
	// Best-effort probe for the final paginated items only.
	if mediaType == "audio" || mediaType == "video" {
		for i := range files {
			mf, err := media.GetFile(files[i].id)
			if err != nil || mf == nil {
				continue
			}
			if mf.DurationSec <= 0 {
				_, _ = media.EnsureDuration(mf)
			}
			files[i].duration = mf.DurationSec
			if mediaType == "audio" {
				if mf.Artist == "" {
					_, _ = media.EnsureArtist(mf)
				}
				files[i].artist = mf.Artist
				if mf.Title == "" {
					_, _ = media.EnsureTitle(mf)
				}
				files[i].title = mf.Title
			}
		}
	}
	return files, nil
}

func CountMedia(query string, mediaType string) (int, error) {
	q := parseMediaQueryFilters(query)
	text := q.text
	base := q.basePath
	trashOnly := q.trashOnly
	ids := q.ids
	bucketID := q.bucketID

	if ids != "" {
		searchQuery := "ids:" + ids
		filters := map[string]string{"type": mediaType}
		if trashOnly {
			filters["trash"] = "true"
		} else {
			filters["trash"] = "false"
		}
		if base != "" {
			filters["path_prefix"] = filepath.ToSlash(filepath.Clean(base))
		}
		results, err := media.Search(searchQuery, filters, 0, 1_000_000)
		if err != nil {
			return 0, err
		}
		if bucketID == "" {
			cnt := 0
			for _, it := range results {
				if base != "" {
					if !strings.HasPrefix(filepath.ToSlash(it.Path), filepath.ToSlash(base)) {
						continue
					}
				}
				cnt++
			}
			return cnt, nil
		}
		cnt := 0
		for _, it := range results {
			if base != "" {
				if !strings.HasPrefix(filepath.ToSlash(it.Path), filepath.ToSlash(base)) {
					continue
				}
			}
			pathForBucket := it.Path
			if it.OriginalPath != "" {
				pathForBucket = it.OriginalPath
			}
			_, bID := BucketIDFromPath(pathForBucket)
			if bID == bucketID {
				cnt++
			}
		}
		return cnt, nil
	}

	if text != "" {
		filters := map[string]string{"type": mediaType}
		if trashOnly {
			filters["trash"] = "true"
		} else {
			filters["trash"] = "false"
		}
		if base != "" {
			filters["path_prefix"] = filepath.ToSlash(filepath.Clean(base))
		}
		results, err := media.Search(text, filters, 0, 1_000_000)
		if err != nil {
			return 0, err
		}
		if bucketID == "" {
			cnt := 0
			for _, it := range results {
				if base != "" {
					if !strings.HasPrefix(filepath.ToSlash(it.Path), filepath.ToSlash(base)) {
						continue
					}
				}
				cnt++
			}
			return cnt, nil
		}
		cnt := 0
		for _, it := range results {
			if base != "" {
				if !strings.HasPrefix(filepath.ToSlash(it.Path), filepath.ToSlash(base)) {
					continue
				}
			}
			pathForBucket := it.Path
			if it.OriginalPath != "" {
				pathForBucket = it.OriginalPath
			}
			_, bID := BucketIDFromPath(pathForBucket)
			if bID == bucketID {
				cnt++
			}
		}
		return cnt, nil
	}

	// Empty-text count path.
	if base == "" && mediaType != "" {
		if bucketID != "" {
			prefix := media.TypeIndexPrefix(mediaType, trashOnly, "uuid")
			if len(prefix) > 0 {
				cnt := 0
				iterErr := db.GetDefault().Iterate(prefix, func(key []byte, _ []byte) error {
					uuid := media.UUIDFromTypeIndexKey(key)
					if uuid == "" {
						return nil
					}
					mf, err := media.GetFile(uuid)
					if err != nil || mf == nil {
						return nil
					}
					if trashOnly && !mf.IsTrash {
						return nil
					}
					if !trashOnly && mf.IsTrash {
						return nil
					}
					pathForBucket := mf.Path
					if mf.OriginalPath != "" {
						pathForBucket = mf.OriginalPath
					}
					_, bID := BucketIDFromPath(pathForBucket)
					if bID == bucketID {
						cnt++
					}
					return nil
				})
				return cnt, iterErr
			}
		}
		prefix := media.TypeIndexPrefix(mediaType, trashOnly, "uuid")
		if len(prefix) > 0 {
			cnt := 0
			if err := db.GetDefault().Iterate(prefix, func(key []byte, _ []byte) error {
				uuid := media.UUIDFromTypeIndexKey(key)
				if uuid == "" {
					return nil
				}
				mf, err := media.GetFile(uuid)
				if err != nil || mf == nil {
					return nil
				}
				if trashOnly && !mf.IsTrash {
					return nil
				}
				if !trashOnly && mf.IsTrash {
					return nil
				}
				cnt++
				return nil
			}); err != nil {
				return 0, err
			}
			return cnt, nil
		}
	}

	// Fallback: scan Pebble media records and apply filters.
	type mediaMeta struct {
		Type    string `json:"type"`
		IsTrash bool   `json:"trash"`
		Path    string `json:"path"`
		Orig    string `json:"original_path"`
	}
	baseSlash := filepath.ToSlash(base)
	if baseSlash != "" {
		baseSlash = filepath.ToSlash(baseSlash)
	}
	count := 0
	err := db.GetDefault().Iterate([]byte("media:uuid:"), func(_ []byte, value []byte) error {
		var mm mediaMeta
		if err := json.Unmarshal(value, &mm); err != nil {
			return nil
		}
		if mediaType != "" && !strings.EqualFold(mm.Type, mediaType) {
			return nil
		}
		if trashOnly && !mm.IsTrash {
			return nil
		}
		if !trashOnly && mm.IsTrash {
			return nil
		}
		if baseSlash != "" {
			if !strings.HasPrefix(filepath.ToSlash(mm.Path), baseSlash) {
				return nil
			}
		}
		if bucketID != "" {
			pathForBucket := mm.Path
			if mm.Orig != "" {
				pathForBucket = mm.Orig
			}
			_, bID := BucketIDFromPath(pathForBucket)
			if bID != bucketID {
				return nil
			}
		}
		count++
		return nil
	})
	return count, err
}

// scanImages walks directories according to query filters and returns Image models.
func ScanImages(offset int, limit int, query string, sortBy model.FileSortBy) ([]*model.Image, error) {
	files, _ := scanMedia(offset, limit, query, sortBy, "image")
	ids := make([]string, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.id)
	}
	tagsByKey, _ := TagHelperInstance.GetTagsByKeys(ids, model.DataTypeImage)
	out := make([]*model.Image, 0, len(files))
	for _, f := range files {
		tags := tagsByKey[f.id]
		if tags == nil {
			tags = []*model.Tag{}
		}

		out = append(out, &model.Image{
			ID:        f.id,
			Title:     filepath.Base(f.path),
			Path:      filepath.ToSlash(f.path),
			Size:      f.size,
			BucketID:  f.bucketID,
			CreatedAt: time.Unix(f.mod, 0),
			UpdatedAt: time.Unix(f.mod, 0),
			Tags:      tags,
		})
	}
	return out, nil
}

func ScanVideos(offset int, limit int, query string, sortBy model.FileSortBy) ([]*model.Video, error) {
	files, _ := scanMedia(offset, limit, query, sortBy, "video")
	ids := make([]string, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.id)
	}
	tagsByKey, _ := TagHelperInstance.GetTagsByKeys(ids, model.DataTypeVideo)
	out := make([]*model.Video, 0, len(files))
	for _, f := range files {
		tags := tagsByKey[f.id]
		if tags == nil {
			tags = []*model.Tag{}
		}

		out = append(out, &model.Video{
			ID:        f.id,
			Title:     filepath.Base(f.path),
			Path:      filepath.ToSlash(f.path),
			Duration:  f.duration,
			Size:      f.size,
			BucketID:  f.bucketID,
			CreatedAt: time.Unix(f.mod, 0),
			UpdatedAt: time.Unix(f.mod, 0),
			Tags:      tags,
		})
	}
	return out, nil
}

func ScanAudios(offset int, limit int, query string, sortBy model.FileSortBy) ([]*model.Audio, error) {
	files, _ := scanMedia(offset, limit, query, sortBy, "audio")
	ids := make([]string, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.id)
	}
	tagsByKey, _ := TagHelperInstance.GetTagsByKeys(ids, model.DataTypeAudio)
	out := make([]*model.Audio, 0, len(files))
	for _, f := range files {
		title := f.title
		if title == "" {
			base := filepath.Base(f.path)
			noExt := strings.TrimSuffix(base, filepath.Ext(base))
			if noExt != "" {
				title = noExt
			} else {
				title = base
			}
		}
		// Generate encrypted file ID for album art (use the audio file itself as fallback)
		albumFileID := GenerateEncryptedFileID(f.path)
		if albumFileID == "" {
			albumFileID = f.id // Fallback to media UUID if encryption fails
		}

		tags := tagsByKey[f.id]
		if tags == nil {
			tags = []*model.Tag{}
		}

		out = append(out, &model.Audio{
			ID:          f.id,
			Title:       title,
			Artist:      f.artist,
			Path:        filepath.ToSlash(f.path),
			Duration:    f.duration,
			Size:        f.size,
			BucketID:    f.bucketID,
			AlbumFileID: albumFileID,
			CreatedAt:   time.Unix(f.mod, 0),
			UpdatedAt:   time.Unix(f.mod, 0),
			Tags:        tags,
		})
	}
	return out, nil
}
