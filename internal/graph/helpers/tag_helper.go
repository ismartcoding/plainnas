package helpers

import (
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/model"
	"strings"
)

type TagHelper struct{}

func DataTypeToInt(dataType model.DataType) int {
	switch dataType {
	case model.DataTypeDefault:
		return 0
	case model.DataTypeAudio:
		return 1
	case model.DataTypeVideo:
		return 2
	case model.DataTypeImage:
		return 3
	default:
		return 0
	}
}

// AddOrUpdate creates or updates a tag and returns its ID
func (h TagHelper) AddOrUpdate(id string, updateFunc func(*db.Tag)) (string, error) {
	return db.AddOrUpdateTag(id, updateFunc)
}

// Get retrieves a tag by ID and converts it to the GraphQL model
func (h TagHelper) Get(id string) (*model.Tag, error) {
	dbTag, err := db.GetTagByID(id)
	if err != nil {
		return nil, err
	}
	if dbTag == nil {
		return nil, nil
	}

	return &model.Tag{
		ID:    dbTag.ID,
		Name:  dbTag.Name,
		Type:  dbTag.Type,
		Count: dbTag.Count,
	}, nil
}

// GetByType retrieves all tags of a specific type
func (h TagHelper) GetByType(tagType int) ([]*model.Tag, error) {
	dbTags, err := db.GetTagsByType(tagType)
	if err != nil {
		return nil, err
	}

	tags := make([]*model.Tag, 0, len(dbTags))
	for _, dbTag := range dbTags {
		tags = append(tags, &model.Tag{
			ID:    dbTag.ID,
			Name:  dbTag.Name,
			Type:  dbTag.Type,
			Count: dbTag.Count,
		})
	}

	return tags, nil
}

// GetAll retrieves all tags
func (h TagHelper) GetAll() ([]*model.Tag, error) {
	// For simplicity, we'll get all types. In a real implementation,
	// you might want to iterate through all known types or have a separate method.
	allTags := make([]*model.Tag, 0)

	// Get tags for each data type
	for _, dataType := range []int{0, 1, 2, 3} { // DEFAULT, AUDIO, VIDEO, IMAGE
		tags, err := h.GetByType(dataType)
		if err != nil {
			return nil, err
		}
		allTags = append(allTags, tags...)
	}

	return allTags, nil
}

// Delete removes a tag by ID
func (h TagHelper) Delete(id string) error {
	return db.DeleteTag(id)
}

// DeleteTagRelationsByTagId removes all relations for a specific tag
func (h TagHelper) DeleteTagRelationsByTagId(tagID string) error {
	return db.DeleteTagRelationsByTagID(tagID)
}

// GetKeysByTagId returns all keys associated with a tag ID
func (h TagHelper) GetKeysByTagId(tagID string) ([]string, error) {
	return db.GetKeysByTagID(tagID)
}

// AddTagRelations adds multiple tag relations
func (h TagHelper) AddTagRelations(relations []*db.TagRelationRef) error {
	return db.SaveTagRelations(relations)
}

// DeleteTagRelationByKeysTagIds removes tag relations by keys and tag IDs
func (h TagHelper) DeleteTagRelationByKeysTagIds(keys []string, tagIDs []string) error {
	return db.DeleteTagRelationsByKeysAndTagIDs(keys, tagIDs)
}

// GetTagsByKey returns tags associated with a media key (UUID) for a specific type
func (h TagHelper) GetTagsByKey(key string, dataType model.DataType) ([]*model.Tag, error) {
	typeInt := DataTypeToInt(dataType)

	relations, err := db.GetTagRelationsByKey(key)
	if err != nil {
		return nil, err
	}

	tags := make([]*model.Tag, 0, len(relations))
	for _, relation := range relations {
		if relation == nil || relation.TagID == "" {
			continue
		}
		if tag, err := h.Get(relation.TagID); err == nil && tag != nil {
			if tag.Type == typeInt {
				tags = append(tags, tag)
			}
		}
	}

	return tags, nil
}

// GetTagsByKeys returns tags associated with multiple media keys (UUIDs) for a specific type.
// It returns a map from key -> tags.
func (h TagHelper) GetTagsByKeys(keys []string, dataType model.DataType) (map[string][]*model.Tag, error) {
	typeInt := DataTypeToInt(dataType)

	clean := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k != "" {
			clean = append(clean, k)
		}
	}
	if len(clean) == 0 {
		return map[string][]*model.Tag{}, nil
	}

	relations, err := db.GetTagRelationsByKeys(clean)
	if err != nil {
		return nil, err
	}

	byKey := make(map[string][]*db.TagRelationRef, 64)
	for _, rel := range relations {
		if rel == nil || rel.Key == "" {
			continue
		}
		byKey[rel.Key] = append(byKey[rel.Key], rel)
	}

	tagCache := make(map[string]*model.Tag, 128)
	out := make(map[string][]*model.Tag, len(clean))
	for _, k := range clean {
		rels := byKey[k]
		if len(rels) == 0 {
			out[k] = []*model.Tag{}
			continue
		}
		tags := make([]*model.Tag, 0, len(rels))
		for _, rel := range rels {
			if rel == nil || rel.TagID == "" {
				continue
			}
			if cached, ok := tagCache[rel.TagID]; ok {
				if cached.Type == typeInt {
					tags = append(tags, cached)
				}
				continue
			}
			tag, err := h.Get(rel.TagID)
			if err != nil {
				continue
			}
			if tag != nil {
				tagCache[rel.TagID] = tag
				if tag.Type == typeInt {
					tags = append(tags, tag)
				}
			}
		}
		out[k] = tags
	}

	return out, nil
}

// Global instance
var TagHelperInstance = TagHelper{}
