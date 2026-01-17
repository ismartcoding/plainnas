package graph

import (
	"image"
	"os"
	"path/filepath"
	"time"

	"ismartcoding/plainnas/internal/graph/helpers"
	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/media"
)

func buildFileInfo(id string, path string) (*model.FileInfo, error) {
	if id == path {
		if uuid, _ := media.FindByPath(filepath.ToSlash(path)); uuid != "" {
			id = uuid
		}
	}

	mf, _ := media.GetFileByUUID(id)
	var (
		updatedAt time.Time
		size      int64
	)
	if st, err := os.Stat(path); err == nil {
		updatedAt = st.ModTime()
		size = st.Size()
	} else if mf != nil {
		updatedAt = time.Unix(mf.ModifiedAt, 0)
		size = mf.Size
	} else {
		// Minimal fallback.
		updatedAt = time.Unix(0, 0)
		size = 0
	}

	dataType := model.DataTypeDefault
	if mf != nil {
		switch mf.Type {
		case "image":
			dataType = model.DataTypeImage
		case "video":
			dataType = model.DataTypeVideo
		case "audio":
			dataType = model.DataTypeAudio
		default:
			dataType = model.DataTypeDefault
		}
	}

	tags, err := helpers.TagHelperInstance.GetTagsByKey(id, dataType)
	if err != nil {
		// Tag lookup should not break the lightbox.
		tags = []*model.Tag{}
	}

	var data model.FileInfoData
	if mf != nil {
		switch mf.Type {
		case "image":
			if f, err := os.Open(path); err == nil {
				cfg, _, derr := image.DecodeConfig(f)
				_ = f.Close()
				if derr == nil && cfg.Width > 0 && cfg.Height > 0 {
					w := cfg.Width
					h := cfg.Height
					data = &model.ImageFileInfo{Width: &w, Height: &h}
				}
			}
		case "video", "audio":
			dur := mf.DurationSec
			if dur <= 0 {
				if d, derr := media.EnsureDuration(mf); derr == nil && d > 0 {
					dur = d
				}
			}
			if dur > 0 {
				d := dur
				if mf.Type == "video" {
					data = &model.VideoFileInfo{Duration: &d}
				} else {
					data = &model.AudioFileInfo{Duration: &d}
				}
			} else if mf.Type == "video" {
				data = &model.VideoFileInfo{}
			} else {
				data = &model.AudioFileInfo{}
			}
		}
	}

	return &model.FileInfo{
		Path:      path,
		UpdatedAt: updatedAt,
		Size:      size,
		Tags:      tags,
		Data:      data,
	}, nil
}
