package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/pkg/eventbus"
	"ismartcoding/plainnas/internal/pkg/log"
)

// shouldSkipHiddenDir checks if a hidden directory should be skipped during scanning
func shouldSkipHiddenDir(path string) bool {
	_ = path
	// Unified trash is `.nas-trash` per-disk and must be excluded from scans.
	// For media trash state, use stored metadata/index flags instead of scanning trash directories.
	return true
}

// isFileInTrash checks if a file path is in the trash directory
func isFileInTrash(path string) bool {
	cleanPath := filepath.Clean(path)
	sep := string(filepath.Separator)
	needle := sep + ".nas-trash" + sep
	return strings.Contains(cleanPath, needle) || strings.HasSuffix(cleanPath, sep+".nas-trash")
}

// ScanAndSync walks a root directory to update media DB and index.
func ScanAndSync(root string) error {
	root = filepath.Clean(root)
	log.Infof("Starting media scan from root: %s", root)
	if root == "/" {
		log.Info("Full disk scan mode: will skip system directories for optimal performance")
	}

	// Optional source directory whitelist: only index files under these prefixes.
	// Empty means "all".
	allowedRoots := db.GetMediaSourceDirs()

	// Unified trash is `.nas-trash` per-disk; do not scan trash locations.

	type fidKey struct {
		fsHash uint64
		ino    uint64
		ctime  int64
	}
	seen := make(map[fidKey]struct{})

	// Resolve filesystem id once for non-root scans (hot path optimization).
	rootFSID := ""
	if filepath.Clean(root) != string(filepath.Separator) {
		if fsid, err := filesystemIDForPath(root); err == nil {
			rootFSID = fsid
		}
	}
	var total int64
	var done int64
	atomic.StoreInt32(&stopFlag, 0)
	atomic.StoreInt32(&pauseFlag, 0)
	setStateRunning()

	// Cancel any previous cleanup operations before starting new scan
	if cleanupCancel != nil {
		cleanupCancel()
	}
	// pre-count files (best-effort)
	if consts.ENABLE_SCAN_PRECOUNT {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				// Restrict traversal to allowed roots as early as possible.
				if len(allowedRoots) > 0 && !dirOverlapsAnyPrefix(path, allowedRoots) {
					return filepath.SkipDir
				}
				// Skip virtual filesystems and system directories
				switch path {
				case "/proc", "/sys", "/dev", "/run", "/tmp", "/var/run", "/var/tmp",
					"/var/cache", "/var/log", "/boot", "/lost+found":
					return filepath.SkipDir
				}
				// Skip hidden directories (except trash directory)
				if len(name) > 0 && name[0] == '.' {
					if shouldSkipHiddenDir(path) {
						return filepath.SkipDir
					} else {
						log.Infof("Pre-counting hidden directory (allowed): %s", path)
					}
				}
				// Skip additional system directories
				if strings.HasPrefix(path, "/var/lib/") ||
					strings.HasPrefix(path, "/usr/") ||
					strings.HasPrefix(path, "/lib/") ||
					strings.HasPrefix(path, "/lib64/") ||
					strings.HasPrefix(path, "/sbin/") ||
					strings.HasPrefix(path, "/bin/") {
					return filepath.SkipDir
				}
				return nil
			}
			if len(name) > 0 && name[0] == '.' {
				return nil
			}
			if len(allowedRoots) > 0 && !pathInAnyPrefix(path, allowedRoots) {
				return nil
			}
			atomic.AddInt64(&total, 1)
			return nil
		})
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	doneCh := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				t := atomic.LoadInt64(&total)
				d := atomic.LoadInt64(&done)
				UpdateProgress(d, t)
				eventbus.GetDefault().Publish(consts.EVENT_MEDIA_SCAN_PROGRESS, map[string]any{
					"indexed": d,
					"pending": max64(t-d, 0),
					"total":   t,
					"state":   getStateString(),
					"root":    filepath.ToSlash(root),
				})
			case <-doneCh:
				return
			}
		}
	}()

	// Pipeline: producer sends file entries to a buffered channel; workers Upsert concurrently
	jobs := make(chan *MediaFile, consts.SCAN_PIPELINE_BUFFER)
	doneCh2 := make(chan struct{})
	// workers
	for w := 0; w < consts.SCAN_INDEXER_WORKERS; w++ {
		go func() {
			for mf := range jobs {
				_ = UpsertMedia(mf)
				atomic.AddInt64(&done, 1)
			}
			doneCh2 <- struct{}{}
		}()
	}
	// producer walk
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if IsStopping() {
			return fmt.Errorf("stopped")
		}
		for IsPaused() {
			time.Sleep(200 * time.Millisecond)
		}
		// Yield briefly every few files to smooth CPU on low-end CPUs
		if atomic.LoadInt64(&done)%consts.SCAN_YIELD_EVERY_N == 0 {
			time.Sleep(time.Duration(consts.SCAN_YIELD_MS) * time.Millisecond)
		}
		name := d.Name()
		if d.IsDir() {
			// Restrict traversal to allowed roots as early as possible.
			if len(allowedRoots) > 0 && !dirOverlapsAnyPrefix(path, allowedRoots) {
				return filepath.SkipDir
			}
			// Skip virtual filesystems and system directories
			switch path {
			case "/proc", "/sys", "/dev", "/run", "/tmp", "/var/run", "/var/tmp",
				"/var/cache", "/var/log", "/boot", "/lost+found":
				return filepath.SkipDir
			}
			// Skip hidden directories
			if len(name) > 0 && name[0] == '.' {
				return filepath.SkipDir
			}
			// Skip additional system directories
			if strings.HasPrefix(path, "/var/lib/") ||
				strings.HasPrefix(path, "/usr/") ||
				strings.HasPrefix(path, "/lib/") ||
				strings.HasPrefix(path, "/lib64/") ||
				strings.HasPrefix(path, "/sbin/") ||
				strings.HasPrefix(path, "/bin/") {
				return filepath.SkipDir
			}
			return nil
		}
		if len(name) > 0 && name[0] == '.' {
			return nil
		}
		if len(allowedRoots) > 0 && !pathInAnyPrefix(path, allowedRoots) {
			return nil
		}
		// Skip metadata-like sidecars (legacy)
		if strings.HasSuffix(name, ".metadata") {
			return nil
		}
		// Skip unified trash trees entirely
		if isFileInTrash(path) {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			// Count this entry to avoid pending hanging when stat fails
			atomic.AddInt64(&done, 1)
			return nil
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || st == nil {
			// Count this entry even if stat decoding failed
			atomic.AddInt64(&done, 1)
			return nil
		}
		ino := uint64(st.Ino)
		ctime := st.Ctim.Sec

		fsid := rootFSID
		if fsid == "" {
			if id2, err := filesystemIDForPath(path); err == nil {
				fsid = id2
			}
		}
		if fsid == "" {
			fsid = string(filepath.Separator)
		}
		id := uuidFromTriplet(fsid, ino, ctime)
		k := fidKey{fsHash: fidHash(fsid), ino: ino, ctime: ctime}
		seen[k] = struct{}{}
		// Check existing by FID
		existing, _ := FindUUIDByFID(fsid, ino, ctime)

		m := &MediaFile{
			UUID:       id,
			FSUUID:     fsid,
			Ino:        ino,
			Ctime:      ctime,
			Path:       path,
			Name:       name,
			Size:       info.Size(),
			ModifiedAt: info.ModTime().Unix(),
			Type:       inferType(name),
			IsTrash:    false,
		}
		if existing != "" && existing != id {
			// Preserve previous UUID if triplet matches (rename/move)
			m.UUID = existing
		}
		jobs <- m
		return nil
	})

	// Unified trash is `.nas-trash` per-disk and must not be scanned.

	close(jobs)
	for w := 0; w < consts.SCAN_INDEXER_WORKERS; w++ {
		<-doneCh2
	}

	// Ensure any pending batched index writes are flushed
	_ = FlushMediaIndexBatch()

	finalIndexed := atomic.LoadInt64(&done)
	finalTotal := atomic.LoadInt64(&total)
	log.Infof("Media scan completed: indexed %d files out of %d total", finalIndexed, finalTotal)

	// Immediately reflect completion to UI before heavier cleanup
	close(doneCh)
	setStateIdle()
	eventbus.GetDefault().Publish(consts.EVENT_MEDIA_SCAN_PROGRESS, map[string]any{
		"indexed": finalIndexed,
		"pending": max64(finalTotal-finalIndexed, 0),
		"total":   finalTotal,
		"state":   getStateString(),
		"root":    filepath.ToSlash(root),
	})

	// Cleanup: remove entries under this root that were not seen (run after final progress update)
	// Use a cancellable context to prevent race conditions with subsequent scans
	cleanupCtx, cancel := context.WithCancel(context.Background())
	currentCleanupID := atomic.AddInt64(&cleanupID, 1)
	cleanupCancel = cancel
	needTagCleanup := consumeTagRelationCleanupNeeded()
	go func() {
		defer func() {
			// Clear the cancel function when cleanup completes (only if this is still the current cleanup)
			if atomic.LoadInt64(&cleanupID) == currentCleanupID {
				cleanupCancel = nil
			}
		}()

		batch := make([][]byte, 0, 1000)
		rootPath := filepath.ToSlash(root)

		// For full disk scans, use conservative cleanup - only remove files that don't exist
		if rootPath == "/" {
			log.Info("Starting cleanup: checking for non-existent files")
			deletedCount := 0
			_ = db.GetDefault().Iterate([]byte("media:path:"), func(key []byte, value []byte) error {
				// Check if cleanup was cancelled
				select {
				case <-cleanupCtx.Done():
					return fmt.Errorf("cleanup cancelled")
				default:
				}

				uuid := string(value)
				mf, err := GetFile(uuid)
				if err != nil || mf == nil {
					return nil
				}

				// Only delete if file doesn't exist on disk
				if _, err := os.Stat(mf.Path); os.IsNotExist(err) {
					batch = append(batch, append([]byte{}, key...))
					_ = DeleteMedia(uuid)
					deletedCount++
				}

				if len(batch) >= 1000 {
					_ = db.GetDefault().BatchDelete(batch)
					batch = batch[:0]
				}
				return nil
			})
			if len(batch) > 0 && cleanupCtx.Err() == nil {
				_ = db.GetDefault().BatchDelete(batch)
			}
			log.Infof("Cleanup completed: removed %d non-existent files", deletedCount)
		} else {
			// Normal cleanup for specific directories using seen-based logic
			log.Infof("Starting cleanup for directory: %s", rootPath)
			deletedCount := 0
			prefix := []byte("media:path:" + rootPath)
			_ = db.GetDefault().Iterate(prefix, func(key []byte, value []byte) error {
				// Check if cleanup was cancelled
				select {
				case <-cleanupCtx.Done():
					return fmt.Errorf("cleanup cancelled")
				default:
				}

				uuid := string(value)
				mf, err := GetFile(uuid)
				if err != nil || mf == nil {
					return nil
				}
				// Never delete trashed media entries during scan cleanup.
				// We intentionally skip scanning `.nas-trash`, and trash state is authoritative in metadata.
				if isFileInTrash(mf.Path) {
					return nil
				}
				k := fidKey{fsHash: fidHash(mf.FSUUID), ino: mf.Ino, ctime: mf.Ctime}
				if _, ok := seen[k]; !ok {
					batch = append(batch, append([]byte{}, key...))
					_ = DeleteMedia(uuid)
					deletedCount++
				}
				if len(batch) >= 1000 {
					_ = db.GetDefault().BatchDelete(batch)
					batch = batch[:0]
				}
				return nil
			})
			if len(batch) > 0 && cleanupCtx.Err() == nil {
				_ = db.GetDefault().BatchDelete(batch)
			}
			log.Infof("Cleanup completed: removed %d outdated entries", deletedCount)
		}

		// After a full reset + reindex, clear stale tag relations for UUIDs that are
		// no longer present in the media store (including items that only existed in
		// the old DB, or items that ended up in `.nas-trash` and are not re-indexed).
		if needTagCleanup && cleanupCtx.Err() == nil {
			log.Info("Starting tag relation cleanup after reindex")
			toDelete := make([][]byte, 0, 1000)
			_ = db.GetDefault().Iterate([]byte("tag_relation_key:"), func(key []byte, _ []byte) error {
				select {
				case <-cleanupCtx.Done():
					return fmt.Errorf("cleanup cancelled")
				default:
				}
				parts := strings.Split(string(key), ":")
				if len(parts) < 3 {
					return nil
				}
				uuid := parts[1]
				tagID := parts[2]
				mf, err := GetFile(uuid)
				if err == nil && mf != nil {
					return nil
				}
				// Delete both index and primary relation key.
				toDelete = append(toDelete, append([]byte{}, key...))
				toDelete = append(toDelete, []byte("tag_relation:"+tagID+":"+uuid))
				if len(toDelete) >= 1000 {
					_ = db.GetDefault().BatchDelete(toDelete)
					toDelete = toDelete[:0]
				}
				return nil
			})
			if len(toDelete) > 0 && cleanupCtx.Err() == nil {
				_ = db.GetDefault().BatchDelete(toDelete)
			}
			_ = db.RebuildAllTagCounts()
			log.Info("Tag relation cleanup after reindex completed")
		}
	}()
	return nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
