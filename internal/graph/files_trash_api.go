package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	plainfs "ismartcoding/plainnas/internal/fs"
	"ismartcoding/plainnas/internal/media"
)

func trashFiles(paths []string) (bool, error) {
	if len(paths) == 0 {
		return true, nil
	}

	_, err := plainfs.TrashPaths(paths)
	if err != nil {
		return false, err
	}

	// Remove any media DB entries for these paths (and children if directories).
	for _, raw := range paths {
		p := filepath.Clean(raw)
		if strings.TrimSpace(p) == "" || p == "." {
			return false, fmt.Errorf("invalid path")
		}
		if p == string(os.PathSeparator) {
			return false, fmt.Errorf("refusing to trash root")
		}

		fi, statErr := os.Lstat(p)
		if statErr != nil {
			// It was moved; treat as non-existent and purge by prefix best-effort.
			_ = media.RemovePath(p)
			purgeMediaIndexByPathPrefix(p)
			continue
		}

		if fi.IsDir() {
			purgeMediaIndexByPathPrefix(p)
			continue
		}

		_ = media.RemovePath(p)
	}

	return true, nil
}
