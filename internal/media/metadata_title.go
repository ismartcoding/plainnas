package media

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var errTitleUnavailable = errors.New("title unavailable")

// EnsureTitle best-effort populates mf.Title and persists it via UpsertMedia
// when it is missing or stale. It avoids doing any work for non-audio types.
func EnsureTitle(mf *MediaFile) (string, error) {
	if mf == nil {
		return "", nil
	}
	if mf.Type != "audio" {
		return mf.Title, nil
	}
	// Cached and still valid.
	if mf.Title != "" && mf.TitleRefMod == mf.ModifiedAt && mf.TitleRefSize == mf.Size {
		return mf.Title, nil
	}

	title, err := ProbeTitle(mf.Path)
	if err != nil {
		return "", err
	}
	if title == "" {
		return "", errTitleUnavailable
	}

	mf.Title = title
	mf.TitleRefMod = mf.ModifiedAt
	mf.TitleRefSize = mf.Size
	_ = UpsertMedia(mf)
	return mf.Title, nil
}

// ProbeTitle returns a best-effort title string from audio file tags.
func ProbeTitle(path string) (string, error) {
	path = filepath.Clean(path)
	if path == "" {
		return "", errTitleUnavailable
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		return probeTitleMP3(path)
	case ".flac":
		return probeTitleFLAC(path)
	case ".mp4", ".m4a", ".mov", ".m4v", ".3gp", ".3gpp":
		return probeTitleMP4(path)
	default:
		return "", errTitleUnavailable
	}
}

func probeTitleMP3(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if t, ok := probeMP3APEv2Text(f, "Title"); ok {
		t = strings.TrimSpace(t)
		if t != "" {
			return t, nil
		}
	}
	if t, ok := probeMP3ID3v2TextFrame(f, "TIT2"); ok {
		t = strings.TrimSpace(t)
		if t != "" {
			return t, nil
		}
	}
	if t, ok := probeMP3ID3v1TextField(f, 3, 33); ok {
		t = strings.TrimSpace(t)
		if t != "" {
			return t, nil
		}
	}
	return "", errTitleUnavailable
}

func probeTitleFLAC(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return "", errTitleUnavailable
	}
	if string(magic[:]) != "fLaC" {
		return "", errTitleUnavailable
	}

	best := ""
	for {
		var hdr [4]byte
		if _, err := io.ReadFull(f, hdr[:]); err != nil {
			break
		}
		isLast := (hdr[0] & 0x80) != 0
		blockType := hdr[0] & 0x7f
		blockLen := int(hdr[1])<<16 | int(hdr[2])<<8 | int(hdr[3])
		if blockLen < 0 || blockLen > 16*1024*1024 {
			return "", errTitleUnavailable
		}

		if blockType != 4 {
			if _, err := f.Seek(int64(blockLen), io.SeekCurrent); err != nil {
				return "", errTitleUnavailable
			}
			if isLast {
				break
			}
			continue
		}

		buf := make([]byte, blockLen)
		if _, err := io.ReadFull(f, buf); err != nil {
			return "", errTitleUnavailable
		}
		t, _ := parseVorbisCommentsFields(buf, []string{"TITLE"}, nil)
		if t != "" {
			return strings.TrimSpace(t), nil
		}
		if best == "" {
			best = t
		}
		if isLast {
			break
		}
	}

	best = strings.TrimSpace(best)
	if best != "" {
		return best, nil
	}
	return "", errTitleUnavailable
}

func probeTitleMP4(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", errTitleUnavailable
	}
	fileSize := st.Size()
	if fileSize < 8 {
		return "", errTitleUnavailable
	}

	moovStart, moovSize, ok := findTopLevelBoxLoose(f, fileSize, "moov")
	if !ok {
		return "", errTitleUnavailable
	}
	typeSet := map[string]bool{"\xa9nam": true}
	if t := scanMP4BoxesForTextTypes(f, moovStart, moovSize, 0, typeSet); t != "" {
		t = strings.TrimSpace(t)
		if t != "" {
			return t, nil
		}
	}
	return "", errTitleUnavailable
}
