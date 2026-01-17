package search

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// in-memory index for search phase: dict + mmap bytes
type mmapIndex struct {
	dict map[string]uint32
	dat  []byte
	idx  []byte
}

func openMmapIndex(dictPath, datPath, idxPath string) (*mmapIndex, error) {
	// load dict
	b, err := os.ReadFile(dictPath)
	if err != nil {
		return nil, err
	}
	var dict map[string]uint32
	if err := json.Unmarshal(b, &dict); err != nil {
		return nil, err
	}
	// mmap dat
	fdDat, err := os.Open(datPath)
	if err != nil {
		return nil, err
	}
	defer fdDat.Close()
	st, err := fdDat.Stat()
	if err != nil {
		return nil, err
	}
	dat, err := unix.Mmap(int(fdDat.Fd()), 0, int(st.Size()), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}
	// mmap idx
	fdIdx, err := os.Open(idxPath)
	if err != nil {
		_ = unix.Munmap(dat)
		return nil, err
	}
	defer fdIdx.Close()
	st2, err := fdIdx.Stat()
	if err != nil {
		_ = unix.Munmap(dat)
		return nil, err
	}
	idx, err := unix.Mmap(int(fdIdx.Fd()), 0, int(st2.Size()), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		_ = unix.Munmap(dat)
		return nil, err
	}
	return &mmapIndex{dict: dict, dat: dat, idx: idx}, nil
}

func (m *mmapIndex) close() {
	if m == nil {
		return
	}
	if m.dat != nil {
		_ = unix.Munmap(m.dat)
	}
	if m.idx != nil {
		_ = unix.Munmap(m.idx)
	}
}

// read posting list for a term id
func (m *mmapIndex) posting(termID uint32) ([]uint64, error) {
	if termID == 0 {
		return nil, nil
	}
	// idx entry is 12 bytes: 8 offset + 4 length
	off := int64((termID - 1) * 12)
	if off+12 > int64(len(m.idx)) {
		return nil, nil
	}
	o := binary.LittleEndian.Uint64(m.idx[off : off+8])
	l := binary.LittleEndian.Uint32(m.idx[off+8 : off+12])
	// read docCount then deltas
	p := int(o)
	end := p + int(l)
	// docCount
	docCount, n := binary.Uvarint(m.dat[p:end])
	if n <= 0 {
		return nil, errors.New("uvarint decode error")
	}
	p += n
	out := make([]uint64, 0, docCount)
	var last uint64
	for i := 0; i < int(docCount) && p < end; i++ {
		v, n2 := binary.Uvarint(m.dat[p:end])
		if n2 <= 0 {
			return nil, errors.New("uvarint decode error")
		}
		p += n2
		last += v
		out = append(out, last)
	}
	return out, nil
}

// postingCapped reads up to capN FileIDs from the posting list (used for fuzzy ngram truncation)
func (m *mmapIndex) postingCapped(termID uint32, capN int) ([]uint64, error) {
	if termID == 0 {
		return nil, nil
	}
	off := int64((termID - 1) * 12)
	if off+12 > int64(len(m.idx)) {
		return nil, nil
	}
	o := binary.LittleEndian.Uint64(m.idx[off : off+8])
	l := binary.LittleEndian.Uint32(m.idx[off+8 : off+12])
	p := int(o)
	end := p + int(l)
	docCount, n := binary.Uvarint(m.dat[p:end])
	if n <= 0 {
		return nil, errors.New("uvarint decode error")
	}
	p += n
	max := int(docCount)
	if capN > 0 && max > capN {
		max = capN
	}
	out := make([]uint64, 0, max)
	var last uint64
	for i := 0; i < max && p < end; i++ {
		v, n2 := binary.Uvarint(m.dat[p:end])
		if n2 <= 0 {
			return nil, errors.New("uvarint decode error")
		}
		p += n2
		last += v
		out = append(out, last)
	}
	return out, nil
}
