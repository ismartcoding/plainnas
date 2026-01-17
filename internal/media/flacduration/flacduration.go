// Package flacduration provides ultra-fast FLAC duration parsing.
//
// It parses FLAC STREAMINFO metadata only (header-only, no decoding).
package flacduration

import (
	"encoding/binary"
	"errors"
	"io"
	"time"
)

var (
	ErrNotFLAC      = errors.New("flac: not a valid flac file")
	ErrNoStreamInfo = errors.New("flac: STREAMINFO not found")
	ErrInvalid      = errors.New("flac: invalid metadata")
)

// Parse reads FLAC duration from STREAMINFO.
func Parse(r io.Reader) (time.Duration, error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return 0, err
	}
	if string(magic[:]) != "fLaC" {
		return 0, ErrNotFLAC
	}

	for {
		var hdr [4]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return 0, err
		}
		isLast := (hdr[0] & 0x80) != 0
		blockType := hdr[0] & 0x7F
		length := int(hdr[1])<<16 | int(hdr[2])<<8 | int(hdr[3])
		if length < 0 {
			return 0, ErrInvalid
		}

		if blockType == 0 {
			// STREAMINFO block length is 34 bytes.
			if length < 34 {
				return 0, ErrInvalid
			}
			buf := make([]byte, 34)
			if _, err := io.ReadFull(r, buf); err != nil {
				return 0, err
			}
			if length > 34 {
				if _, err := io.CopyN(io.Discard, r, int64(length-34)); err != nil {
					return 0, err
				}
			}

			// STREAMINFO layout: 10 bytes before the 8-byte packed field.
			x := binary.BigEndian.Uint64(buf[10:18])
			sampleRate := uint32((x >> 44) & 0xFFFFF) // 20 bits
			totalSamples := x & 0xFFFFFFFFF           // 36 bits
			if sampleRate == 0 || totalSamples == 0 {
				return 0, ErrInvalid
			}
			return time.Duration(totalSamples) * time.Second / time.Duration(sampleRate), nil
		}

		if _, err := io.CopyN(io.Discard, r, int64(length)); err != nil {
			return 0, err
		}
		if isLast {
			break
		}
	}

	return 0, ErrNoStreamInfo
}
