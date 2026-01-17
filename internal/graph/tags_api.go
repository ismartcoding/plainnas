package graph

import (
	"fmt"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/helpers"
	"ismartcoding/plainnas/internal/graph/model"
)

func mediaTypeFromDataType(typeArg model.DataType) (string, error) {
	switch typeArg {
	case model.DataTypeAudio:
		return "audio", nil
	case model.DataTypeVideo:
		return "video", nil
	case model.DataTypeImage:
		return "image", nil
	default:
		return "", fmt.Errorf("unsupported media type: %s", typeArg)
	}
}

func createTag(typeArg model.DataType, name string) (*model.Tag, error) {
	typeInt := helpers.DataTypeToInt(typeArg)
	id, err := helpers.TagHelperInstance.AddOrUpdate("", func(tag *db.Tag) {
		tag.Name = name
		tag.Type = typeInt
	})
	if err != nil {
		return nil, err
	}
	return helpers.TagHelperInstance.Get(id)
}

func updateTag(id string, name string) (*model.Tag, error) {
	tagID, err := helpers.TagHelperInstance.AddOrUpdate(id, func(tag *db.Tag) {
		tag.Name = name
	})
	if err != nil {
		return nil, err
	}
	return helpers.TagHelperInstance.Get(tagID)
}

func deleteTag(id string) (bool, error) {
	if err := helpers.TagHelperInstance.DeleteTagRelationsByTagId(id); err != nil {
		return false, err
	}
	if err := helpers.TagHelperInstance.Delete(id); err != nil {
		return false, err
	}
	return true, nil
}

func addToTags(typeArg model.DataType, tagIds []string, query string) (bool, error) {
	mediaType, err := mediaTypeFromDataType(typeArg)
	if err != nil {
		return false, err
	}

	items, err := helpers.MediaStoreHelperInstance.GetTagRelationStubs(query, mediaType)
	if err != nil {
		return false, err
	}

	for _, tagID := range tagIds {
		existingKeys, err := helpers.TagHelperInstance.GetKeysByTagId(tagID)
		if err != nil {
			return false, err
		}

		existingKeySet := make(map[string]struct{}, len(existingKeys))
		for _, key := range existingKeys {
			existingKeySet[key] = struct{}{}
		}

		newRelations := make([]*db.TagRelationRef, 0, len(items))
		for _, item := range items {
			if _, exists := existingKeySet[item.Key]; !exists {
				newRelations = append(newRelations, item.ToTagRelation(tagID))
			}
		}

		if len(newRelations) > 0 {
			if err := helpers.TagHelperInstance.AddTagRelations(newRelations); err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

func updateTagRelations(typeArg model.DataType, item model.TagRelationStub, addTagIds []string, removeTagIds []string) (bool, error) {
	tr := helpers.TagRelationStub{Key: item.Key, Title: item.Title, Size: item.Size}

	for _, tagID := range addTagIds {
			relation := tr.ToTagRelation(tagID)
			if err := helpers.TagHelperInstance.AddTagRelations([]*db.TagRelationRef{relation}); err != nil {
			return false, err
		}
	}

	if len(removeTagIds) > 0 {
		if err := helpers.TagHelperInstance.DeleteTagRelationByKeysTagIds([]string{tr.Key}, removeTagIds); err != nil {
			return false, err
		}
	}

	return true, nil
}

func removeFromTags(typeArg model.DataType, tagIds []string, query string) (bool, error) {
	mediaType, err := mediaTypeFromDataType(typeArg)
	if err != nil {
		return false, err
	}

	ids, err := helpers.MediaStoreHelperInstance.GetIDs(query, mediaType)
	if err != nil {
		return false, err
	}

	if err := helpers.TagHelperInstance.DeleteTagRelationByKeysTagIds(ids, tagIds); err != nil {
		return false, err
	}

	return true, nil
}

func listTags(typeArg model.DataType) ([]*model.Tag, error) {
	typeInt := helpers.DataTypeToInt(typeArg)
	return helpers.TagHelperInstance.GetByType(typeInt)
}
