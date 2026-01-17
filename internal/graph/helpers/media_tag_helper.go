package helpers

import (
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/media"
	"ismartcoding/plainnas/internal/search"
	"path/filepath"
	"strings"
)

// TagRelationStub represents a media item that can be tagged
type TagRelationStub struct {
	Key   string `json:"key"`
	Size  int64  `json:"size"`
	Title string `json:"title"`
}

// ToTagRelation converts a TagRelationStub to an in-memory relation reference.
// The relation is persisted as key presence in Pebble, not as a JSON payload.
func (t TagRelationStub) ToTagRelation(tagID string) *db.TagRelationRef {
	return &db.TagRelationRef{TagID: tagID, Key: t.Key}
}

// MediaStoreHelper provides functions to get media IDs and stubs based on query
type MediaStoreHelper struct{}

// GetTagRelationStubs returns media items as TagRelationStubs based on query and media type
func (h MediaStoreHelper) GetTagRelationStubs(query string, mediaType string) ([]TagRelationStub, error) {
	filterFields := search.Parse(query)
	text := ""
	ids := ""

	for _, f := range filterFields {
		if f.Name == "text" {
			text = f.Value
		} else if f.Name == "ids" {
			ids = f.Value
		}
	}

	var items []media.MediaFile
	var err error

	if ids != "" {
		// Get specific items by IDs
		for _, id := range strings.Split(ids, ",") {
			if mf, getErr := media.GetFileByUUID(strings.TrimSpace(id)); getErr == nil && mf != nil {
				items = append(items, *mf)
			}
		}
	} else {
		// Search by text and filters
		filters := map[string]string{"type": mediaType}
		items, err = media.Search(text, filters, 0, 10000)
		if err != nil {
			return nil, err
		}
	}

	// Convert to TagRelationStubs
	stubs := make([]TagRelationStub, 0, len(items))
	for _, item := range items {
		stubs = append(stubs, TagRelationStub{
			Key:   item.UUID, // Use UUID as the key
			Size:  item.Size,
			Title: filepath.Base(item.Path),
		})
	}

	return stubs, nil
}

// GetIDs returns media item IDs based on query and media type
func (h MediaStoreHelper) GetIDs(query string, mediaType string) ([]string, error) {
	filterFields := search.Parse(query)
	text := ""
	ids := ""

	for _, f := range filterFields {
		if f.Name == "text" {
			text = f.Value
		} else if f.Name == "ids" {
			ids = f.Value
		}
	}

	var items []media.MediaFile
	var err error

	if ids != "" {
		// Return the provided IDs directly
		return strings.Split(ids, ","), nil
	} else {
		// Search by text and filters
		filters := map[string]string{"type": mediaType}
		items, err = media.Search(text, filters, 0, 10000)
		if err != nil {
			return nil, err
		}
	}

	// Extract UUIDs
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.UUID)
	}

	return result, nil
}

// Global instance
var MediaStoreHelperInstance = MediaStoreHelper{}
