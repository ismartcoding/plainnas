package flacduration

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestParse_StreamInfoDuration(t *testing.T) {
	// Build a minimal FLAC stream:
	// "fLaC" + one STREAMINFO metadata block (type 0), marked as last.
	const sampleRate = uint64(48000)
	const seconds = uint64(123)
	totalSamples := sampleRate * seconds

	streaminfo := make([]byte, 34)
	// STREAMINFO packed field starts at byte 10.
	// We only need sampleRate (20 bits at bits 44..63) and totalSamples (low 36 bits).
	x := (sampleRate << 44) | (totalSamples & 0xFFFFFFFFF)
	binary.BigEndian.PutUint64(streaminfo[10:18], x)

	var b bytes.Buffer
	b.WriteString("fLaC")
	// Metadata block header: isLast=1, type=0 (STREAMINFO), length=34.
	b.WriteByte(0x80)
	b.Write([]byte{0x00, 0x00, 0x22})
	b.Write(streaminfo)

	d, err := Parse(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	want := 123 * time.Second
	if d != want {
		t.Fatalf("duration mismatch: got %v want %v", d, want)
	}
}

func TestParse_NotFLAC(t *testing.T) {
	_, err := Parse(bytes.NewReader([]byte("NOPE")))
	if err != ErrNotFLAC {
		t.Fatalf("expected ErrNotFLAC, got %v", err)
	}
}

func TestParse_NoStreamInfo(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("fLaC")
	// One non-STREAMINFO block (type 4), marked as last, length=0.
	b.WriteByte(0x80 | 0x04)
	b.Write([]byte{0x00, 0x00, 0x00})

	_, err := Parse(bytes.NewReader(b.Bytes()))
	if err != ErrNoStreamInfo {
		t.Fatalf("expected ErrNoStreamInfo, got %v", err)
	}
}

func TestParse_InvalidStreamInfo(t *testing.T) {
	streaminfo := make([]byte, 34)
	// sampleRate=0 and totalSamples>0 should be rejected.
	binary.BigEndian.PutUint64(streaminfo[10:18], 1)

	var b bytes.Buffer
	b.WriteString("fLaC")
	b.WriteByte(0x80)
	b.Write([]byte{0x00, 0x00, 0x22})
	b.Write(streaminfo)

	_, err := Parse(bytes.NewReader(b.Bytes()))
	if err != ErrInvalid {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}
