package graph

import (
	"strings"

	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/media"
	"ismartcoding/plainnas/internal/search"
)

type mediaItemsAction string

const (
	mediaItemsActionTrash   mediaItemsAction = "trash"
	mediaItemsActionRestore mediaItemsAction = "restore"
	mediaItemsActionDelete  mediaItemsAction = "delete"
)

func runMediaItemsAction(typeArg model.DataType, query string, action mediaItemsAction) (*model.MediaActionResult, error) {
	fields := search.Parse(query)
	ids := ""
	text := ""
	for _, f := range fields {
		if f.Name == "ids" {
			ids = f.Value
		} else if f.Name == "text" {
			text = f.Value
		}
	}

	filters := map[string]string{}
	if action == mediaItemsActionRestore || action == mediaItemsActionDelete {
		filters["trash"] = "true"
	}
	if typeArg == model.DataTypeAudio || typeArg == model.DataTypeVideo || typeArg == model.DataTypeImage {
		filters["type"] = strings.ToLower(typeArg.String())
	}

	if ids != "" {
		for _, id := range strings.Split(ids, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			switch action {
			case mediaItemsActionTrash:
				// Keep old behavior: trash only if it exists.
				if mf, err := media.GetFileByUUID(id); err == nil && mf != nil {
					_ = media.TrashUUID(id)
				}
			case mediaItemsActionRestore:
				_ = media.RestoreUUID(id)
			case mediaItemsActionDelete:
				_ = media.DeleteUUIDPermanently(id)
			}
		}
	} else {
		items, _ := media.Search(text, filters, 0, 10000)
		for _, it := range items {
			switch action {
			case mediaItemsActionTrash:
				_ = media.TrashUUID(it.UUID)
			case mediaItemsActionRestore:
				_ = media.RestoreUUID(it.UUID)
			case mediaItemsActionDelete:
				_ = media.DeleteUUIDPermanently(it.UUID)
			}
		}
	}

	// Ensure index reflects changes immediately
	_ = media.FlushMediaIndexBatch()
	return &model.MediaActionResult{Type: typeArg, Query: query}, nil
}

func trashMediaItems(typeArg model.DataType, query string) (*model.MediaActionResult, error) {
	return runMediaItemsAction(typeArg, query, mediaItemsActionTrash)
}

func restoreMediaItems(typeArg model.DataType, query string) (*model.MediaActionResult, error) {
	return runMediaItemsAction(typeArg, query, mediaItemsActionRestore)
}

func deleteMediaItems(typeArg model.DataType, query string) (*model.MediaActionResult, error) {
	return runMediaItemsAction(typeArg, query, mediaItemsActionDelete)
}
