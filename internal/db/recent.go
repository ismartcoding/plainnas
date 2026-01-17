package db

import (
	"slices"
)

const recentFilesKey = "recent_files"

// AddRecentFile adds a file path into recent files list (most-recent first),
// keeps unique paths and trims to the latest 500 records.
func AddRecentFile(path string) {
	var stored []string
	_ = GetDefault().LoadJSON(recentFilesKey, &stored)

	// Remove existing occurrences
	idx := slices.Index(stored, path)
	if idx >= 0 {
		stored = append(stored[:idx], stored[idx+1:]...)
	}
	// Prepend new path
	stored = append([]string{path}, stored...)
	// Trim to 500
	if len(stored) > 500 {
		stored = stored[:500]
	}
	_ = GetDefault().StoreJSON(recentFilesKey, stored)
}

// GetRecentFiles returns up to limit recent file paths (most-recent first).
func GetRecentFiles(limit int) []string {
	var stored []string
	_ = GetDefault().LoadJSON(recentFilesKey, &stored)
	if limit > 0 && len(stored) > limit {
		return stored[:limit]
	}
	return stored
}
