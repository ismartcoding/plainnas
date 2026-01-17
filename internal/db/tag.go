package db

import (
	"encoding/json"
	"fmt"
	"ismartcoding/plainnas/internal/pkg/shortid"
	"strings"
)

type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  int    `json:"type"`
	Count int    `json:"count"`
}

// TagRelationRef is an in-memory reference describing the existence of a relation.
// It is not persisted as a value payload; relation presence is encoded in Pebble keys.
type TagRelationRef struct {
	TagID string
	Key   string
}

const tagPrefix = "tag:"
const tagRelationPrefix = "tag_relation:"
const tagRelationByKeyPrefix = "tag_relation_key:"

var tagRelationValue = []byte("1")

func tagKey(id string) string {
	return tagPrefix + id
}

func tagRelationKey(tagID, key string) string {
	return fmt.Sprintf("%s%s:%s", tagRelationPrefix, tagID, key)
}

func tagRelationByKeyKey(key string, tagID string) string {
	return fmt.Sprintf("%s%s:%s", tagRelationByKeyPrefix, key, tagID)
}

func tagRelationByKeyPrefixFor(key string) string {
	return fmt.Sprintf("%s%s:", tagRelationByKeyPrefix, key)
}

func tagRelationPrefixForTagID(tagID string) string {
	return tagRelationPrefix + tagID + ":"
}

// EnsureTagRelationKeyIndex ensures the tag_relation_key: secondary index exists.
// For older DBs that only have tag_relation: keys, it backfills the new index.
func EnsureTagRelationKeyIndex() error {
	db := GetDefault()
	found := false
	_ = db.Iterate([]byte(tagRelationByKeyPrefix), func(_ []byte, _ []byte) error {
		found = true
		return ErrIterateStop
	})
	if found {
		return nil
	}

	// Backfill: read primary keys and populate the by-key index.
	return db.Iterate([]byte(tagRelationPrefix), func(dbKey []byte, _ []byte) error {
		parts := strings.Split(string(dbKey), ":")
		if len(parts) < 3 {
			return nil
		}
		// parts[0] = "tag_relation", parts[1] = tagID, parts[2] = key
		tagID := parts[1]
		key := parts[2]
		idx := tagRelationByKeyKey(key, tagID)
		_ = db.Set([]byte(idx), tagRelationValue, nil)
		return nil
	})
}

func GetTagByID(id string) (*Tag, error) {
	db := GetDefault()
	data, err := db.Get([]byte(tagKey(id)))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var tag Tag
	if err := json.Unmarshal(data, &tag); err != nil {
		return nil, err
	}
	return &tag, nil
}

func GetTagsByType(tagType int) ([]*Tag, error) {
	db := GetDefault()
	var tags []*Tag

	err := db.Iterate([]byte(tagPrefix), func(key []byte, value []byte) error {
		var tag Tag
		if err := json.Unmarshal(value, &tag); err != nil {
			return err
		}
		if tag.Type == tagType {
			tags = append(tags, &tag)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return tags, nil
}

func SaveTag(tag *Tag) error {
	db := GetDefault()
	data, err := json.Marshal(tag)
	if err != nil {
		return err
	}
	return db.Set([]byte(tagKey(tag.ID)), data, nil)
}

func DeleteTag(id string) error {
	db := GetDefault()
	return db.Delete([]byte(tagKey(id)))
}

func AddOrUpdateTag(id string, updateFunc func(*Tag)) (string, error) {
	var tag *Tag
	var err error

	if id == "" {
		// Create new tag
		tag = &Tag{
			ID:    shortid.New(),
			Name:  "",
			Type:  0,
			Count: 0,
		}
	} else {
		// Update existing tag
		tag, err = GetTagByID(id)
		if err != nil {
			return "", err
		}
		if tag == nil {
			return "", fmt.Errorf("tag not found: %s", id)
		}
	}

	// Apply updates
	updateFunc(tag)

	// Save the tag
	if err := SaveTag(tag); err != nil {
		return "", err
	}

	return tag.ID, nil
}

// DeleteTagRelationsByTagID removes all relations for a specific tag
func DeleteTagRelationsByTagID(tagID string) error {
	db := GetDefault()
	var keysToDelete [][]byte

	err := db.Iterate([]byte(tagRelationPrefixForTagID(tagID)), func(dbKey []byte, _ []byte) error {
		parts := strings.Split(string(dbKey), ":")
		if len(parts) < 3 {
			return nil
		}
		// parts[1] = tagID, parts[2] = key
		key := parts[2]
		keysToDelete = append(keysToDelete, dbKey)
		keysToDelete = append(keysToDelete, []byte(tagRelationByKeyKey(key, tagID)))
		return nil
	})

	if err != nil {
		return err
	}

	if err := db.BatchDelete(keysToDelete); err != nil {
		return err
	}
	return RecomputeTagCount(tagID)
}

// Tag Relation CRUD operations

func GetTagRelationsByKey(key string) ([]*TagRelationRef, error) {
	db := GetDefault()
	relations := make([]*TagRelationRef, 0, 8)
	idxPrefix := []byte(tagRelationByKeyPrefixFor(key))
	if err := db.Iterate(idxPrefix, func(dbKey []byte, _ []byte) error {
		parts := strings.Split(string(dbKey), ":")
		if len(parts) < 3 {
			return nil
		}
		// parts[0] = "tag_relation_key", parts[1] = key, parts[2] = tagID
		tagID := parts[2]
		if tagID == "" {
			return nil
		}
		relations = append(relations, &TagRelationRef{TagID: tagID, Key: key})
		return nil
	}); err != nil {
		return nil, err
	}
	return relations, nil
}

func GetTagRelationsByKeys(keys []string) ([]*TagRelationRef, error) {
	db := GetDefault()
	relations := make([]*TagRelationRef, 0, len(keys))
	for _, k := range keys {
		if strings.TrimSpace(k) == "" {
			continue
		}
		idxPrefix := []byte(tagRelationByKeyPrefixFor(k))
		if err := db.Iterate(idxPrefix, func(dbKey []byte, _ []byte) error {
			parts := strings.Split(string(dbKey), ":")
			if len(parts) < 3 {
				return nil
			}
			tagID := parts[2]
			if tagID == "" {
				return nil
			}
			relations = append(relations, &TagRelationRef{TagID: tagID, Key: k})
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return relations, nil
}

// DeleteTagRelationsByKeys deletes all tag relations for the given item keys.
// It uses the by-key index to avoid scanning the full relation space.
func DeleteTagRelationsByKeys(keys []string) error {
	db := GetDefault()
	keysToDelete := make([][]byte, 0, len(keys)*8)
	affected := make(map[string]struct{}, 64)

	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		idxPrefix := []byte(tagRelationByKeyPrefixFor(k))
		if err := db.Iterate(idxPrefix, func(dbKey []byte, _ []byte) error {
			parts := strings.Split(string(dbKey), ":")
			if len(parts) < 3 {
				return nil
			}
			tagID := parts[2]
			if tagID == "" {
				return nil
			}
			keysToDelete = append(keysToDelete, dbKey)
			keysToDelete = append(keysToDelete, []byte(tagRelationKey(tagID, k)))
			affected[tagID] = struct{}{}
			return nil
		}); err != nil {
			return err
		}
	}

	if err := db.BatchDelete(keysToDelete); err != nil {
		return err
	}
	for tagID := range affected {
		if err := RecomputeTagCount(tagID); err != nil {
			return err
		}
	}
	return nil
}

func GetKeysByTagID(tagID string) ([]string, error) {
	db := GetDefault()
	keys := make([]string, 0, 64)
	// Primary key format: tag_relation:<tagID>:<key>
	prefix := []byte(tagRelationPrefixForTagID(tagID))
	err := db.Iterate(prefix, func(dbKey []byte, _ []byte) error {
		parts := strings.Split(string(dbKey), ":")
		if len(parts) < 3 {
			return nil
		}
		// parts[0] = "tag_relation", parts[1] = tagID, parts[2] = key
		if parts[1] == tagID {
			keys = append(keys, parts[2])
		}
		return nil
	})

	return keys, err
}
func DeleteTagRelationsByKeysAndTagID(keys []string, tagID string) error {
	db := GetDefault()
	keysToDelete := make([][]byte, 0, len(keys)*2)
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		keysToDelete = append(keysToDelete, []byte(tagRelationKey(tagID, k)))
		keysToDelete = append(keysToDelete, []byte(tagRelationByKeyKey(k, tagID)))
	}
	if err := db.BatchDelete(keysToDelete); err != nil {
		return err
	}
	return RecomputeTagCount(tagID)
}

func DeleteTagRelationsByKeysAndTagIDs(keys []string, tagIDs []string) error {
	db := GetDefault()
	keysToDelete := make([][]byte, 0, len(keys)*len(tagIDs)*2)
	affected := make(map[string]struct{}, len(tagIDs))

	for _, id := range tagIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		affected[id] = struct{}{}
		for _, k := range keys {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			keysToDelete = append(keysToDelete, []byte(tagRelationKey(id, k)))
			keysToDelete = append(keysToDelete, []byte(tagRelationByKeyKey(k, id)))
		}
	}

	if err := db.BatchDelete(keysToDelete); err != nil {
		return err
	}
	for id := range affected {
		if err := RecomputeTagCount(id); err != nil {
			return err
		}
	}
	return nil
}

func SaveTagRelation(tagID string, key string) error {
	tagID = strings.TrimSpace(tagID)
	key = strings.TrimSpace(key)
	if tagID == "" || key == "" {
		return nil
	}
	db := GetDefault()
	if err := db.Set([]byte(tagRelationKey(tagID, key)), tagRelationValue, nil); err != nil {
		return err
	}
	if err := db.Set([]byte(tagRelationByKeyKey(key, tagID)), tagRelationValue, nil); err != nil {
		return err
	}
	return RecomputeTagCount(tagID)
}

func SaveTagRelations(relations []*TagRelationRef) error {
	if len(relations) == 0 {
		return nil
	}
	affected := make(map[string]struct{}, 8)
	for _, relation := range relations {
		if relation == nil {
			continue
		}
		if err := SaveTagRelation(relation.TagID, relation.Key); err != nil {
			return err
		}
		affected[relation.TagID] = struct{}{}
	}
	// SaveTagRelation already recomputes, but callers may pass duplicates; enforce once.
	for tagID := range affected {
		if err := RecomputeTagCount(tagID); err != nil {
			return err
		}
	}
	return nil
}

// RecomputeTagCount recalculates Tag.Count from the primary tag_relation: entries
// for a specific tagID and persists it into the tag record.
func RecomputeTagCount(tagID string) error {
	tagID = strings.TrimSpace(tagID)
	if tagID == "" {
		return nil
	}
	tag, err := GetTagByID(tagID)
	if err != nil {
		return err
	}
	if tag == nil {
		return nil
	}

	db := GetDefault()
	count := 0
	if err := db.Iterate([]byte(tagRelationPrefixForTagID(tagID)), func(_ []byte, _ []byte) error {
		count++
		return nil
	}); err != nil {
		return err
	}

	if tag.Count == count {
		return nil
	}
	tag.Count = count
	return SaveTag(tag)
}

// RebuildAllTagCounts recomputes Tag.Count for all tags based on existing
// tag_relation: entries.
func RebuildAllTagCounts() error {
	db := GetDefault()
	counts := make(map[string]int, 1024)

	// Count relations per tag by scanning primary keys once.
	if err := db.Iterate([]byte(tagRelationPrefix), func(dbKey []byte, _ []byte) error {
		parts := strings.Split(string(dbKey), ":")
		if len(parts) < 3 {
			return nil
		}
		// parts[0] = "tag_relation", parts[1] = tagID
		tagID := parts[1]
		if tagID != "" {
			counts[tagID]++
		}
		return nil
	}); err != nil {
		return err
	}

	// Update tag records.
	return db.Iterate([]byte(tagPrefix), func(_ []byte, value []byte) error {
		var tag Tag
		if err := json.Unmarshal(value, &tag); err != nil {
			return err
		}
		newCount := counts[tag.ID]
		if tag.Count == newCount {
			return nil
		}
		tag.Count = newCount
		data, err := json.Marshal(&tag)
		if err != nil {
			return err
		}
		return db.Set([]byte(tagKey(tag.ID)), data, nil)
	})
}
