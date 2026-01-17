// Package mp3duration provides ultra-fast MP3 duration parsing.
// Header-only, no decoding, no external dependencies.
//
// Design:
//  1. Skip ID3v2
//  2. Find first MPEG frame header
//  3. Prefer Xing / Info / VBRI
//  4. Fallback to CBR estimation
//
// Safe for NAS / media indexer / batch scan.
package mp3duration

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"time"
)

var (
	ErrNotMP3 = errors.New("mp3: invalid mp3 file")
)

// Parse reads MP3 duration using header-only parsing.
func Parse(r io.ReadSeeker, fileSize int64) (time.Duration, error) {
	// 1) Skip ID3v2
	off, err := skipID3v2(r)
	if err != nil {
		return 0, err
	}

	// 2) Find first frame header (be tolerant to padding/junk)
	syncPos, err := findFrameSync(r, off, 256*1024)
	if err != nil {
		return 0, err
	}
	if _, err := r.Seek(syncPos, io.SeekStart); err != nil {
		return 0, err
	}

	h, err := readFrameHeader(r)
	if err != nil {
		return 0, err
	}

	// 3) Try Xing / Info (mainly Layer III)
	if d, ok := tryXing(r, h); ok {
		return d, nil
	}

	// 4) Try VBRI
	if d, ok := tryVBRI(r, h); ok {
		return d, nil
	}

	// 5) Fallback: CBR estimation
	bitrate := h.Bitrate * 1000
	if bitrate <= 0 {
		return 0, ErrNotMP3
	}
	return time.Duration(fileSize*8) * time.Second / time.Duration(bitrate), nil
}

// ---------------- ID3 ----------------

func skipID3v2(r io.ReadSeeker) (int64, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	var h [10]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return 0, err
	}
	if string(h[0:3]) != "ID3" {
		return r.Seek(0, io.SeekStart)
	}
	size := int64(h[6]&0x7f)<<21 |
		int64(h[7]&0x7f)<<14 |
		int64(h[8]&0x7f)<<7 |
		int64(h[9]&0x7f)
	return r.Seek(10+size, io.SeekStart)
}

func findFrameSync(r io.ReadSeeker, start int64, maxScan int64) (int64, error) {
	if _, err := r.Seek(start, io.SeekStart); err != nil {
		return 0, err
	}

	buf := make([]byte, 32*1024)
	pos := start
	var scanned int64
	var prev byte
	havePrev := false

	for scanned < maxScan {
		n, err := r.Read(buf)
		if n > 0 {
			for i := 0; i < n; i++ {
				b := buf[i]
				if havePrev {
					if prev == 0xFF && (b&0xE0) == 0xE0 {
						// candidate sync at (pos + i - 1)
						return pos + int64(i) - 1, nil
					}
				}
				prev = b
				havePrev = true
			}
			pos += int64(n)
			scanned += int64(n)
		}
		if err != nil {
			break
		}
	}
	return 0, ErrNotMP3
}

// ---------------- Frame Header ----------------

type frameHeader struct {
	Version int // 1, 2, or 25 (for 2.5)
	Layer   int // 1, 2, or 3
	Bitrate int // kbps
	Rate    int // Hz
	Mode    int // 0..3 (3 == mono)
}

func readFrameHeader(r io.Reader) (*frameHeader, error) {
	var b [4]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return nil, err
	}
	if b[0] != 0xFF || (b[1]&0xE0) != 0xE0 {
		return nil, ErrNotMP3
	}

	versionBits := (b[1] >> 3) & 0x03
	layerBits := (b[1] >> 1) & 0x03
	bitrateIdx := (b[2] >> 4) & 0x0F
	rateIdx := (b[2] >> 2) & 0x03
	mode := (b[3] >> 6) & 0x03

	version, ok := map[byte]int{0: 25, 2: 2, 3: 1}[versionBits]
	if !ok {
		return nil, ErrNotMP3
	}
	layer, ok := map[byte]int{1: 3, 2: 2, 3: 1}[layerBits]
	if !ok {
		return nil, ErrNotMP3
	}
	if bitrateIdx == 0 || bitrateIdx == 0x0F || rateIdx == 0x03 {
		return nil, ErrNotMP3
	}

	btByLayer, ok := bitrateTable[version]
	if !ok {
		return nil, ErrNotMP3
	}
	bt, ok := btByLayer[layer]
	if !ok {
		return nil, ErrNotMP3
	}
	bitrate := bt[bitrateIdx]
	rate := sampleRateTable[version][rateIdx]

	if bitrate == 0 || rate == 0 {
		return nil, ErrNotMP3
	}

	return &frameHeader{
		Version: version,
		Layer:   layer,
		Bitrate: bitrate,
		Rate:    rate,
		Mode:    int(mode),
	}, nil
}

func samplesPerFrame(h *frameHeader) int {
	if h == nil {
		return 0
	}
	switch h.Layer {
	case 1:
		return 384
	case 2:
		return 1152
	case 3:
		if h.Version == 1 {
			return 1152
		}
		return 576
	default:
		return 0
	}
}

// ---------------- Xing ----------------

func tryXing(r io.ReadSeeker, h *frameHeader) (time.Duration, bool) {
	// Xing/Info is primarily used with Layer III.
	if h == nil || h.Layer != 3 {
		return 0, false
	}

	// After reading the 4-byte frame header, the reader is positioned at frame side info.
	// Side info length depends on version + channel mode.
	offset := int64(0)
	if h.Version == 1 {
		if h.Mode == 3 {
			offset += 17
		} else {
			offset += 32
		}
	} else {
		if h.Mode == 3 {
			offset += 9
		} else {
			offset += 17
		}
	}

	if _, err := r.Seek(offset, io.SeekCurrent); err != nil {
		return 0, false
	}

	var tag [4]byte
	if _, err := io.ReadFull(r, tag[:]); err != nil {
		return 0, false
	}

	if string(tag[:]) != "Xing" && string(tag[:]) != "Info" {
		return 0, false
	}

	var flags uint32
	if err := binary.Read(r, binary.BigEndian, &flags); err != nil {
		return 0, false
	}
	if flags&0x1 == 0 {
		return 0, false
	}

	var frames uint32
	if err := binary.Read(r, binary.BigEndian, &frames); err != nil {
		return 0, false
	}

	samples := samplesPerFrame(h)
	if samples <= 0 || h.Rate <= 0 {
		return 0, false
	}
	return time.Duration(frames*uint32(samples)) * time.Second / time.Duration(h.Rate), true
}

// ---------------- VBRI ----------------

func tryVBRI(r io.ReadSeeker, h *frameHeader) (time.Duration, bool) {
	if h == nil || h.Rate <= 0 {
		return 0, false
	}
	// VBRI offset: 32 bytes after frame header (as commonly used in Fraunhofer encoders)
	if _, err := r.Seek(32, io.SeekCurrent); err != nil {
		return 0, false
	}

	var tag [4]byte
	if _, err := io.ReadFull(r, tag[:]); err != nil {
		return 0, false
	}
	if string(tag[:]) != "VBRI" {
		return 0, false
	}

	// version(2) delay(2) quality(2) bytes(4) = 10
	if _, err := r.Seek(10, io.SeekCurrent); err != nil {
		return 0, false
	}

	var frames uint32
	if err := binary.Read(r, binary.BigEndian, &frames); err != nil {
		return 0, false
	}

	samples := samplesPerFrame(h)
	if samples <= 0 {
		return 0, false
	}
	return time.Duration(frames*uint32(samples)) * time.Second / time.Duration(h.Rate), true
}

// ---------------- Tables ----------------

var bitrateTable = map[int]map[int][16]int{
	1: { // MPEG1
		1: {0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448, 0},
		2: {0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 0},
		3: {0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},
	},
	2: { // MPEG2
		1: {0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256, 0},
		2: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
		3: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
	},
	25: { // MPEG2.5
		1: {0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256, 0},
		2: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
		3: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
	},
}

var sampleRateTable = map[int][3]int{
	1:  {44100, 48000, 32000},
	2:  {22050, 24000, 16000},
	25: {11025, 12000, 8000},
}

// ParseFile is a convenience helper.
func ParseFile(path string) (time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return Parse(f, st.Size())
}
