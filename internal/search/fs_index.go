package search

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/db"

	"github.com/cespare/xxhash/v2"
)

// FileID uniquely identifies a file across renames/moves.
type FileID uint64

// FileMeta is stored in Pebble as the source of truth.
type FileMeta struct {
	FileID         uint64 `json:"fileId"`
	Path           string `json:"path"`
	Name           string `json:"name"`
	Ext            string `json:"ext"`
	Size           uint64 `json:"size"`
	MTime          int64  `json:"mtime"`
	IsDir          bool   `json:"isDir"`
	ContentIndexed bool   `json:"contentIndexed"`
}

// Pebble key helpers
func keyFileMeta(id uint64) []byte   { return []byte(fmt.Sprintf("f:%d", id)) }
func keyPathToID(path string) []byte { return []byte("p:" + filepath.ToSlash(path)) }

var pebGet = func(key []byte) ([]byte, error) { return db.GetDefault().Get(key) }

// Index directory structure
var indexDirOverride string

func indexDir() string {
	if indexDirOverride != "" {
		return indexDirOverride
	}
	return filepath.Join(consts.DATA_DIR, "searchidx")
}

func namePostingsDat() string { return filepath.Join(indexDir(), "name.postings.dat") }
func namePostingsIdx() string { return filepath.Join(indexDir(), "name.postings.idx") }
func nameDictJSON() string    { return filepath.Join(indexDir(), "name.dict.json") }

func pathPostingsDat() string { return filepath.Join(indexDir(), "path.postings.dat") }
func pathPostingsIdx() string { return filepath.Join(indexDir(), "path.postings.idx") }
func pathDictJSON() string    { return filepath.Join(indexDir(), "path.dict.json") }

// Ngram index files for fuzzy search (ASCII 2-gram + CJK bigram)
func nameNgramPostingsDat() string { return filepath.Join(indexDir(), "name_ngram.postings.dat") }
func nameNgramPostingsIdx() string { return filepath.Join(indexDir(), "name_ngram.postings.idx") }
func nameNgramDictJSON() string    { return filepath.Join(indexDir(), "name_ngram.dict.json") }
func pathNgramPostingsDat() string { return filepath.Join(indexDir(), "path_ngram.postings.dat") }
func pathNgramPostingsIdx() string { return filepath.Join(indexDir(), "path_ngram.postings.idx") }
func pathNgramDictJSON() string    { return filepath.Join(indexDir(), "path_ngram.dict.json") }

// Filter index files (bitmap-style postings for ext/size/mtime)
func filterPostingsDat() string { return filepath.Join(indexDir(), "filter.postings.dat") }
func filterPostingsIdx() string { return filepath.Join(indexDir(), "filter.postings.idx") }
func filterDictJSON() string    { return filepath.Join(indexDir(), "filter.dict.json") }

// IndexExists reports whether the custom inverted index exists on disk.
func IndexExists() bool {
	_, e1 := os.Stat(namePostingsDat())
	_, e2 := os.Stat(namePostingsIdx())
	_, e3 := os.Stat(nameDictJSON())
	_, e4 := os.Stat(pathPostingsDat())
	_, e5 := os.Stat(pathPostingsIdx())
	_, e6 := os.Stat(pathDictJSON())
	// Require fuzzy ngram indexes too, so watcher rebuilds if they are missing
	_, e7 := os.Stat(nameNgramPostingsDat())
	_, e8 := os.Stat(nameNgramPostingsIdx())
	_, e9 := os.Stat(nameNgramDictJSON())
	_, e10 := os.Stat(pathNgramPostingsDat())
	_, e11 := os.Stat(pathNgramPostingsIdx())
	_, e12 := os.Stat(pathNgramDictJSON())
	return e1 == nil && e2 == nil && e3 == nil && e4 == nil && e5 == nil && e6 == nil && e7 == nil && e8 == nil && e9 == nil && e10 == nil && e11 == nil && e12 == nil
}

// FileID generation using xxhash64(dev:ino:ctime)
func genFileID(fi os.FileInfo) (uint64, error) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return 0, errors.New("unsupported stat")
	}
	s := fmt.Sprintf("%d:%d:%d", uint64(st.Dev), uint64(st.Ino), st.Ctim.Sec)
	return xxhash.Sum64String(s), nil
}

// postings builder in-memory
type termMap map[string][]uint64

// IndexPaths scans roots, writes Pebble metadata, and builds name/path inverted indexes.
func IndexPaths(roots []string, showHidden bool) error {
	if len(roots) == 0 {
		return nil
	}
	_ = os.MkdirAll(indexDir(), 0o755)
	names := make(termMap, 1<<16)
	paths := make(termMap, 1<<16)
	// Fuzzy term maps
	nameNgrams := make(termMap, 1<<16)
	pathNgrams := make(termMap, 1<<16)
	peb := db.GetDefault()

	for _, root := range roots {
		filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			// Always exclude PlainNAS trash trees from the search index.
			// These are implementation details and must never enter the normal file index.
			name := d.Name()
			if d.IsDir() {
				if name == ".nas-trash" {
					return filepath.SkipDir
				}
			}
			if strings.Contains(filepath.Clean(p), string(filepath.Separator)+".nas-trash"+string(filepath.Separator)) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			// Skip heavy/virtual pseudo filesystems
			if d.IsDir() {
				switch p {
				case "/proc", "/sys", "/dev", "/run", "/tmp", "/var/run", "/var/tmp":
					return filepath.SkipDir
				}
			}
			if !showHidden && strings.HasPrefix(name, ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			fi, e := os.Lstat(p)
			if e != nil {
				return nil
			}
			fid, e := genFileID(fi)
			if e != nil {
				return nil
			}
			meta := FileMeta{
				FileID: fid,
				Path:   filepath.ToSlash(p),
				Name:   name,
				Ext:    strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), "."),
				Size:   uint64(fi.Size()),
				MTime:  fi.ModTime().Unix(),
				IsDir:  fi.IsDir(),
			}
			// Pebble writes (source of truth)
			b, _ := json.Marshal(meta)
			_ = peb.Set(keyFileMeta(fid), b, nil)
			_ = peb.Set(keyPathToID(meta.Path), []byte(fmt.Sprintf("%d", fid)), nil)
			// exact tokens
			for _, t := range tokenize(name) {
				names[t] = append(names[t], fid)
			}
			for _, t := range tokenize(meta.Path) {
				paths[t] = append(paths[t], fid)
			}
			// ngram tokens for fuzzy search
			for _, ng := range buildQueryNgrams(name) {
				nameNgrams[ng] = append(nameNgrams[ng], fid)
			}
			for _, ng := range buildQueryNgrams(meta.Path) {
				pathNgrams[ng] = append(pathNgrams[ng], fid)
			}
			return nil
		})
	}

	// Build and persist indexes
	if err := buildIndexFiles(names, nameDictJSON(), namePostingsDat(), namePostingsIdx()); err != nil {
		return err
	}
	if err := buildIndexFiles(paths, pathDictJSON(), pathPostingsDat(), pathPostingsIdx()); err != nil {
		return err
	}
	// Build ngram fuzzy indexes
	if err := buildIndexFiles(nameNgrams, nameNgramDictJSON(), nameNgramPostingsDat(), nameNgramPostingsIdx()); err != nil {
		return err
	}
	if err := buildIndexFiles(pathNgrams, pathNgramDictJSON(), pathNgramPostingsDat(), pathNgramPostingsIdx()); err != nil {
		return err
	}
	// Build filter index from Pebble metadata
	if err := BuildFilterIndex(); err != nil {
		return err
	}
	return nil
}

// BuildFilterIndex scans Pebble FileMeta entries and writes filter postings for ext/size/mtime
func BuildFilterIndex() error {
	peb := db.GetDefault()
	// term -> docIDs for filters
	terms := make(termMap, 1<<14)
	// Iterate all file metas
	if err := peb.Iterate([]byte("f:"), func(key []byte, value []byte) error {
		var m FileMeta
		if err := json.Unmarshal(value, &m); err != nil {
			return nil
		}
		docID := m.FileID
		// ext
		if m.Ext != "" {
			terms["ext:"+m.Ext] = append(terms["ext:"+m.Ext], docID)
		}
		// size bucket
		terms["size:"+sizeBucket(m.Size)] = append(terms["size:"+sizeBucket(m.Size)], docID)
		// mtime bucket (month)
		terms["mtime:"+mtimeBucket(m.MTime)] = append(terms["mtime:"+mtimeBucket(m.MTime)], docID)
		return nil
	}); err != nil {
		return err
	}
	// Persist filter postings
	return buildIndexFiles(terms, filterDictJSON(), filterPostingsDat(), filterPostingsIdx())
}

func sizeBucket(sz uint64) string {
	switch {
	case sz < 1<<10: // <1KB
		return "s0"
	case sz < 1<<20: // <1MB
		return "s1"
	case sz < 10<<20: // <10MB
		return "s2"
	case sz < 100<<20: // <100MB
		return "s3"
	case sz < 1<<30: // <1GB
		return "s4"
	default:
		return "s5"
	}
}

func mtimeBucket(ts int64) string {
	// YYYYMM buckets
	if ts <= 0 {
		return "m0"
	}
	// Basic math to avoid time package overhead
	// Approximate months since epoch; fine for coarse filters
	// Use 30-day months
	days := ts / (24 * 3600)
	months := days / 30
	return fmt.Sprintf("m%d", months)
}

// buildIndexFiles sorts, delta-encodes, and writes postings + dictionary + offsets
func buildIndexFiles(terms termMap, dictPath, datPath, idxPath string) error {
	// Collect and sort terms
	keys := make([]string, 0, len(terms))
	for k := range terms {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Prepare files
	fdat, err := os.Create(datPath)
	if err != nil {
		return err
	}
	defer fdat.Close()
	w := bufio.NewWriterSize(fdat, 1<<20)

	// offsets array: for TermID starting at 1
	type offrec struct {
		Off uint64
		Len uint32
	}
	offsets := make([]offrec, len(keys))

	// Dictionary map term -> TermID
	dict := make(map[string]uint32, len(keys))

	// Write postings sequentially
	var cur uint64 = 0
	for i, term := range keys {
		ids := terms[term]
		sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
		// record offset
		off := cur
		// encode doc count
		if _, err := writeUvarint(w, uint64(len(ids))); err != nil {
			return err
		}
		cur += uint64(varintLen(uint64(len(ids))))
		// delta-encode ids
		var last uint64 = 0
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

	// Write offsets file
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

	// Write dictionary json
	bdict, _ := json.Marshal(dict)
	if err := os.WriteFile(dictPath, bdict, 0o644); err != nil {
		return err
	}
	return nil
}

// varint helpers
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

func slicePage(out []string, offset, limit int) []string {
	if offset >= len(out) {
		return []string{}
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end]
}

// tryAbsolutePath resolves an absolute path via filesystem only.
// Returns (results, true) when handled; (nil, false) when the path does not exist.
func tryAbsolutePath(q string, offset, limit int) ([]string, bool) {
	if !strings.HasPrefix(q, "/") {
		return nil, false
	}
	fsPath := filepath.FromSlash(q)
	st, err := os.Stat(fsPath)
	if err != nil {
		return nil, false
	}
	if !st.IsDir() {
		return slicePage([]string{filepath.ToSlash(fsPath)}, offset, limit), true
	}
	ents, err := os.ReadDir(fsPath)
	if err != nil {
		return []string{}, true
	}
	out := make([]string, 0, len(ents))
	for _, e := range ents {
		out = append(out, filepath.ToSlash(filepath.Join(fsPath, e.Name())))
	}
	return slicePage(out, offset, limit), true
}

// SearchIndex executes name/path token search with fuzzy fallback and optional parent prefix filter.
func SearchIndex(text string, parent string, offset int, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	text = strings.TrimSpace(text)
	isPathQuery := strings.Contains(text, "/")

	parent = filepath.ToSlash(parent)
	q := text
	boundary := ""
	dirPrefix := ""
	if isPathQuery {
		q = filepath.ToSlash(q)
		if parent != "" && !strings.HasPrefix(q, "/") {
			q = path.Join(parent, q)
		}
		q = path.Clean(q)
		boundary = q

		if out, ok := tryAbsolutePath(q, offset, limit); ok {
			return out, nil
		}

		if bID, _ := pebGet(keyPathToID(boundary)); bID != nil {
			if id, err := strconv.ParseUint(string(bID), 10, 64); err == nil {
				if b, _ := pebGet(keyFileMeta(id)); b != nil {
					var m FileMeta
					_ = json.Unmarshal(b, &m)
					if !m.IsDir {
						out := []string{m.Path}
						if offset >= len(out) {
							return []string{}, nil
						}
						end := offset + limit
						if end > len(out) {
							end = len(out)
						}
						return out[offset:end], nil
					}
					dirPrefix = m.Path
					if dirPrefix != "" && !strings.HasSuffix(dirPrefix, "/") {
						dirPrefix += "/"
					}
				}
			}
		}
	}

	// Open exact indexes based on mode
	var nm *mmapIndex
	var pm *mmapIndex
	if isPathQuery {
		var err error
		pm, err = openMmapIndex(pathDictJSON(), pathPostingsDat(), pathPostingsIdx())
		if err != nil {
			if pm != nil {
				pm.close()
			}
			return []string{}, nil
		}
		defer pm.close()
	} else {
		var err error
		nm, err = openMmapIndex(nameDictJSON(), namePostingsDat(), namePostingsIdx())
		if err != nil {
			if nm != nil {
				nm.close()
			}
			return []string{}, nil
		}
		defer nm.close()
	}

	// tokenize query terms
	queryForTokens := text
	if isPathQuery {
		queryForTokens = q
	}
	terms := tokenize(queryForTokens)
	// Collect exact union per term across fields and intersect starting from shortest
	var idsExact []uint64
	if len(terms) > 0 {
		type termSet struct{ ids []uint64 }
		sets := make([]termSet, 0, len(terms))
		for _, t := range terms {
			var union []uint64
			if isPathQuery {
				pid := pm.dict[t]
				ppl, _ := pm.posting(pid)
				union = unionSorted(union, ppl)
			} else {
				nid := nm.dict[t]
				npl, _ := nm.posting(nid)
				union = unionSorted(union, npl)
			}
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

	// Decide fuzzy enablement per rule
	enableFuzzy := true
	if len(idsExact) >= limit*3 {
		enableFuzzy = false
	}

	finalIDs := idsExact
	if enableFuzzy {
		// Open fuzzy ngram indexes if available; tolerate missing ones
		if isPathQuery {
			fpm, err := openMmapIndex(pathNgramDictJSON(), pathNgramPostingsDat(), pathNgramPostingsIdx())
			if err == nil {
				defer fpm.close()
				ngrams := buildQueryNgrams(q)
				if len(ngrams) > 0 {
					type termSet struct{ ids []uint64 }
					fsets := make([]termSet, 0, len(ngrams))
					for _, ng := range ngrams {
						pid := fpm.dict[ng]
						ppl, _ := fpm.postingCapped(pid, 20000)
						if len(ppl) > 0 {
							fsets = append(fsets, termSet{ids: ppl})
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
		} else {
			fnm, err := openMmapIndex(nameNgramDictJSON(), nameNgramPostingsDat(), nameNgramPostingsIdx())
			if err == nil {
				defer fnm.close()
				ngrams := buildQueryNgrams(text)
				if len(ngrams) > 0 {
					type termSet struct{ ids []uint64 }
					fsets := make([]termSet, 0, len(ngrams))
					for _, ng := range ngrams {
						nid := fnm.dict[ng]
						npl, _ := fnm.postingCapped(nid, 20000)
						if len(npl) > 0 {
							fsets = append(fsets, termSet{ids: npl})
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
	}

	// Map to paths via Pebble and apply parent prefix filter if provided
	out := make([]string, 0, len(finalIDs))
	for _, id := range finalIDs {
		b, _ := pebGet(keyFileMeta(id))
		if b == nil {
			continue
		}
		var m FileMeta
		_ = json.Unmarshal(b, &m)
		if isPathQuery {
			if dirPrefix != "" {
				if !strings.HasPrefix(m.Path, dirPrefix) {
					continue
				}
			} else if boundary != "" {
				if m.Path != boundary && !strings.HasPrefix(m.Path, boundary+"/") {
					continue
				}
			}
		} else if parent != "" {
			if !strings.HasPrefix(m.Path, parent) {
				continue
			}
		}
		out = append(out, m.Path)
	}
	// apply offset/limit on final IDs only
	return slicePage(out, offset, limit), nil
}
