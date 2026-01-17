package graph

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/helpers"
	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/media"
)

// listMediaBuckets groups media files by their parent directory (Android-style "bucket").
// Kept as a helper to keep generated resolver file small.
func listMediaBuckets(typeArg model.DataType) ([]*model.MediaBucket, error) {
	mediaType := ""
	switch typeArg {
	case model.DataTypeAudio:
		mediaType = "audio"
	case model.DataTypeVideo:
		mediaType = "video"
	case model.DataTypeImage:
		mediaType = "image"
	case model.DataTypeDefault:
		mediaType = ""
	}

	type bucketAgg struct {
		dir      string
		name     string
		count    int
		topItems []string
	}

	add := func(mf *media.MediaFile, buckets map[string]*bucketAgg) {
		if mf == nil {
			return
		}
		if mf.IsTrash {
			// Buckets are for the main media views; exclude trash.
			return
		}
		pathForBucket := mf.Path
		if mf.OriginalPath != "" {
			pathForBucket = mf.OriginalPath
		}
		dir, bucketID := helpers.BucketIDFromPath(pathForBucket)
		b := buckets[bucketID]
		if b == nil {
			b = &bucketAgg{dir: dir, name: helpers.BucketNameFromDir(dir)}
			buckets[bucketID] = b
		}
		b.count++
		if len(b.topItems) < 4 {
			b.topItems = append(b.topItems, filepath.ToSlash(pathForBucket))
		}
	}

	buckets := make(map[string]*bucketAgg, 256)

	if mediaType != "" {
		// Use moddesc index so topItems tend to be representative (recent).
		prefix := media.TypeIndexPrefix(mediaType, false, "moddesc")
		iterErr := db.GetDefault().Iterate(prefix, func(key []byte, _ []byte) error {
			uuid := media.UUIDFromTypeIndexKey(key)
			if uuid == "" {
				return nil
			}
			mf, err := media.GetFile(uuid)
			if err != nil || mf == nil {
				return nil
			}
			add(mf, buckets)
			return nil
		})
		if iterErr != nil {
			return nil, iterErr
		}
	} else {
		// DEFAULT: scan all media entries (non-trash) and bucketize.
		iterErr := db.GetDefault().Iterate([]byte("media:uuid:"), func(_ []byte, value []byte) error {
			var mf media.MediaFile
			if err := json.Unmarshal(value, &mf); err != nil {
				return nil
			}
			add(&mf, buckets)
			return nil
		})
		if iterErr != nil {
			return nil, iterErr
		}
	}

	out := make([]*model.MediaBucket, 0, len(buckets))
	for id, b := range buckets {
		out = append(out, &model.MediaBucket{
			ID:        id,
			Name:      b.name,
			ItemCount: b.count,
			TopItems:  b.topItems,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].ItemCount != out[j].ItemCount {
			return out[i].ItemCount > out[j].ItemCount
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})

	return out, nil
}
