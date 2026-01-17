package helpers

import (
	"path/filepath"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/model"
)

const (
	keyAudioPlaylist = "audio_playlist"
	keyAudioCurrent  = "audio_current"
	keyAudioMode     = "audio_mode"
)

func LoadAudioPlaylist() []model.PlaylistAudio {
	var items []model.PlaylistAudio
	_ = db.GetDefault().LoadJSON(keyAudioPlaylist, &items)
	return items
}

func SaveAudioPlaylist(items []model.PlaylistAudio) error {
	return db.GetDefault().StoreJSON(keyAudioPlaylist, items)
}

func LoadAudioCurrent() string {
	b, _ := db.GetDefault().Get([]byte(keyAudioCurrent))
	if b == nil {
		return ""
	}
	return string(b)
}

func SaveAudioCurrent(path string) error {
	return db.GetDefault().Set([]byte(keyAudioCurrent), []byte(filepath.ToSlash(path)), nil)
}

func LoadAudioMode() string {
	b, _ := db.GetDefault().Get([]byte(keyAudioMode))
	if b == nil {
		return ""
	}
	return string(b)
}

func SaveAudioMode(mode string) error {
	return db.GetDefault().Set([]byte(keyAudioMode), []byte(mode), nil)
}
