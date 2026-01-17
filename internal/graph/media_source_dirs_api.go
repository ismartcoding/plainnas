package graph

import (
	"context"
	"reflect"

	"ismartcoding/plainnas/internal/db"
)

func setMediaSourceDirsModel(ctx context.Context, dirs []string) (bool, error) {
	old := db.GetMediaSourceDirs()
	db.SetMediaSourceDirs(dirs)
	newDirs := db.GetMediaSourceDirs()
	if !reflect.DeepEqual(old, newDirs) {
		// Source dirs affect media indexing. If a scan/reindex is in progress, cancel it and restart.
		rebuildMediaIndex("/")
	}
	return true, nil
}
