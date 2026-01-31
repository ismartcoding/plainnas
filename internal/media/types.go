package media

import (
	"path/filepath"
	"strings"
)

// MediaFile represents a file entry stored in Pebble.
type MediaFile struct {
	UUID string `json:"uuid"`
	// FSUUID is a stable filesystem identifier (typically the filesystem UUID).
	// It is used (together with inode+ctime) for identity and UUID generation.
	FSUUID string `json:"fsuuid"`
	Ino    uint64 `json:"ino"`
	Ctime  int64  `json:"ctime"`
	// Media metadata (best-effort). DurationSec is cached duration in seconds.
	// Duration is considered valid when DurationRefMod/DurationRefSize match current file metadata.
	DurationSec     int   `json:"duration_sec"`
	DurationRefMod  int64 `json:"duration_ref_mod"`
	DurationRefSize int64 `json:"duration_ref_size"`
	// Artist is best-effort extracted from audio tags. It is considered valid when
	// ArtistRefMod/ArtistRefSize match current file metadata.
	Artist        string `json:"artist"`
	ArtistRefMod  int64  `json:"artist_ref_mod"`
	ArtistRefSize int64  `json:"artist_ref_size"`
	// Title is best-effort extracted from audio tags. It is considered valid when
	// TitleRefMod/TitleRefSize match current file metadata.
	Title        string `json:"title"`
	TitleRefMod  int64  `json:"title_ref_mod"`
	TitleRefSize int64  `json:"title_ref_size"`
	// Path is the current physical file path. When trashed, it points to the trash location.
	Path string `json:"path"`
	// OriginalPath preserves the original file path before moving to trash.
	OriginalPath string `json:"original_path"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	ModifiedAt   int64  `json:"modified_at"`
	Type         string `json:"type"`
	// Trash metadata
	IsTrash   bool   `json:"trash"`
	TrashPath string `json:"trash_path"`
	DeletedAt int64  `json:"deleted_at"`
}

func inferType(name string) string {
	lower := strings.ToLower(name)
	ext := strings.ToLower(filepath.Ext(lower))
	switch ext {
	case ".mp3", ".wav", ".wma", ".ogg", ".m4a", ".opus", ".flac", ".aac":
		return "audio"
	case ".mp4", ".mkv", ".webm", ".avi", ".3gp", ".mov", ".m4v", ".3gpp":
		return "video"
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tif", ".tiff", ".heic", ".heif", ".avif", ".svg":
		return "image"
	default:
		return "other"
	}
}
