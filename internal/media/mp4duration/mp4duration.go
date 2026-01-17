// Package mp4duration provides ultra-fast MP4/MOV/M4A duration parsing.
//
// This is a MINIMAL, duration-only subset:
// - No track parsing
// - No codec parsing
// - No sample tables
// - Only moov -> mvhd
//
// Safe for NAS / indexer / batch scan use.
//
// Usage:
//
//	d, err := mp4duration.Parse(file)
package mp4duration

import (
	"encoding/binary"
	"errors"
	"io"
	"time"
)

var (
	ErrNotMP4    = errors.New("mp4: not a valid MP4 file")
	ErrNoMoov    = errors.New("mp4: moov box not found")
	ErrNoMVHD    = errors.New("mp4: mvhd box not found")
	ErrTimescale = errors.New("mp4: invalid timescale")
)

// Parse reads MP4/MOV/M4A duration from container metadata only.
// It does NOT decode media streams.
func Parse(r io.ReadSeeker) (time.Duration, error) {
	// MP4 files may place moov at head or tail
	// We do a sequential scan of top-level boxes
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	for {
		size, boxType, err := readBoxHeader(r)
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}

		if size < 8 {
			return 0, ErrNotMP4
		}

		if boxType == "moov" {
			return parseMoov(r, size-8)
		}

		// skip box payload
		if _, err := r.Seek(int64(size-8), io.SeekCurrent); err != nil {
			return 0, err
		}
	}

	return 0, ErrNoMoov
}

func parseMoov(r io.ReadSeeker, moovSize uint64) (time.Duration, error) {
	start, _ := r.Seek(0, io.SeekCurrent)
	end := start + int64(moovSize)

	for {
		pos, _ := r.Seek(0, io.SeekCurrent)
		if pos >= end {
			break
		}

		size, boxType, err := readBoxHeader(r)
		if err != nil {
			return 0, err
		}

		if boxType == "mvhd" {
			return parseMVHD(r, size-8)
		}

		if _, err := r.Seek(int64(size-8), io.SeekCurrent); err != nil {
			return 0, err
		}
	}

	return 0, ErrNoMVHD
}

func parseMVHD(r io.Reader, _ uint64) (time.Duration, error) {
	var version uint8
	var flags [3]byte

	if err := binary.Read(r, binary.BigEndian, &version); err != nil {
		return 0, err
	}
	if _, err := io.ReadFull(r, flags[:]); err != nil {
		return 0, err
	}

	var timescale uint32
	var duration uint64

	if version == 1 {
		// creation_time (8) + modification_time (8)
		if _, err := io.CopyN(io.Discard, r, 16); err != nil {
			return 0, err
		}
		if err := binary.Read(r, binary.BigEndian, &timescale); err != nil {
			return 0, err
		}
		if err := binary.Read(r, binary.BigEndian, &duration); err != nil {
			return 0, err
		}
	} else {
		// creation_time (4) + modification_time (4)
		if _, err := io.CopyN(io.Discard, r, 8); err != nil {
			return 0, err
		}
		if err := binary.Read(r, binary.BigEndian, &timescale); err != nil {
			return 0, err
		}
		var d32 uint32
		if err := binary.Read(r, binary.BigEndian, &d32); err != nil {
			return 0, err
		}
		duration = uint64(d32)
	}

	if timescale == 0 {
		return 0, ErrTimescale
	}

	return time.Duration(duration) * time.Second / time.Duration(timescale), nil
}

func readBoxHeader(r io.Reader) (size uint64, boxType string, err error) {
	var size32 uint32
	var typ [4]byte

	if err = binary.Read(r, binary.BigEndian, &size32); err != nil {
		return
	}
	if _, err = io.ReadFull(r, typ[:]); err != nil {
		return
	}

	boxType = string(typ[:])

	if size32 == 1 {
		// largesize
		if err = binary.Read(r, binary.BigEndian, &size); err != nil {
			return
		}
	} else {
		size = uint64(size32)
	}

	return
}
