package graph

import (
	"os"

	"ismartcoding/plainnas/internal/graph/helpers"
)

// filesCount returns the real entry count of the current directory implied by query.
// This is used by the paged file browser UI to display an accurate total.
func filesCount(query string) (int, error) {
	q := helpers.ParseFilesQuery(query)
	if q.TrashOnly {
		// Keep current behavior simple: return global trash count.
		return trashCount()
	}

	base := helpers.BuildBaseDir(q.RootPath, q.RelativePath)
	st, err := os.Stat(base)
	if err != nil || !st.IsDir() {
		return 0, nil
	}

	count, err := helpers.CountDirEntries(base, q.ShowHidden)
	if err != nil {
		return 0, nil
	}
	return count, nil
}
