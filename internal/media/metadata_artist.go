package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

var errArtistUnavailable = errors.New("artist unavailable")

// EnsureArtist best-effort populates mf.Artist and persists it via UpsertMedia
// when it is missing or stale. It avoids doing any work for non-audio types.
func EnsureArtist(mf *MediaFile) (string, error) {
	if mf == nil {
		return "", nil
	}
	if mf.Type != "audio" {
		return mf.Artist, nil
	}
	// Cached and still valid.
	if mf.Artist != "" && mf.ArtistRefMod == mf.ModifiedAt && mf.ArtistRefSize == mf.Size {
		return mf.Artist, nil
	}

	artist, err := ProbeArtist(mf.Path)
	if err != nil {
		return "", err
	}
	if artist == "" {
		return "", errArtistUnavailable
	}

	mf.Artist = artist
	mf.ArtistRefMod = mf.ModifiedAt
	mf.ArtistRefSize = mf.Size
	_ = UpsertMedia(mf)
	return mf.Artist, nil
}

// ProbeArtist returns a best-effort artist string from audio file tags.
func ProbeArtist(path string) (string, error) {
	path = filepath.Clean(path)
	if path == "" {
		return "", errArtistUnavailable
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		return probeArtistMP3(path)
	case ".flac":
		return probeArtistFLAC(path)
	case ".mp4", ".m4a", ".mov", ".m4v", ".3gp", ".3gpp":
		return probeArtistMP4(path)
	default:
		return "", errArtistUnavailable
	}
}

func probeArtistMP3(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// APEv2 tags are common in some MP3 collections (often UTF-8) even when
	// ID3v2 is missing and ID3v1 is present in legacy encodings.
	if a, ok := probeMP3APEv2Text(f, "Artist"); ok {
		a = strings.TrimSpace(a)
		if a != "" {
			return a, nil
		}
	}

	if a, ok := probeMP3ID3v2TextFrame(f, "TPE1"); ok {
		a = strings.TrimSpace(a)
		if a != "" {
			return a, nil
		}
	}
	if a, ok := probeMP3ID3v1TextField(f, 33, 63); ok {
		a = strings.TrimSpace(a)
		if a != "" {
			return a, nil
		}
	}
	return "", errArtistUnavailable
}

func probeMP3APEv2Text(f *os.File, wantKey string) (string, bool) {
	st, err := f.Stat()
	if err != nil {
		return "", false
	}
	sz := st.Size()
	if sz < 32 {
		return "", false
	}

	footerOff := sz - 32
	footer := make([]byte, 32)
	if _, err := f.ReadAt(footer, footerOff); err != nil {
		return "", false
	}
	if string(footer[0:8]) != "APETAGEX" {
		return "", false
	}
	version := binary.LittleEndian.Uint32(footer[8:12])
	if version != 2000 && version != 1000 {
		return "", false
	}
	tagSize := int64(binary.LittleEndian.Uint32(footer[12:16]))
	itemCount := int(binary.LittleEndian.Uint32(footer[16:20]))
	if tagSize < 32 || tagSize > sz {
		return "", false
	}
	if itemCount < 0 || itemCount > 256 {
		return "", false
	}

	tagStart := sz - tagSize
	itemsEnd := sz - 32
	if tagStart < 0 || itemsEnd < tagStart {
		return "", false
	}
	itemsSize := itemsEnd - tagStart
	if itemsSize <= 0 || itemsSize > 8*1024*1024 {
		return "", false
	}

	buf := make([]byte, itemsSize)
	if _, err := f.ReadAt(buf, tagStart); err != nil {
		return "", false
	}

	pos := 0
	for i := 0; i < itemCount && pos+8 <= len(buf); i++ {
		valueSize := int(binary.LittleEndian.Uint32(buf[pos : pos+4]))
		_ = binary.LittleEndian.Uint32(buf[pos+4 : pos+8])
		pos += 8
		if valueSize < 0 || valueSize > len(buf) {
			return "", false
		}
		k0 := bytes.IndexByte(buf[pos:], 0)
		if k0 < 0 {
			return "", false
		}
		key := string(buf[pos : pos+k0])
		pos += k0 + 1
		if pos+valueSize > len(buf) {
			return "", false
		}
		val := buf[pos : pos+valueSize]
		pos += valueSize

		if !strings.EqualFold(key, wantKey) {
			continue
		}
		val = bytes.Trim(val, "\x00")
		s := strings.TrimSpace(decodeLegacyTextBestEffort(val))
		if s == "" {
			return "", false
		}
		return s, true
	}

	return "", false
}

func probeMP3ID3v2TextFrame(f *os.File, wantFrameID string) (string, bool) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", false
	}
	var hdr [10]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return "", false
	}
	if string(hdr[0:3]) != "ID3" {
		return "", false
	}
	ver := hdr[3]
	if ver != 3 && ver != 4 {
		return "", false
	}
	flags := hdr[5]
	// If unsynchronisation is set, we bail out (best-effort).
	if flags&0x80 != 0 {
		return "", false
	}

	tagSize := int(decodeSyncsafe32(hdr[6:10]))
	if tagSize <= 0 || tagSize > 8*1024*1024 {
		return "", false
	}
	buf := make([]byte, tagSize)
	if _, err := io.ReadFull(f, buf); err != nil {
		return "", false
	}

	a := parseID3v2TextFrame(buf, ver, flags, wantFrameID)
	if a == "" {
		return "", false
	}
	return a, true
}

func probeMP3ID3v1TextField(f *os.File, start int, end int) (string, bool) {
	st, err := f.Stat()
	if err != nil {
		return "", false
	}
	if st.Size() < 128 {
		return "", false
	}
	if start < 0 || end > 128 || start >= end {
		return "", false
	}
	if _, err := f.Seek(-128, io.SeekEnd); err != nil {
		return "", false
	}
	var tag [128]byte
	if _, err := io.ReadFull(f, tag[:]); err != nil {
		return "", false
	}
	if string(tag[0:3]) != "TAG" {
		return "", false
	}
	b := bytes.Trim(tag[start:end], "\x00")
	b = bytes.TrimRight(b, " ")
	s := strings.TrimSpace(decodeLegacyTextBestEffort(b))
	if s == "" {
		return "", false
	}
	return s, true
}

func parseID3v2TextFrame(tag []byte, ver byte, flags byte, wantFrameID string) string {
	off := 0
	// Extended header
	if flags&0x40 != 0 {
		if len(tag) < 4 {
			return ""
		}
		var extSize int
		if ver == 3 {
			extSize = int(binary.BigEndian.Uint32(tag[0:4]))
		} else {
			extSize = int(decodeSyncsafe32(tag[0:4]))
		}
		if extSize <= 0 || extSize > len(tag) {
			return ""
		}
		off += extSize
	}

	for {
		if off+10 > len(tag) {
			break
		}
		id := tag[off : off+4]
		if id[0] == 0 && id[1] == 0 && id[2] == 0 && id[3] == 0 {
			break
		}
		frameID := string(id)
		var sz int
		if ver == 3 {
			sz = int(binary.BigEndian.Uint32(tag[off+4 : off+8]))
		} else {
			sz = int(decodeSyncsafe32(tag[off+4 : off+8]))
		}
		if sz <= 0 {
			break
		}
		dataStart := off + 10
		dataEnd := dataStart + sz
		if dataStart < 0 || dataEnd > len(tag) {
			break
		}
		payload := tag[dataStart:dataEnd]
		if frameID == wantFrameID {
			if s := decodeID3Text(payload); s != "" {
				return s
			}
		}
		off = dataEnd
	}
	return ""
}

func decodeID3Text(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	enc := b[0]
	p := b[1:]
	// Trim common terminators.
	p = bytes.Trim(p, "\x00")

	switch enc {
	case 0:
		// ISO-8859-1 per spec, but some libraries write GBK here.
		return strings.TrimSpace(decodeLegacyTextBestEffort(p))
	case 3:
		// UTF-8
		if utf8.Valid(p) {
			return strings.TrimSpace(string(p))
		}
		// Some collections incorrectly store GBK in UTF-8 fields.
		return strings.TrimSpace(decodeLegacyTextBestEffort(p))
	case 1, 2:
		// UTF-16 (BOM for 1) / UTF-16BE for 2
		if len(p) < 2 {
			return ""
		}
		var bo binary.ByteOrder = binary.BigEndian
		if enc == 1 {
			if p[0] == 0xff && p[1] == 0xfe {
				bo = binary.LittleEndian
				p = p[2:]
			} else if p[0] == 0xfe && p[1] == 0xff {
				bo = binary.BigEndian
				p = p[2:]
			}
		}
		if len(p)%2 != 0 {
			p = p[:len(p)-1]
		}
		u16 := make([]uint16, 0, len(p)/2)
		for i := 0; i+1 < len(p); i += 2 {
			u16 = append(u16, bo.Uint16(p[i:i+2]))
		}
		return strings.TrimSpace(string(utf16.Decode(u16)))
	default:
		return ""
	}
}

func decodeLegacyTextBestEffort(b []byte) string {
	b = bytes.Trim(b, "\x00")
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return ""
	}
	if utf8.Valid(b) {
		return string(b)
	}
	if s, ok := tryDecodeGBKIfLikely(b); ok {
		return s
	}
	return latin1ToString(b)
}

func latin1ToString(b []byte) string {
	r := make([]rune, 0, len(b))
	for _, c := range b {
		r = append(r, rune(c))
	}
	return string(r)
}

func tryDecodeGBKIfLikely(b []byte) (string, bool) {
	// Quick GBK validation: accept ASCII; require valid lead/trail pairs for bytes >= 0x80.
	pairs := 0
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			i++
			continue
		}
		if i+1 >= len(b) {
			return "", false
		}
		lead := b[i]
		trail := b[i+1]
		if lead < 0x81 || lead > 0xFE {
			return "", false
		}
		if trail < 0x40 || trail > 0xFE || trail == 0x7F {
			return "", false
		}
		pairs++
		i += 2
	}
	if pairs == 0 {
		return "", false
	}
	out, err := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	if err != nil {
		return "", false
	}
	out = bytes.Trim(out, "\x00")
	out = bytes.TrimSpace(out)
	if len(out) == 0 || !utf8.Valid(out) {
		return "", false
	}
	s := string(out)
	if !containsHan(s) {
		return "", false
	}
	return s, true
}

func containsHan(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

func probeArtistFLAC(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return "", errArtistUnavailable
	}
	if string(magic[:]) != "fLaC" {
		return "", errArtistUnavailable
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
			return "", errArtistUnavailable
		}

		if blockType != 4 {
			if _, err := f.Seek(int64(blockLen), io.SeekCurrent); err != nil {
				return "", errArtistUnavailable
			}
			if isLast {
				break
			}
			continue
		}

		buf := make([]byte, blockLen)
		if _, err := io.ReadFull(f, buf); err != nil {
			return "", errArtistUnavailable
		}
		a, fallback := parseVorbisCommentsFields(buf, []string{"ARTIST"}, []string{"ALBUMARTIST"})
		if a != "" {
			return a, nil
		}
		if best == "" {
			best = fallback
		}
		if isLast {
			break
		}
	}

	best = strings.TrimSpace(best)
	if best != "" {
		return best, nil
	}
	return "", errArtistUnavailable
}

func parseVorbisCommentsFields(b []byte, primaryKeys []string, fallbackKeys []string) (value string, fallback string) {
	if len(b) < 8 {
		return "", ""
	}
	off := 0
	vendorLen := int(binary.LittleEndian.Uint32(b[off:]))
	off += 4
	if vendorLen < 0 || off+vendorLen > len(b) {
		return "", ""
	}
	off += vendorLen
	if off+4 > len(b) {
		return "", ""
	}
	n := int(binary.LittleEndian.Uint32(b[off:]))
	off += 4
	if n < 0 || n > 1_000_000 {
		return "", ""
	}
	primarySet := map[string]bool{}
	for _, k := range primaryKeys {
		primarySet[strings.ToUpper(strings.TrimSpace(k))] = true
	}
	fallbackSet := map[string]bool{}
	for _, k := range fallbackKeys {
		fallbackSet[strings.ToUpper(strings.TrimSpace(k))] = true
	}

	for i := 0; i < n; i++ {
		if off+4 > len(b) {
			break
		}
		l := int(binary.LittleEndian.Uint32(b[off:]))
		off += 4
		if l < 0 || off+l > len(b) {
			break
		}
		c := string(b[off : off+l])
		off += l
		k, v, ok := strings.Cut(c, "=")
		if !ok {
			continue
		}
		ku := strings.ToUpper(strings.TrimSpace(k))
		vv := strings.TrimSpace(v)
		if vv == "" {
			continue
		}
		if primarySet[ku] {
			return vv, fallback
		}
		if fallbackSet[ku] && fallback == "" {
			fallback = vv
		}
	}
	return "", fallback
}

func probeArtistMP4(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", errArtistUnavailable
	}
	fileSize := st.Size()
	if fileSize < 8 {
		return "", errArtistUnavailable
	}

	// Scan only inside moov for performance.
	moovStart, moovSize, ok := findTopLevelBoxLoose(f, fileSize, "moov")
	if !ok {
		return "", errArtistUnavailable
	}
	typeSet := map[string]bool{"\xa9ART": true, "aART": true}
	if a := scanMP4BoxesForTextTypes(f, moovStart, moovSize, 0, typeSet); a != "" {
		return a, nil
	}
	return "", errArtistUnavailable
}

func findTopLevelBoxLoose(f *os.File, fileSize int64, boxType string) (start int64, size int64, ok bool) {
	pos := int64(0)
	for pos+8 <= fileSize {
		bs, bt, header, ok := readMP4BoxHeaderAtLoose(f, pos, fileSize)
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

func scanMP4BoxesForTextTypes(f *os.File, start, size int64, depth int, typeSet map[string]bool) string {
	if depth > 12 {
		return ""
	}
	end := start + size
	pos := start
	for pos+8 <= end {
		bs, bt, header, ok := readMP4BoxHeaderAtLoose(f, pos, end)
		if !ok {
			return ""
		}
		payloadStart := pos + header
		payloadSize := bs - header
		if payloadSize < 0 || payloadStart+payloadSize > end {
			return ""
		}

		switch bt {
		case "meta":
			// FullBox: 4 bytes version/flags then children.
			if payloadSize > 4 {
				if a := scanMP4BoxesForTextTypes(f, payloadStart+4, payloadSize-4, depth+1, typeSet); a != "" {
					return a
				}
			}
		case "moov", "udta", "ilst":
			if a := scanMP4BoxesForTextTypes(f, payloadStart, payloadSize, depth+1, typeSet); a != "" {
				return a
			}
		default:
			if typeSet[bt] {
				if a := parseMP4TextItem(f, payloadStart, payloadSize); a != "" {
					return a
				}
			}
		}

		pos += bs
	}
	return ""
}

func parseMP4TextItem(f *os.File, start, size int64) string {
	end := start + size
	pos := start
	for pos+8 <= end {
		bs, bt, header, ok := readMP4BoxHeaderAtLoose(f, pos, end)
		if !ok {
			return ""
		}
		payloadStart := pos + header
		payloadSize := bs - header
		if payloadSize < 0 || payloadStart+payloadSize > end {
			return ""
		}
		if bt != "data" {
			pos += bs
			continue
		}
		// data is FullBox (4) + type+locale (8) then payload.
		if payloadSize <= 12 {
			pos += bs
			continue
		}
		payload := payloadStart + 12
		payloadLen := payloadSize - 12
		if payloadLen <= 0 || payloadLen > 1<<20 {
			pos += bs
			continue
		}
		b := make([]byte, payloadLen)
		if _, err := f.ReadAt(b, payload); err != nil {
			return ""
		}
		s := strings.TrimSpace(string(bytes.Trim(b, "\x00")))
		return s
	}
	return ""
}

func readMP4BoxHeaderAtLoose(f *os.File, pos int64, limit int64) (boxSize int64, boxType string, headerSize int64, ok bool) {
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
	return sz, bt, headerSize, true
}

func decodeSyncsafe32(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	return (uint32(b[0]&0x7f) << 21) | (uint32(b[1]&0x7f) << 14) | (uint32(b[2]&0x7f) << 7) | uint32(b[3]&0x7f)
}
