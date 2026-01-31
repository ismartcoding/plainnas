package media

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestProbeTitle_ID3v23_TIT2(t *testing.T) {
	// Minimal MP3-like file with ID3v2.3 tag only.
	title := "Test Title"
	payload := append([]byte{0x00}, []byte(title)...)

	// Frame: TIT2 + size (big endian) + flags + payload
	frame := make([]byte, 0, 10+len(payload))
	frame = append(frame, []byte("TIT2")...)
	sz := make([]byte, 4)
	binary.BigEndian.PutUint32(sz, uint32(len(payload)))
	frame = append(frame, sz...)
	frame = append(frame, 0x00, 0x00) // flags
	frame = append(frame, payload...)

	hdr := make([]byte, 0, 10)
	hdr = append(hdr, []byte("ID3")...)
	hdr = append(hdr, 0x03, 0x00) // v2.3.0
	hdr = append(hdr, 0x00)       // flags
	size := syncsafe32(len(frame))
	hdr = append(hdr, size[:]...)

	b := append(hdr, frame...)
	p := writeTempFileArtist(t, "title-*.mp3", b)
	defer os.Remove(p)

	got, err := ProbeTitle(filepath.Clean(p))
	if err != nil {
		t.Fatalf("ProbeTitle err: %v", err)
	}
	if got != title {
		t.Fatalf("ProbeTitle = %q, want %q", got, title)
	}
}

func TestProbeTitle_ID3v1_GBK(t *testing.T) {
	// Create an ID3v1 tag with a GBK-encoded title.
	want := "信仰"
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(want))
	if err != nil {
		t.Fatalf("GBK encode: %v", err)
	}

	tag := make([]byte, 128)
	copy(tag[0:3], []byte("TAG"))
	copy(tag[3:33], gbk)

	p := writeTempFileArtist(t, "title-id3v1-*.mp3", append([]byte{0xff, 0xfb, 0x90, 0x00}, tag...))
	defer os.Remove(p)

	got, err := ProbeTitle(filepath.Clean(p))
	if err != nil {
		t.Fatalf("ProbeTitle err: %v", err)
	}
	if got != want {
		t.Fatalf("ProbeTitle = %q, want %q", got, want)
	}
}
