package media

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractEmbeddedCoverMP3_APIC(t *testing.T) {
	pngBytes := mustTinyPNG()

	apicBody := &bytes.Buffer{}
	apicBody.WriteByte(0)             // encoding
	apicBody.WriteString("image/png") // mime
	apicBody.WriteByte(0)
	apicBody.WriteByte(3) // picture type (front cover)
	apicBody.WriteByte(0) // desc terminator
	apicBody.Write(pngBytes)

	frame := &bytes.Buffer{}
	frame.WriteString("APIC")
	_ = binary.Write(frame, binary.BigEndian, uint32(apicBody.Len()))
	frame.Write([]byte{0, 0})
	frame.Write(apicBody.Bytes())

	tagSize := frame.Len()
	sz := encodeSynchsafe32(tagSize)

	mp3 := &bytes.Buffer{}
	mp3.WriteString("ID3")
	mp3.WriteByte(3) // v2.3
	mp3.WriteByte(0)
	mp3.WriteByte(0)
	mp3.Write(sz[:])
	mp3.Write(frame.Bytes())

	p := writeTempFile(t, "cover-*.mp3", mp3.Bytes())
	defer os.Remove(p)

	b, mime, ok := extractEmbeddedCoverMP3(p)
	if !ok {
		t.Fatalf("expected ok")
	}
	if mime != "image/png" {
		t.Fatalf("expected image/png, got %q", mime)
	}
	if !bytes.Equal(b, pngBytes) {
		t.Fatalf("image bytes mismatch")
	}
}

func TestExtractEmbeddedCoverFLAC_PICTURE(t *testing.T) {
	jpgBytes := mustTinyJPEG()

	pic := &bytes.Buffer{}
	binary.Write(pic, binary.BigEndian, uint32(3)) // type
	binary.Write(pic, binary.BigEndian, uint32(len("image/jpeg")))
	pic.WriteString("image/jpeg")
	binary.Write(pic, binary.BigEndian, uint32(0)) // desc len
	binary.Write(pic, binary.BigEndian, uint32(1)) // w
	binary.Write(pic, binary.BigEndian, uint32(1)) // h
	binary.Write(pic, binary.BigEndian, uint32(24))
	binary.Write(pic, binary.BigEndian, uint32(0))
	binary.Write(pic, binary.BigEndian, uint32(len(jpgBytes)))
	pic.Write(jpgBytes)

	flac := &bytes.Buffer{}
	flac.WriteString("fLaC")
	// last=1, type=6
	blockLen := pic.Len()
	flac.WriteByte(0x80 | 6)
	flac.WriteByte(byte((blockLen >> 16) & 0xff))
	flac.WriteByte(byte((blockLen >> 8) & 0xff))
	flac.WriteByte(byte(blockLen & 0xff))
	flac.Write(pic.Bytes())

	p := writeTempFile(t, "cover-*.flac", flac.Bytes())
	defer os.Remove(p)

	b, mime, ok := extractEmbeddedCoverFLAC(p)
	if !ok {
		t.Fatalf("expected ok")
	}
	if mime != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %q", mime)
	}
	if !bytes.Equal(b, jpgBytes) {
		t.Fatalf("image bytes mismatch")
	}
}

func TestExtractEmbeddedCoverMP4_Covr(t *testing.T) {
	jpgBytes := mustTinyJPEG()

	dataPayload := &bytes.Buffer{}
	dataPayload.Write([]byte{0, 0, 0, 0})                   // version/flags
	binary.Write(dataPayload, binary.BigEndian, uint32(13)) // type: JPEG
	binary.Write(dataPayload, binary.BigEndian, uint32(0))  // locale
	dataPayload.Write(jpgBytes)

	dataBox := mp4Box("data", dataPayload.Bytes())
	covrBox := mp4Box("covr", dataBox)
	ilstBox := mp4Box("ilst", covrBox)
	metaPayload := &bytes.Buffer{}
	metaPayload.Write([]byte{0, 0, 0, 0}) // fullbox header
	metaPayload.Write(ilstBox)
	metaBox := mp4Box("meta", metaPayload.Bytes())
	udtaBox := mp4Box("udta", metaBox)
	moovBox := mp4Box("moov", udtaBox)
	ftyp := mp4Box("ftyp", []byte("isom\x00\x00\x00\x00isom"))

	mp4 := &bytes.Buffer{}
	mp4.Write(ftyp)
	mp4.Write(moovBox)

	p := writeTempFile(t, "cover-*.mp4", mp4.Bytes())
	defer os.Remove(p)

	b, mime, ok := extractEmbeddedCoverMP4(p)
	if !ok {
		t.Fatalf("expected ok")
	}
	if mime != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %q", mime)
	}
	if !bytes.Equal(b, jpgBytes) {
		t.Fatalf("image bytes mismatch")
	}
}

func writeTempFile(t *testing.T, pattern string, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return f.Name()
}

func mustTinyPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func mustTinyJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 4, G: 5, B: 6, A: 255})
	buf := &bytes.Buffer{}
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 80}); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func mp4Box(typ string, payload []byte) []byte {
	b := &bytes.Buffer{}
	sz := uint32(8 + len(payload))
	binary.Write(b, binary.BigEndian, sz)
	b.WriteString(typ)
	b.Write(payload)
	return b.Bytes()
}

func encodeSynchsafe32(v int) [4]byte {
	var b [4]byte
	b[0] = byte((v >> 21) & 0x7f)
	b[1] = byte((v >> 14) & 0x7f)
	b[2] = byte((v >> 7) & 0x7f)
	b[3] = byte(v & 0x7f)
	return b
}

func TestWriteTempCoverImage(t *testing.T) {
	b := mustTinyPNG()
	p, cleanup, ok := writeTempCoverImage(b, "image/png")
	if !ok {
		t.Fatalf("expected ok")
	}
	defer cleanup()
	if filepath.Ext(p) != ".png" {
		t.Fatalf("expected .png, got %s", filepath.Ext(p))
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("stat: %v", err)
	}
}

func TestSidecarCoverPreferredOverEmbeddedMP4(t *testing.T) {
	jpgBytes := mustTinyJPEG()

	// Build MP4 with embedded covr.
	dataPayload := &bytes.Buffer{}
	dataPayload.Write([]byte{0, 0, 0, 0})                   // version/flags
	binary.Write(dataPayload, binary.BigEndian, uint32(13)) // type: JPEG
	binary.Write(dataPayload, binary.BigEndian, uint32(0))  // locale
	dataPayload.Write(jpgBytes)
	dataBox := mp4Box("data", dataPayload.Bytes())
	covrBox := mp4Box("covr", dataBox)
	ilstBox := mp4Box("ilst", covrBox)
	metaPayload := &bytes.Buffer{}
	metaPayload.Write([]byte{0, 0, 0, 0})
	metaPayload.Write(ilstBox)
	metaBox := mp4Box("meta", metaPayload.Bytes())
	udtaBox := mp4Box("udta", metaBox)
	moovBox := mp4Box("moov", udtaBox)
	ftyp := mp4Box("ftyp", []byte("isom\x00\x00\x00\x00isom"))
	mp4 := &bytes.Buffer{}
	mp4.Write(ftyp)
	mp4.Write(moovBox)

	dir := t.TempDir()
	mp4Path := filepath.Join(dir, "movie.mp4")
	if err := os.WriteFile(mp4Path, mp4.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile mp4: %v", err)
	}

	// Sidecar should win.
	sidecarPath := filepath.Join(dir, "movie.jpg")
	if err := os.WriteFile(sidecarPath, mustTinyPNG(), 0o600); err != nil {
		t.Fatalf("WriteFile sidecar: %v", err)
	}

	p, cleanup, ok := maybeExtractCoverToTempImage(mp4Path)
	if !ok {
		t.Fatalf("expected ok")
	}
	if cleanup != nil {
		defer cleanup()
	}
	if !strings.HasSuffix(p, "movie.jpg") {
		t.Fatalf("expected sidecar path, got %q", p)
	}
}
