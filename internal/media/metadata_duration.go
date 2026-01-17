package media

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"ismartcoding/plainnas/internal/media/flacduration"
	"ismartcoding/plainnas/internal/media/mp3duration"
	"ismartcoding/plainnas/internal/media/mp4duration"
)

var errDurationUnavailable = errors.New("duration unavailable")

// EnsureDuration best-effort populates mf.DurationSec and persists it via UpsertMedia
// when it is missing or stale. It avoids doing any work for non audio/video types.
func EnsureDuration(mf *MediaFile) (int, error) {
	if mf == nil {
		return 0, nil
	}
	if mf.Type != "audio" && mf.Type != "video" {
		return mf.DurationSec, nil
	}
	// Cached and still valid.
	if mf.DurationSec > 0 && mf.DurationRefMod == mf.ModifiedAt && mf.DurationRefSize == mf.Size {
		return mf.DurationSec, nil
	}

	d, err := ProbeDurationSec(mf.Path)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, errDurationUnavailable
	}

	mf.DurationSec = d
	mf.DurationRefMod = mf.ModifiedAt
	mf.DurationRefSize = mf.Size
	_ = UpsertMedia(mf)
	return mf.DurationSec, nil
}

// ProbeDurationSec returns duration in seconds for the given media file path.
// Strategy:
//  1. MP4/MOV/M4A via moov->mvhd (ultra-fast, pure Go)
//  2. WAV via RIFF headers (pure Go)
//  3. MP3 CBR via first frame header + file size (pure Go; VBR may fail)
func ProbeDurationSec(path string) (int, error) {
	path = filepath.Clean(path)
	if path == "" {
		return 0, errDurationUnavailable
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp4", ".m4a", ".mov", ".m4v", ".3gp", ".3gpp":
		return probeDurationMP4Container(path)
	case ".wav":
		return probeDurationWAV(path)
	case ".flac":
		return probeDurationFLAC(path)
	case ".mp3":
		return probeDurationMP3(path)
	default:
		return 0, errDurationUnavailable
	}
}

func probeDurationMP4Container(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	d, err := mp4duration.Parse(f)
	if err != nil {
		return 0, errDurationUnavailable
	}
	if d <= 0 {
		return 0, errDurationUnavailable
	}
	sec := d.Seconds()
	if sec <= 0 {
		return 0, errDurationUnavailable
	}
	return int(math.Round(sec)), nil
}

// probeDurationWAV parses RIFF/WAVE headers and computes duration from fmt/data chunks.
func probeDurationWAV(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, 128*1024)
	var hdr [12]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, err
	}
	if string(hdr[0:4]) != "RIFF" || string(hdr[8:12]) != "WAVE" {
		return 0, errDurationUnavailable
	}

	var (
		sampleRate   uint32
		blockAlign   uint16
		dataSize     uint32
		haveFmtChunk bool
		haveData     bool
	)

	for {
		var chunkHdr [8]byte
		if _, err := io.ReadFull(r, chunkHdr[:]); err != nil {
			break
		}
		chunkID := string(chunkHdr[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHdr[4:8])
		switch chunkID {
		case "fmt ":
			buf := make([]byte, chunkSize)
			if _, err := io.ReadFull(r, buf); err != nil {
				return 0, err
			}
			if len(buf) < 16 {
				return 0, errDurationUnavailable
			}
			// AudioFormat := binary.LittleEndian.Uint16(buf[0:2])
			// NumChannels := binary.LittleEndian.Uint16(buf[2:4])
			sampleRate = binary.LittleEndian.Uint32(buf[4:8])
			// ByteRate := binary.LittleEndian.Uint32(buf[8:12])
			blockAlign = binary.LittleEndian.Uint16(buf[12:14])
			// BitsPerSample := binary.LittleEndian.Uint16(buf[14:16])
			haveFmtChunk = true
		case "data":
			dataSize = chunkSize
			haveData = true
			// Skip payload without reading into memory
			if _, err := io.CopyN(io.Discard, r, int64(chunkSize)); err != nil {
				return 0, err
			}
		default:
			// Skip unknown chunk
			if _, err := io.CopyN(io.Discard, r, int64(chunkSize)); err != nil {
				return 0, err
			}
		}
		// Chunks are padded to even sizes
		if chunkSize%2 == 1 {
			_, _ = r.ReadByte()
		}
		if haveFmtChunk && haveData {
			break
		}
	}
	if !haveFmtChunk || !haveData || sampleRate == 0 || blockAlign == 0 || dataSize == 0 {
		return 0, errDurationUnavailable
	}
	sec := float64(dataSize) / float64(uint32(blockAlign)*sampleRate)
	if sec <= 0 {
		return 0, errDurationUnavailable
	}
	return int(math.Round(sec)), nil
}

func probeDurationFLAC(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	d, err := flacduration.Parse(f)
	if err != nil {
		return 0, errDurationUnavailable
	}
	sec := d.Seconds()
	if sec <= 0 {
		return 0, errDurationUnavailable
	}
	return int(math.Round(sec)), nil
}

func probeDurationMP3(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if st.Size() <= 0 {
		return 0, errDurationUnavailable
	}

	d, err := mp3duration.Parse(f, st.Size())
	if err != nil {
		return 0, errDurationUnavailable
	}
	sec := d.Seconds()
	if sec <= 0 {
		return 0, errDurationUnavailable
	}
	return int(math.Round(sec)), nil
}
