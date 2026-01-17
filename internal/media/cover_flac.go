package media

import (
	"encoding/binary"
	"io"
	"os"
	"strings"
)

func extractEmbeddedCoverFLAC(path string) (b []byte, mime string, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", false
	}
	defer f.Close()

	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return nil, "", false
	}
	if string(magic[:]) != "fLaC" {
		return nil, "", false
	}

	// Iterate metadata blocks until last.
	for {
		var hdr [4]byte
		if _, err := io.ReadFull(f, hdr[:]); err != nil {
			return nil, "", false
		}
		isLast := (hdr[0] & 0x80) != 0
		blockType := hdr[0] & 0x7f
		blockLen := int(hdr[1])<<16 | int(hdr[2])<<8 | int(hdr[3])
		if blockLen < 0 || blockLen > 50*1024*1024 {
			return nil, "", false
		}

		if blockType != 6 {
			if _, err := f.Seek(int64(blockLen), io.SeekCurrent); err != nil {
				return nil, "", false
			}
			if isLast {
				break
			}
			continue
		}

		if blockLen > 25*1024*1024 {
			return nil, "", false
		}
		buf := make([]byte, blockLen)
		if _, err := io.ReadFull(f, buf); err != nil {
			return nil, "", false
		}

		img, mt, ok := parseFLACPictureBlock(buf)
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

func parseFLACPictureBlock(b []byte) (img []byte, mime string, ok bool) {
	// https://xiph.org/flac/format.html#metadata_block_picture
	if len(b) < 4*8 {
		return nil, "", false
	}
	i := 0
	_ = binary.BigEndian.Uint32(b[i:]) // picture type
	i += 4
	if i+4 > len(b) {
		return nil, "", false
	}
	mimeLen := int(binary.BigEndian.Uint32(b[i:]))
	i += 4
	if mimeLen < 0 || i+mimeLen > len(b) {
		return nil, "", false
	}
	mime = strings.ToLower(strings.TrimSpace(string(b[i : i+mimeLen])))
	i += mimeLen
	if i+4 > len(b) {
		return nil, "", false
	}
	descLen := int(binary.BigEndian.Uint32(b[i:]))
	i += 4
	if descLen < 0 || i+descLen > len(b) {
		return nil, "", false
	}
	i += descLen
	// Skip width/height/depth/colors
	if i+16 > len(b) {
		return nil, "", false
	}
	i += 16
	if i+4 > len(b) {
		return nil, "", false
	}
	picLen := int(binary.BigEndian.Uint32(b[i:]))
	i += 4
	if picLen < 0 || i+picLen > len(b) {
		return nil, "", false
	}
	img = b[i : i+picLen]
	if len(img) == 0 {
		return nil, "", false
	}
	return img, mime, true
}
