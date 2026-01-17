package media

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/db"

	"github.com/cespare/xxhash/v2"
	"golang.org/x/sys/unix"
)

// Custom media index directory and files
var (
	mediaIndexPath string
)

func init() {
	mediaIndexPath = filepath.Join(consts.DATA_DIR, "searchidx_media")
}

func mediaNameDat() string  { return filepath.Join(mediaIndexPath, "name.postings.dat") }
func mediaNameIdx() string  { return filepath.Join(mediaIndexPath, "name.postings.idx") }
func mediaNameDict() string { return filepath.Join(mediaIndexPath, "name.dict.json") }
func mediaPathDat() string  { return filepath.Join(mediaIndexPath, "path.postings.dat") }
func mediaPathIdx() string  { return filepath.Join(mediaIndexPath, "path.postings.idx") }
func mediaPathDict() string { return filepath.Join(mediaIndexPath, "path.dict.json") }

// Ngram index files for fuzzy search
func mediaNameNgramDat() string  { return filepath.Join(mediaIndexPath, "name_ngram.postings.dat") }
func mediaNameNgramIdx() string  { return filepath.Join(mediaIndexPath, "name_ngram.postings.idx") }
func mediaNameNgramDict() string { return filepath.Join(mediaIndexPath, "name_ngram.dict.json") }
func mediaPathNgramDat() string  { return filepath.Join(mediaIndexPath, "path_ngram.postings.dat") }
func mediaPathNgramIdx() string  { return filepath.Join(mediaIndexPath, "path_ngram.postings.idx") }
func mediaPathNgramDict() string { return filepath.Join(mediaIndexPath, "path_ngram.dict.json") }

// ResetMediaIndex closes and removes the media index on disk.
func ResetMediaIndex() error {
	if mediaIndexPath == "" {
		mediaIndexPath = filepath.Join(consts.DATA_DIR, "searchidx_media")
	}
	return os.RemoveAll(mediaIndexPath)
}

// Search queries the media index and returns matched media records.
type mediaMmap struct {
	dict map[string]uint32
	dat  []byte
	idx  []byte
}

func openMediaMmap(dictPath, datPath, idxPath string) (*mediaMmap, error) {
	b, err := os.ReadFile(dictPath)
	if err != nil {
		return nil, err
	}
	var dict map[string]uint32
	if err := json.Unmarshal(b, &dict); err != nil {
		return nil, err
	}
	fdat, err := os.Open(datPath)
	if err != nil {
		return nil, err
	}
	defer fdat.Close()
	st, err := fdat.Stat()
	if err != nil {
		return nil, err
	}
	dat, err := unix.Mmap(int(fdat.Fd()), 0, int(st.Size()), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}
	fidx, err := os.Open(idxPath)
	if err != nil {
		_ = unix.Munmap(dat)
		return nil, err
	}
	defer fidx.Close()
	st2, err := fidx.Stat()
	if err != nil {
		_ = unix.Munmap(dat)
		return nil, err
	}
	idx, err := unix.Mmap(int(fidx.Fd()), 0, int(st2.Size()), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		_ = unix.Munmap(dat)
		return nil, err
	}
	return &mediaMmap{dict: dict, dat: dat, idx: idx}, nil
}

func (m *mediaMmap) close() {
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

func (m *mediaMmap) posting(termID uint32) ([]uint64, error) {
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
		return nil, fmt.Errorf("uvarint decode error")
	}
	p += n
	out := make([]uint64, 0, docCount)
	var last uint64
	for i := 0; i < int(docCount) && p < end; i++ {
		v, n2 := binary.Uvarint(m.dat[p:end])
		if n2 <= 0 {
			return nil, fmt.Errorf("uvarint decode error")
		}
		p += n2
		last += v
		out = append(out, last)
	}
	return out, nil
}

// postingCapped reads up to capN FileIDs from the posting list (used for fuzzy ngram truncation)
func (m *mediaMmap) postingCapped(termID uint32, capN int) ([]uint64, error) {
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
		return nil, fmt.Errorf("uvarint decode error")
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
			return nil, fmt.Errorf("uvarint decode error")
		}
		p += n2
		last += v
		out = append(out, last)
	}
	return out, nil
}

// BuildMediaIndex rebuilds the media inverted index from Pebble records.
func BuildMediaIndex() error {
	_ = os.MkdirAll(mediaIndexPath, 0o755)
	names := make(map[string][]uint64, 1<<16)
	paths := make(map[string][]uint64, 1<<16)
	// Fuzzy ngram term maps
	nameNgrams := make(map[string][]uint64, 1<<16)
	pathNgrams := make(map[string][]uint64, 1<<16)
	peb := db.GetDefault()
	err := peb.Iterate([]byte("media:uuid:"), func(_ []byte, value []byte) error {
		var m MediaFile
		if err := json.Unmarshal(value, &m); err != nil {
			return nil
		}
		docID := xxhash.Sum64String(m.UUID)
		_ = peb.Set(keyByDocID(docID), []byte(m.UUID), nil)
		for _, t := range tokenize(m.Name) {
			names[t] = append(names[t], docID)
		}
		for _, t := range tokenize(filepath.ToSlash(m.Path)) {
			paths[t] = append(paths[t], docID)
		}
		// ngram tokens
		for _, ng := range buildQueryNgrams(m.Name) {
			nameNgrams[ng] = append(nameNgrams[ng], docID)
		}
		for _, ng := range buildQueryNgrams(filepath.ToSlash(m.Path)) {
			pathNgrams[ng] = append(pathNgrams[ng], docID)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := buildIndexFiles(names, mediaNameDict(), mediaNameDat(), mediaNameIdx()); err != nil {
		return err
	}
	if err := buildIndexFiles(paths, mediaPathDict(), mediaPathDat(), mediaPathIdx()); err != nil {
		return err
	}
	// Build ngram indexes (append-only)
	if err := buildIndexFiles(nameNgrams, mediaNameNgramDict(), mediaNameNgramDat(), mediaNameNgramIdx()); err != nil {
		return err
	}
	if err := buildIndexFiles(pathNgrams, mediaPathNgramDict(), mediaPathNgramDat(), mediaPathNgramIdx()); err != nil {
		return err
	}
	return nil
}

func Search(query string, filters map[string]string, offset int, limit int) ([]MediaFile, error) {
	// Normalize filters
	filtType := strings.ToLower(filters["type"])
	filtTrashStr := strings.ToLower(filters["trash"]) // "true" or "false" or empty
	hasTrashFilter := filtTrashStr != ""
	wantTrash := filtTrashStr == "true"
	pathPrefixRaw := filters["path_prefix"]
	pathPrefixes := func() []string {
		raw := strings.TrimSpace(pathPrefixRaw)
		if raw == "" {
			return nil
		}
		parts := strings.Split(raw, "|")
		out := make([]string, 0, len(parts))
		seen := map[string]struct{}{}
		for _, p := range parts {
			p = filepath.ToSlash(filepath.Clean(strings.TrimSpace(p)))
			if p == "" {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
		if len(out) == 0 {
			return nil
		}
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out
	}()
	matchPathPrefix := func(path string) bool {
		if len(pathPrefixes) == 0 {
			return true
		}
		p := filepath.ToSlash(filepath.Clean(path))
		for _, pre := range pathPrefixes {
			if strings.HasPrefix(p, pre) {
				return true
			}
		}
		return false
	}

	// Handle ids: queries directly
	if strings.HasPrefix(query, "ids:") {
		idsStr := strings.TrimPrefix(query, "ids:")
		ids := strings.Split(idsStr, ",")
		out := make([]MediaFile, 0, len(ids))
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if mf, err := GetFile(id); err == nil && mf != nil {
				// Apply type filter if specified
				if filtType != "" && mf.Type != filtType {
					continue
				}
				// Apply trash filter if specified
				if hasTrashFilter && mf.IsTrash != wantTrash {
					continue
				}
				// Apply path prefix filter(s)
				if !matchPathPrefix(mf.Path) {
					continue
				}
				out = append(out, *mf)
			}
		}
		// Apply offset and limit
		if offset > len(out) {
			return []MediaFile{}, nil
		}
		end := offset + limit
		if end > len(out) {
			end = len(out)
		}
		return out[offset:end], nil
	}

	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	// Try index-backed search first; if index missing or fails, fallback to Pebble scan
	useFallback := !MediaIndexExists()
	var out []MediaFile
	if !useFallback {
		// Open exact indexes for name and path
		nm, err1 := openMediaMmap(mediaNameDict(), mediaNameDat(), mediaNameIdx())
		pm, err2 := openMediaMmap(mediaPathDict(), mediaPathDat(), mediaPathIdx())
		if err1 != nil || err2 != nil {
			if nm != nil {
				nm.close()
			}
			if pm != nil {
				pm.close()
			}
			useFallback = true
		} else {
			defer nm.close()
			defer pm.close()
			exactTerms := tokenize(query)
			var idsExact []uint64
			if len(exactTerms) > 0 {
				type termSet struct{ ids []uint64 }
				sets := make([]termSet, 0, len(exactTerms))
				for _, t := range exactTerms {
					nid := nm.dict[t]
					pid := pm.dict[t]
					npl, _ := nm.posting(nid)
					ppl, _ := pm.posting(pid)
					union := unionSorted(npl, ppl)
					sets = append(sets, termSet{ids: union})
				}
				sort.Slice(sets, func(i, j int) bool { return len(sets[i].ids) < len(sets[j].ids) })
				for i, s := range sets {
					if i == 0 {
						idsExact = s.ids
					} else {
						idsExact = intersectSorted(idsExact, s.ids)
					}
					if len(idsExact) == 0 {
						break
					}
				}
			}
			enableFuzzy := true
			if len(idsExact) >= limit*3 {
				enableFuzzy = false
			}
			finalIDs := idsExact
			if enableFuzzy {
				fnm, e1 := openMediaMmap(mediaNameNgramDict(), mediaNameNgramDat(), mediaNameNgramIdx())
				fpm, e2 := openMediaMmap(mediaPathNgramDict(), mediaPathNgramDat(), mediaPathNgramIdx())
				if e1 == nil && e2 == nil {
					defer fnm.close()
					defer fpm.close()
					ngrams := buildQueryNgrams(query)
					if len(ngrams) > 0 {
						type termSet struct{ ids []uint64 }
						fsets := make([]termSet, 0, len(ngrams))
						for _, ng := range ngrams {
							nid := fnm.dict[ng]
							pid := fpm.dict[ng]
							npl, _ := fnm.postingCapped(nid, 20000)
							ppl, _ := fpm.postingCapped(pid, 20000)
							union := unionSorted(npl, ppl)
							if len(union) > 0 {
								fsets = append(fsets, termSet{ids: union})
							}
						}
						sort.Slice(fsets, func(i, j int) bool { return len(fsets[i].ids) < len(fsets[j].ids) })
						var idsFuzzy []uint64
						for i, s := range fsets {
							if i == 0 {
								idsFuzzy = s.ids
							} else {
								idsFuzzy = intersectSorted(idsFuzzy, s.ids)
							}
							if len(idsFuzzy) == 0 {
								break
							}
						}
						finalIDs = unionSorted(finalIDs, idsFuzzy)
					}
				}
			}
			peb := db.GetDefault()
			out = make([]MediaFile, 0, len(finalIDs))
			for _, docID := range finalIDs {
				b, _ := peb.Get(keyByDocID(docID))
				if b == nil {
					continue
				}
				uuid := string(b)
				mf, err := GetFile(uuid)
				if err != nil || mf == nil {
					continue
				}
				if filtType != "" && mf.Type != filtType {
					continue
				}
				if hasTrashFilter && mf.IsTrash != wantTrash {
					continue
				}
				if !matchPathPrefix(mf.Path) {
					continue
				}
				out = append(out, *mf)
			}
			if len(out) == 0 && strings.TrimSpace(query) != "" {
				useFallback = true
			}
		}
	}

	if useFallback {
		// Fallback: iterate all media and do substring matching on tokens
		peb := db.GetDefault()
		terms := tokenize(query)
		rawq := strings.ToLower(strings.TrimSpace(query))
		lowerHas := func(s string, term string) bool {
			return strings.Contains(strings.ToLower(s), term)
		}
		matches := func(mf *MediaFile) bool {
			if mf == nil {
				return false
			}
			// Filters first
			if filtType != "" && mf.Type != filtType {
				return false
			}
			if hasTrashFilter && mf.IsTrash != wantTrash {
				return false
			}
			if !matchPathPrefix(mf.Path) {
				return false
			}
			if len(terms) == 0 {
				// No tokens (likely non-ASCII query). Use raw substring matching.
				if rawq == "" {
					return true
				}
				return lowerHas(mf.Name, rawq) || lowerHas(filepath.ToSlash(mf.Path), rawq)
			}
			name := mf.Name
			path := filepath.ToSlash(mf.Path)
			for _, t := range terms {
				if !(lowerHas(name, t) || lowerHas(path, t)) {
					return false
				}
			}
			return true
		}
		out = make([]MediaFile, 0, 256)
		_ = peb.Iterate([]byte("media:uuid:"), func(_ []byte, value []byte) error {
			var mf MediaFile
			if err := json.Unmarshal(value, &mf); err != nil {
				return nil
			}
			if matches(&mf) {
				out = append(out, mf)
			}
			return nil
		})
	}

	if offset > len(out) {
		return []MediaFile{}, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}

// Helpers for tokenization and index file building (shared pattern with FS search)
func tokenize(s string) []string {
	s = strings.ToLower(s)
	repl := func(r rune) rune {
		switch r {
		case '/', '.', '_', '-', ' ':
			return ' '
		default:
			if r < 128 {
				return r
			}
			return ' '
		}
	}
	s = strings.Map(repl, s)
	return strings.Fields(s)
}

// buildQueryNgrams returns query ngrams for fuzzy search
// ASCII: lowercase; split by separators; 2-gram for tokens with length >= 3
// CJK: bigram on contiguous CJK sequences without using any dictionary
func buildQueryNgrams(s string) []string {
	s = strings.ToLower(s)
	toks := tokenize(s)
	out := make([]string, 0, 16)
	// ASCII 2-gram
	for _, t := range toks {
		if len(t) < 3 {
			continue
		}
		for i := 0; i+2 <= len(t); i++ {
			out = append(out, t[i:i+2])
		}
	}
	// CJK bigrams from original string
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		if isCJK(runes[i]) {
			j := i + 1
			for j < len(runes) && isCJK(runes[j]) {
				j++
			}
			if j-i >= 2 {
				for k := i; k+1 < j; k++ {
					out = append(out, string(runes[k:k+2]))
				}
			}
			i = j
		} else {
			i++
		}
	}
	return out
}

func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF:
		return true
	case r >= 0x3400 && r <= 0x4DBF:
		return true
	case r >= 0xF900 && r <= 0xFAFF:
		return true
	case r >= 0x3040 && r <= 0x309F:
		return true
	case r >= 0x30A0 && r <= 0x30FF:
		return true
	case r >= 0xAC00 && r <= 0xD7A3:
		return true
	default:
		return false
	}
}

func buildIndexFiles(terms map[string][]uint64, dictPath, datPath, idxPath string) error {
	keys := make([]string, 0, len(terms))
	for k := range terms {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fdat, err := os.Create(datPath)
	if err != nil {
		return err
	}
	defer fdat.Close()
	w := bufio.NewWriterSize(fdat, 1<<20)
	type offrec struct {
		Off uint64
		Len uint32
	}
	offsets := make([]offrec, len(keys))
	dict := make(map[string]uint32, len(keys))
	var cur uint64
	for i, term := range keys {
		ids := terms[term]
		sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
		off := cur
		if _, err := writeUvarint(w, uint64(len(ids))); err != nil {
			return err
		}
		cur += uint64(varintLen(uint64(len(ids))))
		var last uint64
		for _, id := range ids {
			d := id - last
			n, err := writeUvarint(w, d)
			if err != nil {
				return err
			}
			cur += uint64(n)
			last = id
		}
		dict[term] = uint32(i + 1)
		offsets[i] = offrec{Off: off, Len: uint32(cur - off)}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fidx, err := os.Create(idxPath)
	if err != nil {
		return err
	}
	defer fidx.Close()
	bw := bufio.NewWriterSize(fidx, 1<<20)
	for _, r := range offsets {
		if err := binary.Write(bw, binary.LittleEndian, r.Off); err != nil {
			return err
		}
		if err := binary.Write(bw, binary.LittleEndian, r.Len); err != nil {
			return err
		}
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	bdict, _ := json.Marshal(dict)
	if err := os.WriteFile(dictPath, bdict, 0o644); err != nil {
		return err
	}
	return nil
}

func writeUvarint(w *bufio.Writer, x uint64) (int, error) {
	var buf [10]byte
	n := binary.PutUvarint(buf[:], x)
	_, err := w.Write(buf[:n])
	return n, err
}
func varintLen(x uint64) int {
	var buf [10]byte
	return binary.PutUvarint(buf[:], x)
}

func intersectSorted(a, b []uint64) []uint64 {
	i, j := 0, 0
	out := make([]uint64, 0, min(len(a), len(b)))
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			out = append(out, a[i])
			i++
			j++
			continue
		}
		if a[i] < b[j] {
			i++
		} else {
			j++
		}
	}
	return out
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// unionSorted merges two sorted unique uint64 slices into a sorted unique union
func unionSorted(a, b []uint64) []uint64 {
	out := make([]uint64, 0, len(a)+len(b))
	i, j := 0, 0
	var last uint64
	hasLast := false
	for i < len(a) && j < len(b) {
		var v uint64
		if a[i] == b[j] {
			v = a[i]
			i++
			j++
		} else if a[i] < b[j] {
			v = a[i]
			i++
		} else {
			v = b[j]
			j++
		}
		if !hasLast || v != last {
			out = append(out, v)
			last = v
			hasLast = true
		}
	}
	for i < len(a) {
		v := a[i]
		i++
		if !hasLast || v != last {
			out = append(out, v)
			last = v
			hasLast = true
		}
	}
	for j < len(b) {
		v := b[j]
		j++
		if !hasLast || v != last {
			out = append(out, v)
			last = v
			hasLast = true
		}
	}
	return out
}
