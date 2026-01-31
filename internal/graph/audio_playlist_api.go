package graph

import (
	"path/filepath"

	"ismartcoding/plainnas/internal/graph/helpers"
	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/media"
)

func addPlaylistAudios(query string) (bool, error) {
	filters := map[string]string{"type": "audio"}
	items, _ := media.Search(query, filters, 0, 1000)
	playlist := helpers.LoadAudioPlaylist()
	existing := make(map[string]struct{}, len(playlist))
	for _, it := range playlist {
		existing[it.Path] = struct{}{}
	}
	for _, f := range items {
		p := filepath.ToSlash(f.Path)
		if _, ok := existing[p]; ok {
			continue
		}
		mf := f
		mf.Path = p
		playlist = append(playlist, helpers.PlaylistAudioFromMediaFile(&mf))
		existing[p] = struct{}{}
	}
	_ = helpers.SaveAudioPlaylist(playlist)
	return true, nil
}

func reorderPlaylistAudios(paths []string) (bool, error) {
	current := helpers.LoadAudioPlaylist()
	if len(current) == 0 || len(paths) == 0 {
		return true, nil
	}
	byPath := make(map[string]model.PlaylistAudio, len(current))
	for _, it := range current {
		byPath[it.Path] = it
	}
	ordered := make([]model.PlaylistAudio, 0, len(current))
	for _, p := range paths {
		p = filepath.ToSlash(p)
		if it, ok := byPath[p]; ok {
			ordered = append(ordered, it)
			delete(byPath, p)
		}
	}
	for _, it := range current {
		if _, ok := byPath[it.Path]; ok {
			ordered = append(ordered, it)
			delete(byPath, it.Path)
		}
	}
	_ = helpers.SaveAudioPlaylist(ordered)
	return true, nil
}

func playAudio(path string) (*model.PlaylistAudio, error) {
	p := filepath.ToSlash(path)
	pa := helpers.PlaylistAudioFromPath(p)
	playlist := helpers.LoadAudioPlaylist()
	found := false
	for _, it := range playlist {
		if it.Path == p {
			found = true
			break
		}
	}
	if !found {
		playlist = append(playlist, pa)
		_ = helpers.SaveAudioPlaylist(playlist)
	}
	_ = helpers.SaveAudioCurrent(p)
	out := pa
	return &out, nil
}

func updateAudioPlayMode(mode model.MediaPlayMode) (bool, error) {
	_ = helpers.SaveAudioMode(mode.String())
	return true, nil
}

func clearAudioPlaylist() (bool, error) {
	_ = helpers.SaveAudioCurrent("")
	_ = helpers.SaveAudioPlaylist([]model.PlaylistAudio{})
	return true, nil
}

func deletePlaylistAudio(path string) (bool, error) {
	p := filepath.ToSlash(path)
	playlist := helpers.LoadAudioPlaylist()
	newList := make([]model.PlaylistAudio, 0, len(playlist))
	for _, it := range playlist {
		if it.Path == p {
			continue
		}
		newList = append(newList, it)
	}
	_ = helpers.SaveAudioPlaylist(newList)
	return true, nil
}
