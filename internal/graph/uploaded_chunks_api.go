package graph

import (
	"os"
	"sort"
	"strconv"
	"strings"

	"ismartcoding/plainnas/internal/consts"
	"path/filepath"
)

func uploadedChunks(fileID string) ([]int, error) {
	dir := filepath.Join(consts.DATA_DIR, "upload_tmp", fileID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []int{}, nil
	}
	out := make([]int, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "chunk_") {
			idxStr := strings.TrimPrefix(name, "chunk_")
			if v, err := strconv.Atoi(idxStr); err == nil {
				out = append(out, v)
			}
		}
	}
	sort.Ints(out)
	return out, nil
}
