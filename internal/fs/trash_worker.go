package fs

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	trashWorkerOnce sync.Once
	trashWorkCh     chan string
)

func ensureTrashWorker() {
	trashWorkerOnce.Do(func() {
		trashWorkCh = make(chan string, 1024)
		go func() {
			for id := range trashWorkCh {
				it, err := loadTrashItem(id)
				if err != nil || it == nil {
					continue
				}
				if it.Size != nil && it.EntryCount != nil {
					continue
				}
				full := trashAbsPath(it.Disk, it.TrashRelPath)
				// Best-effort stats; failures do not affect delete success.
				var size int64
				var count int64
				fi, err := os.Lstat(full)
				if err != nil {
					continue
				}
				if !fi.IsDir() {
					size = fi.Size()
					count = 1
				} else {
					// BFS using WalkDir is fine here; it is async and low priority.
					_ = filepath.WalkDir(full, func(_ string, d os.DirEntry, err error) error {
						if err != nil {
							return nil
						}
						// Count entries (including directories).
						count++
						if !d.IsDir() {
							if info, e := d.Info(); e == nil {
								size += info.Size()
							}
						}
						return nil
					})
				}
				it.Size = &size
				it.EntryCount = &count
				_ = storeTrashItem(it)
			}
		}()
	})
}

func enqueueTrashStats(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	ensureTrashWorker()
	select {
	case trashWorkCh <- id:
	default:
		// drop when busy
	}
}
