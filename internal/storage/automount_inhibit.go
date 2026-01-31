//go:build linux

package storage

import "sync/atomic"

var autoMountInhibitCount atomic.Int32

// InhibitAutoMount temporarily disables EnsureMountedUSBVolumes.
// Call the returned function to re-enable.
func InhibitAutoMount() (release func()) {
	autoMountInhibitCount.Add(1)
	return func() {
		autoMountInhibitCount.Add(-1)
	}
}

func autoMountInhibited() bool {
	return autoMountInhibitCount.Load() > 0
}
