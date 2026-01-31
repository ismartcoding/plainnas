package graph

import (
	"context"
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/config"
	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/db"
	plainfs "ismartcoding/plainnas/internal/fs"
	"ismartcoding/plainnas/internal/graph/helpers"
	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/media"
)

func app(_ context.Context) (*model.App, error) {
	urlToken := db.GetURLToken()

	indexed, total, state := media.GetProgress()
	pending := total - indexed
	if pending < 0 {
		pending = 0
	}

	stored := helpers.LoadAudioPlaylist()
	playlist := make([]*model.PlaylistAudio, 0, len(stored))
	for i := range stored {
		it := stored[i]
		it.Path = filepath.ToSlash(it.Path)
		if it.Title == "" {
			it.Title = filepath.Base(it.Path)
		}
		playlist = append(playlist, &model.PlaylistAudio{
			Title:    it.Title,
			Artist:   it.Artist,
			Path:     it.Path,
			Duration: it.Duration,
		})
	}
	audioCurrent := helpers.LoadAudioCurrent()
	audioMode := helpers.LoadAudioMode()
	if strings.TrimSpace(audioMode) == "" {
		audioMode = model.MediaPlayModeRepeat.String()
	}

	return &model.App{
		URLToken:     urlToken,
		DocPreviewAvailable: plainfs.LibreOfficeAvailable(),
		HTTPPort:     config.GetDefault().GetInt("graphql.http_port"),
		HTTPSPort:    config.GetDefault().GetInt("graphql.https_port"),
		Audios:       playlist,
		AudioCurrent: audioCurrent,
		AudioMode:    audioMode,
		DataDir:      consts.DATA_DIR,
		ScanProgress: &model.ScanProgress{Indexed: indexed, Pending: pending, Total: total, State: state},
	}, nil
}
