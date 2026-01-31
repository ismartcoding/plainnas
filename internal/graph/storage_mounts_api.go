package graph

import (
	"fmt"
	"strings"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/model"
)

const volumeIDRemotePrefix = "remote:"
const volumeIDFSUUIDPrefix = "fsuuid:"
const volumeIDDevPrefix = "dev:"
const mountIDPartitionPrefix = "part:"

// ListMounts returns both mounted volumes and disk partitions.
//
// The list is intentionally "flat"; the UI is expected to correlate mounts with
// disks via StorageMount.diskID and StorageMount.path.
func ListMounts() ([]*model.StorageMount, error) {
	mounted, err := listMountedVolumes()
	if err != nil {
		return mounted, err
	}
	parts, perr := listDiskPartitions()
	if perr != nil {
		// Best-effort: still return mounted volumes.
		return mounted, nil
	}
	return append(mounted, parts...), nil
}

func setMountAlias(id string, alias string) (bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return false, fmt.Errorf("id is empty")
	}
	alias = strings.TrimSpace(alias)
	if err := db.SetVolumeAlias(id, alias); err != nil {
		return false, err
	}
	return true, nil
}
