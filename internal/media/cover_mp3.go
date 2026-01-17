package media

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"strings"
)

func extractEmbeddedCoverMP3(path string) (b []byte, mime string, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", false
	}
	defer f.Close()

	var h [10]byte
	if _, err := io.ReadFull(f, h[:]); err != nil {
		return nil, "", false
	}
	if string(h[0:3]) != "ID3" {
		return nil, "", false
	}
	ver := h[3]
	flags := h[5]
	if ver != 3 && ver != 4 {
		return nil, "", false
	}

	tagSize := synchsafe32ToInt(h[6:10])
	if tagSize <= 0 || tagSize > 50*1024*1024 {
		return nil, "", false
	}

	pos := int64(10)
	end := pos + int64(tagSize)

	// Skip extended header if present (best-effort).
	if flags&0x40 != 0 {
		var ext [4]byte
		if _, err := io.ReadFull(f, ext[:]); err != nil {
			return nil, "", false
		}
		extSize := int64(binary.BigEndian.Uint32(ext[:]))
		if ver == 4 {
			extSize = int64(synchsafe32ToInt(ext[:]))
		}
		if extSize < 0 || extSize > int64(tagSize) {
			return nil, "", false
		}
		// In v2.3, ext header size excludes these 4 bytes; in v2.4 it includes them.
		// This is messy in the wild; we do a safe skip bounded by tag.
		if ver == 3 {
			pos += 4 + extSize
		} else {
			pos += extSize
		}
		if _, err := f.Seek(pos, io.SeekStart); err != nil {
			return nil, "", false
		}
	}

	for pos+10 <= end {
		if _, err := f.Seek(pos, io.SeekStart); err != nil {
			return nil, "", false
		}
		var fh [10]byte
		if _, err := io.ReadFull(f, fh[:]); err != nil {
			return nil, "", false
		}
		// Padding
		if bytes.Equal(fh[:], make([]byte, 10)) {
			break
		}
		frameID := string(fh[0:4])
		var frameSize int
		if ver == 4 {
			frameSize = synchsafe32ToInt(fh[4:8])
		} else {
			frameSize = int(binary.BigEndian.Uint32(fh[4:8]))
		}
		if frameSize <= 0 {
			break
		}

		pos += 10
		if pos+int64(frameSize) > end {
			break
		}

		if frameID != "APIC" {
			pos += int64(frameSize)
			continue
		}

		// Limit memory: APIC can be large.
		if frameSize > 25*1024*1024 {
			return nil, "", false
		}
		buf := make([]byte, frameSize)
		if _, err := io.ReadFull(f, buf); err != nil {
			return nil, "", false
		}

		img, mt, ok := parseID3APIC(buf)
		if !ok {
			return nil, "", false
		}
		if mt == "" {
			mt = sniffImageMime(img)
		}
		return img, mt, true
	}

	return nil, "", false
}

func synchsafe32ToInt(b []byte) int {
	if len(b) < 4 {
		return 0
	}
	return int(b[0]&0x7f)<<21 | int(b[1]&0x7f)<<14 | int(b[2]&0x7f)<<7 | int(b[3]&0x7f)
}

func parseID3APIC(frame []byte) (img []byte, mime string, ok bool) {
	if len(frame) < 4 {
		return nil, "", false
	}
	enc := frame[0]
	i := 1

	// MIME type (latin1, null-terminated)
	j := bytes.IndexByte(frame[i:], 0)
	if j < 0 {
		return nil, "", false
	}
	mime = strings.ToLower(strings.TrimSpace(string(frame[i : i+j])))
	i += j + 1
	if i >= len(frame) {
		return nil, "", false
	}

	// Picture type
	i++
	if i > len(frame) {
		return nil, "", false
	}

	// Description (null-terminated; depends on encoding)
	switch enc {
	case 0, 3:
		k := bytes.IndexByte(frame[i:], 0)
		if k < 0 {
			return nil, "", false
		}
		i += k + 1
	case 1, 2:
		k := indexUTF16Terminator(frame[i:])
		if k < 0 {
			return nil, "", false
		}
		i += k + 2
	default:
		// Unknown encoding; best effort: treat as no description.
		k := bytes.IndexByte(frame[i:], 0)
		if k < 0 {
			return nil, "", false
		}
		i += k + 1
	}

	if i >= len(frame) {
		return nil, "", false
	}
	img = frame[i:]
	if len(img) == 0 {
		return nil, "", false
	}
	return img, mime, true
}

func indexUTF16Terminator(b []byte) int {
	// Find 0x00 0x00
	for i := 0; i+1 < len(b); i += 2 {
		if b[i] == 0x00 && b[i+1] == 0x00 {
			return i
		}
	}
	// Some tags might not be aligned; fallback to byte scan.
	for i := 0; i+1 < len(b); i++ {
		if b[i] == 0x00 && b[i+1] == 0x00 {
			return i
		}
	}
	return -1
}

func sniffImageMime(b []byte) string {
	if len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff {
		return "image/jpeg"
	}
	if len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) {
		return "image/png"
	}
	if len(b) >= 6 {
		if bytes.Equal(b[:6], []byte("GIF87a")) || bytes.Equal(b[:6], []byte("GIF89a")) {
			return "image/gif"
		}
	}
	return ""
}
