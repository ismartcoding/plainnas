package media

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func writeTempFileArtist(t *testing.T, pattern string, b []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(b); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return f.Name()
}

func syncsafe32(n int) [4]byte {
	// ID3 uses 7-bit bytes.
	return [4]byte{
		byte((n >> 21) & 0x7f),
		byte((n >> 14) & 0x7f),
		byte((n >> 7) & 0x7f),
		byte(n & 0x7f),
	}
}

func TestProbeArtist_ID3v23_TPE1(t *testing.T) {
	// Minimal MP3-like file with ID3v2.3 tag only.
	// ID3 header: "ID3" + ver 03 00 + flags 00 + size (syncsafe)
	artist := "Test Artist"
	payload := append([]byte{0x00}, []byte(artist)...)

	// Frame: TPE1 + size (big endian) + flags + payload
	frame := make([]byte, 0, 10+len(payload))
	frame = append(frame, []byte("TPE1")...)
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
	p := writeTempFileArtist(t, "artist-*.mp3", b)
	defer os.Remove(p)

	got, err := ProbeArtist(filepath.Clean(p))
	if err != nil {
		t.Fatalf("ProbeArtist err: %v", err)
	}
	if got != artist {
		t.Fatalf("ProbeArtist = %q, want %q", got, artist)
	}
}

func TestProbeArtist_APEv2_Artist(t *testing.T) {
	// APEv2 footer at end-of-file. Values are typically UTF-8.
	artist := "张信哲"

	item := &bytes.Buffer{}
	// valueSize + flags
	binary.Write(item, binary.LittleEndian, uint32(len([]byte(artist))))
	binary.Write(item, binary.LittleEndian, uint32(0))
	item.WriteString("Artist")
	item.WriteByte(0)
	item.WriteString(artist)

	items := item.Bytes()
	footer := &bytes.Buffer{}
	footer.WriteString("APETAGEX")
	binary.Write(footer, binary.LittleEndian, uint32(2000))
	binary.Write(footer, binary.LittleEndian, uint32(len(items)+32))
	binary.Write(footer, binary.LittleEndian, uint32(1))
	binary.Write(footer, binary.LittleEndian, uint32(0))
	footer.Write(make([]byte, 8))

	mp3 := &bytes.Buffer{}
	mp3.Write([]byte{0xff, 0xfb, 0x90, 0x00}) // MP3 frame sync-like prefix
	mp3.Write(items)
	mp3.Write(footer.Bytes())

	p := writeTempFileArtist(t, "artist-ape-*.mp3", mp3.Bytes())
	defer os.Remove(p)

	got, err := ProbeArtist(filepath.Clean(p))
	if err != nil {
		t.Fatalf("ProbeArtist err: %v", err)
	}
	if got != artist {
		t.Fatalf("ProbeArtist = %q, want %q", got, artist)
	}
}
