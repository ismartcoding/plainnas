package helpers

import (
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/media"
)

func audioTitleFallback(path string) string {
	base := filepath.Base(filepath.ToSlash(path))
	noExt := strings.TrimSuffix(base, filepath.Ext(base))
	if noExt != "" {
		return noExt
	}
	return base
}

func PlaylistAudioFromMediaFile(mf *media.MediaFile) model.PlaylistAudio {
	if mf == nil {
		return model.PlaylistAudio{}
	}

	m := *mf
	p := filepath.ToSlash(m.Path)

	if m.DurationSec <= 0 {
		_, _ = media.EnsureDuration(&m)
	}
	if m.Artist == "" {
		_, _ = media.EnsureArtist(&m)
	}
	if m.Title == "" {
		_, _ = media.EnsureTitle(&m)
	}

	title := strings.TrimSpace(m.Title)
	if title == "" {
		title = audioTitleFallback(p)
	}

	return model.PlaylistAudio{
		Title:    title,
		Artist:   strings.TrimSpace(m.Artist),
		Path:     p,
		Duration: m.DurationSec,
	}
}

func PlaylistAudioFromPath(path string) model.PlaylistAudio {
	p := filepath.ToSlash(path)

	if uuid, _ := media.FindByPath(p); uuid != "" {
		if mf, err := media.GetFile(uuid); err == nil && mf != nil {
			return PlaylistAudioFromMediaFile(mf)
		}
	}

	return model.PlaylistAudio{Title: audioTitleFallback(p), Artist: "", Path: p, Duration: 0}
}
