package media

import (
	"context"
	"sync/atomic"
)

const (
	stateIdle    int32 = 0
	stateRunning int32 = 1
	statePaused  int32 = 2
	stateStopped int32 = 3
)

var (
	scanState     int32
	pauseFlag     int32
	stopFlag      int32
	lastIndexed   int64
	lastTotal     int64
	cleanupCancel context.CancelFunc // Cancel previous cleanup operations
	cleanupID     int64              // ID to track current cleanup operation
	// When ResetAllMediaData is used, tag relations may become stale (e.g. UUIDs
	// that only existed in old media DB). We clean them up after the next scan.
	tagRelationCleanupNeeded int32
)

func getStateString() string {
	switch atomic.LoadInt32(&scanState) {
	case stateRunning:
		if atomic.LoadInt32(&pauseFlag) == 1 {
			return "paused"
		}
		return "running"
	case statePaused:
		return "paused"
	case stateStopped:
		return "stopped"
	default:
		return "idle"
	}
}

func setStateRunning() { atomic.StoreInt32(&scanState, stateRunning) }
func setStateIdle()    { atomic.StoreInt32(&scanState, stateIdle) }

func PauseScan()  { atomic.StoreInt32(&pauseFlag, 1); atomic.StoreInt32(&scanState, statePaused) }
func ResumeScan() { atomic.StoreInt32(&pauseFlag, 0); atomic.StoreInt32(&scanState, stateRunning) }
func StopScan() {
	atomic.StoreInt32(&stopFlag, 1)
	atomic.StoreInt32(&scanState, stateStopped)
	// Cancel any running cleanup operations
	if cleanupCancel != nil {
		cleanupCancel()
	}
}

func markTagRelationCleanupNeeded() {
	atomic.StoreInt32(&tagRelationCleanupNeeded, 1)
}

func consumeTagRelationCleanupNeeded() bool {
	return atomic.SwapInt32(&tagRelationCleanupNeeded, 0) == 1
}

func IsPaused() bool   { return atomic.LoadInt32(&pauseFlag) == 1 }
func IsStopping() bool { return atomic.LoadInt32(&stopFlag) == 1 }

// UpdateProgress is called by scanner to store last values
func UpdateProgress(indexed int64, total int64) {
	atomic.StoreInt64(&lastIndexed, indexed)
	atomic.StoreInt64(&lastTotal, total)
}

// GetProgress returns last known values
func GetProgress() (indexed int64, total int64, state string) {
	return atomic.LoadInt64(&lastIndexed), atomic.LoadInt64(&lastTotal), getStateString()
}
