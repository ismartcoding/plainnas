package fs

// TrashItem is the authoritative metadata for a trashed filesystem entry.
// Stored in Pebble under the trash:* namespace.
//
// Optional fields use pointers so "null" can be represented in JSON.
type TrashItem struct {
	ID           string `json:"id"`
	Type         string `json:"type"` // "file" | "dir"
	OriginalPath string `json:"original_path"`
	Disk         string `json:"disk"` // mountpoint (e.g. /mnt/usb1)
	TrashRelPath string `json:"trash_rel_path"`
	DeletedAt    int64  `json:"deleted_at"`
	UID          int    `json:"uid"`
	GID          int    `json:"gid"`
	Mode         int    `json:"mode"`
	Size         *int64 `json:"size"`
	EntryCount   *int64 `json:"entry_count"`
}
