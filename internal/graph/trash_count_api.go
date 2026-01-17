package graph

import plainfs "ismartcoding/plainnas/internal/fs"

func trashCount() (int, error) {
	return plainfs.TrashCount()
}
