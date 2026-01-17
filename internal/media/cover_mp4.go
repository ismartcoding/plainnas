package media

import (
	"encoding/binary"
	"os"
	"strings"
)

func extractEmbeddedCoverMP4(path string) (b []byte, mime string, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", false
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, "", false
	}
	size := st.Size()
	if size < 8 {
		return nil, "", false
	}

	// Only scan inside moov for performance.
	moovStart, moovSize, ok := findTopLevelBox(f, size, "moov")
	if !ok {
		return nil, "", false
	}

	img, mt, ok := scanMP4BoxesForCover(f, moovStart, moovSize, 0)
	if !ok {
		return nil, "", false
	}
	if mt == "" {
		mt = sniffImageMime(img)
	}
	return img, mt, mt != ""
}

func findTopLevelBox(f *os.File, fileSize int64, boxType string) (start int64, size int64, ok bool) {
	pos := int64(0)
	for pos+8 <= fileSize {
		bs, bt, header, ok := readMP4BoxHeaderAt(f, pos, fileSize)
		if !ok {
			return 0, 0, false
		}
		if bt == boxType {
			return pos + header, bs - header, true
		}
		pos += bs
	}
	return 0, 0, false
}

func scanMP4BoxesForCover(f *os.File, start, size int64, depth int) (img []byte, mime string, ok bool) {
	if depth > 12 {
		return nil, "", false
	}
	end := start + size
	pos := start
	for pos+8 <= end {
		bs, bt, header, ok := readMP4BoxHeaderAt(f, pos, end)
		if !ok {
			return nil, "", false
		}
		payloadStart := pos + header
		payloadSize := bs - header
		if payloadSize < 0 || payloadStart+payloadSize > end {
			return nil, "", false
		}

		switch bt {
		case "covr":
			if img, mime, ok = parseMP4CovrBox(f, payloadStart, payloadSize); ok {
				return img, mime, true
			}
		case "meta":
			// meta is a FullBox: 4 bytes version/flags then children.
			if payloadSize <= 4 {
				break
			}
			if img, mime, ok = scanMP4BoxesForCover(f, payloadStart+4, payloadSize-4, depth+1); ok {
				return img, mime, true
			}
		case "moov", "udta", "trak", "mdia", "minf", "stbl", "ilst":
			if img, mime, ok = scanMP4BoxesForCover(f, payloadStart, payloadSize, depth+1); ok {
				return img, mime, true
			}
		}

		pos += bs
	}
	return nil, "", false
}

func parseMP4CovrBox(f *os.File, start, size int64) (img []byte, mime string, ok bool) {
	end := start + size
	pos := start
	for pos+8 <= end {
		bs, bt, header, ok := readMP4BoxHeaderAt(f, pos, end)
		if !ok {
			return nil, "", false
		}
		payloadStart := pos + header
		payloadSize := bs - header
		if payloadSize < 0 || payloadStart+payloadSize > end {
			return nil, "", false
		}

		if bt != "data" {
			pos += bs
			continue
		}
		// data is a FullBox (4 bytes version/flags) + 8 bytes (type + locale) then payload.
		if payloadSize < 12 {
			pos += bs
			continue
		}
		h := make([]byte, 12)
		if _, err := f.ReadAt(h, payloadStart); err != nil {
			return nil, "", false
		}
		dataType := binary.BigEndian.Uint32(h[4:8])
		payload := payloadStart + 12
		payloadLen := payloadSize - 12
		if payloadLen <= 0 || payloadLen > 25*1024*1024 {
			return nil, "", false
		}
		b := make([]byte, payloadLen)
		if _, err := f.ReadAt(b, payload); err != nil {
			return nil, "", false
		}
		switch dataType {
		case 13:
			mime = "image/jpeg"
		case 14:
			mime = "image/png"
		default:
			mime = sniffImageMime(b)
		}
		if mime == "" {
			return nil, "", false
		}
		return b, mime, true
	}
	return nil, "", false
}

func readMP4BoxHeaderAt(f *os.File, pos int64, limit int64) (boxSize int64, boxType string, headerSize int64, ok bool) {
	if pos+8 > limit {
		return 0, "", 0, false
	}
	var h [8]byte
	if _, err := f.ReadAt(h[:], pos); err != nil {
		return 0, "", 0, false
	}
	sz := int64(binary.BigEndian.Uint32(h[0:4]))
	bt := string(h[4:8])
	if sz == 0 {
		sz = limit - pos
		headerSize = 8
		return sz, bt, headerSize, sz >= 8
	}
	if sz == 1 {
		if pos+16 > limit {
			return 0, "", 0, false
		}
		var ext [8]byte
		if _, err := f.ReadAt(ext[:], pos+8); err != nil {
			return 0, "", 0, false
		}
		sz = int64(binary.BigEndian.Uint64(ext[:]))
		headerSize = 16
	} else {
		headerSize = 8
	}
	if sz < headerSize || pos+sz > limit {
		return 0, "", 0, false
	}
	// Sanity: MP4 types are ASCII-ish.
	if len(bt) != 4 || strings.IndexFunc(bt, func(r rune) bool { return r < 0x20 || r > 0x7e }) >= 0 {
		return 0, "", 0, false
	}
	return sz, bt, headerSize, true
}
