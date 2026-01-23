package search

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

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

// SearchIndex executes name/path token search with fuzzy fallback, optional parent prefix filter, and optional file size filter.
// If sizeOp is non-empty, applies file size filtering using the filter index.
func SearchIndex(text string, parent string, offset int, limit int, sizeOp string, sizeBytes uint64) ([]string, error) {
	limit, offset, text, isPathQuery, parent, q, boundary, dirPrefix := searchIndexPreprocess(text, parent, offset, limit)

	if isPathQuery {
		if out, ok := tryAbsolutePath(q, offset, limit); ok {
			return out, nil
		}
		dirPrefix = searchIndexMaybeSetDirPrefix(boundary, &dirPrefix, sizeOp, sizeBytes, offset, limit)
	}

	// Get size filter IDs from filter index if size operator is specified
	var sizeFilterIDs []uint64
	if sizeOp != "" {
		sizeFilterIDs = getSizeFilterIDs(sizeOp, sizeBytes)
		// If no text query, return size filter results directly
		if text == "" {
			out := mapIDsToPathsWithFilters(sizeFilterIDs, isPathQuery, dirPrefix, boundary, parent, sizeOp, sizeBytes)
			return slicePage(out, offset, limit), nil
		}
	}

	// Execute text search
	nm, pm, err := searchIndexOpenIndexes(isPathQuery)
	if err != nil {
		return []string{}, nil
	}
	defer func() {
		if nm != nil {
			nm.close()
		}
		if pm != nil {
			pm.close()
		}
	}()

	terms := searchIndexTokenizeTerms(text, q, isPathQuery)
	idsExact := searchIndexCollectIDs(terms, isPathQuery, nm, pm)
	finalIDs := searchIndexMaybeFuzzy(idsExact, isPathQuery, limit, q, text)

	// Intersect with size filter IDs if size filter is specified
	if len(sizeFilterIDs) > 0 {
		finalIDs = intersectSorted(finalIDs, sizeFilterIDs)
	}

	// Map IDs to paths and apply filters
	out := mapIDsToPathsWithFilters(finalIDs, isPathQuery, dirPrefix, boundary, parent, sizeOp, sizeBytes)
	return slicePage(out, offset, limit), nil
}

// getSizeFilterIDs retrieves file IDs from filter index that match the size criteria
func getSizeFilterIDs(sizeOp string, sizeBytes uint64) []uint64 {
	fm, err := openMmapIndex(filterDictJSON(), filterPostingsDat(), filterPostingsIdx())
	if err != nil {
		return nil
	}
	defer fm.close()

	buckets := getSizeBuckets(sizeOp, sizeBytes)
	var result []uint64
	for _, bucket := range buckets {
		termID := fm.dict["size:"+bucket]
		if termID > 0 {
			postings, _ := fm.posting(termID)
			result = unionSorted(result, postings)
		}
	}
	return result
}

// getSizeBuckets returns the size buckets that should be queried for the given operator and size
// Since buckets are coarse-grained ranges, we need to include buckets that might contain matching files
func getSizeBuckets(op string, sizeBytes uint64) []string {
	allBuckets := []string{"s0", "s1", "s2", "s3", "s4", "s5"}
	bucket := sizeBucket(sizeBytes)

	switch op {
	case ">", ">=":
		// Include current bucket and all larger buckets
		// Current bucket is included because it may contain larger files
		var result []string
		found := false
		for _, b := range allBuckets {
			if b == bucket {
				found = true
			}
			if found {
				result = append(result, b)
			}
		}
		return result

	case "<", "<=":
		// Include current bucket and all smaller buckets
		// Current bucket is included because it may contain smaller files
		var result []string
		for _, b := range allBuckets {
			result = append(result, b)
			if b == bucket {
				break
			}
		}
		return result

	case "=":
		return []string{bucket}

	case "!=":
		// Return all buckets, precise filtering will be done later
		return allBuckets

	default:
		return allBuckets
	}
}

// mapIDsToPathsWithFilters maps file IDs to paths and applies path prefix and size filters
func mapIDsToPathsWithFilters(finalIDs []uint64, isPathQuery bool, dirPrefix, boundary, parent string, sizeOp string, sizeBytes uint64) []string {
	out := make([]string, 0, len(finalIDs))
	for _, id := range finalIDs {
		b, _ := pebGet(keyFileMeta(id))
		if b == nil {
			continue
		}
		var m FileMeta
		_ = json.Unmarshal(b, &m)

		// Skip directories when size filter is active
		if sizeOp != "" && m.IsDir {
			continue
		}

		// Apply path prefix filters
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

		// Apply precise size filtering since buckets are coarse-grained
		if sizeOp != "" && !fileSizeMatches(m.Size, sizeOp, sizeBytes) {
			continue
		}

		out = append(out, m.Path)
	}
	return out
}

// --- SearchIndex helpers ---
func searchIndexPreprocess(text, parent string, offset, limit int) (int, int, string, bool, string, string, string, string) {
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
	}
	return limit, offset, text, isPathQuery, parent, q, boundary, dirPrefix
}

func searchIndexMaybeSetDirPrefix(boundary string, dirPrefix *string, sizeOp string, sizeBytes uint64, offset, limit int) string {
	bID, _ := pebGet(keyPathToID(boundary))
	if bID == nil {
		return *dirPrefix
	}
	id, err := strconv.ParseUint(string(bID), 10, 64)
	if err != nil {
		return *dirPrefix
	}
	b, _ := pebGet(keyFileMeta(id))
	if b == nil {
		return *dirPrefix
	}
	var m FileMeta
	_ = json.Unmarshal(b, &m)
	if !m.IsDir {
		// Exact file match case - will be handled by caller
		// Just return empty dirPrefix to signal no directory traversal needed
		return ""
	}
	*dirPrefix = m.Path
	if *dirPrefix != "" && !strings.HasSuffix(*dirPrefix, "/") {
		*dirPrefix += "/"
	}
	return *dirPrefix
}

func searchIndexOpenIndexes(isPathQuery bool) (nm, pm *mmapIndex, err error) {
	if isPathQuery {
		pm, err = openMmapIndex(pathDictJSON(), pathPostingsDat(), pathPostingsIdx())
		return nil, pm, err
	}
	nm, err = openMmapIndex(nameDictJSON(), namePostingsDat(), namePostingsIdx())
	return nm, nil, err
}

func searchIndexTokenizeTerms(text, q string, isPathQuery bool) []string {
	if isPathQuery {
		return tokenize(q)
	}
	return tokenize(text)
}

func searchIndexCollectIDs(terms []string, isPathQuery bool, nm, pm *mmapIndex) []uint64 {
	if len(terms) == 0 {
		return nil
	}
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
	if len(sets) == 0 {
		return nil
	}
	idsExact := sets[0].ids
	for i := 1; i < len(sets); i++ {
		idsExact = intersectSorted(idsExact, sets[i].ids)
		if len(idsExact) == 0 {
			break
		}
	}
	return idsExact
}

func searchIndexMaybeFuzzy(idsExact []uint64, isPathQuery bool, limit int, q, text string) []uint64 {
	if len(idsExact) >= limit*3 {
		return idsExact
	}
	finalIDs := idsExact
	if isPathQuery {
		fpm, err := openMmapIndex(pathNgramDictJSON(), pathNgramPostingsDat(), pathNgramPostingsIdx())
		if err != nil {
			return finalIDs
		}
		defer fpm.close()
		ngrams := buildQueryNgrams(q)
		if len(ngrams) == 0 {
			return finalIDs
		}
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
		if len(fsets) == 0 {
			return finalIDs
		}
		idsFuzzy := fsets[0].ids
		for i := 1; i < len(fsets); i++ {
			idsFuzzy = intersectSorted(idsFuzzy, fsets[i].ids)
			if len(idsFuzzy) == 0 {
				break
			}
		}
		finalIDs = unionSorted(finalIDs, idsFuzzy)
		return finalIDs
	}
	fnm, err := openMmapIndex(nameNgramDictJSON(), nameNgramPostingsDat(), nameNgramPostingsIdx())
	if err != nil {
		return finalIDs
	}
	defer fnm.close()
	ngrams := buildQueryNgrams(text)
	if len(ngrams) == 0 {
		return finalIDs
	}
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
	if len(fsets) == 0 {
		return finalIDs
	}
	idsFuzzy := fsets[0].ids
	for i := 1; i < len(fsets); i++ {
		idsFuzzy = intersectSorted(idsFuzzy, fsets[i].ids)
		if len(idsFuzzy) == 0 {
			break
		}
	}
	finalIDs = unionSorted(finalIDs, idsFuzzy)
	return finalIDs
}

// fileSizeMatches returns true if the file size matches the operator and sizeBytes
func fileSizeMatches(fileSize uint64, op string, sizeBytes uint64) bool {
	switch op {
	case ">":
		return fileSize > sizeBytes
	case ">=":
		return fileSize >= sizeBytes
	case "<":
		return fileSize < sizeBytes
	case "<=":
		return fileSize <= sizeBytes
	case "=":
		return fileSize == sizeBytes
	case "!=":
		return fileSize != sizeBytes
	default:
		return true
	}
}
