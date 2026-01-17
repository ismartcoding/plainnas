package search

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

type memPebble struct {
	m map[string][]byte
}

func (p *memPebble) get(key []byte) ([]byte, error) {
	v, ok := p.m[string(key)]
	if !ok {
		return nil, nil
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (p *memPebble) setPathToID(path string, id uint64) {
	p.m[string(keyPathToID(path))] = []byte(strconv.FormatUint(id, 10))
}

func (p *memPebble) setMeta(m FileMeta) {
	b, _ := json.Marshal(m)
	p.m[string(keyFileMeta(m.FileID))] = b
}

func buildTestIndexes(t *testing.T, tmpDir string, metas []FileMeta) {
	t.Helper()
	indexDirOverride = tmpDir

	names := make(termMap, 16)
	paths := make(termMap, 16)
	for _, m := range metas {
		for _, tok := range tokenize(m.Name) {
			names[tok] = append(names[tok], m.FileID)
		}
		for _, tok := range tokenize(m.Path) {
			paths[tok] = append(paths[tok], m.FileID)
		}
	}

	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := buildIndexFiles(names, nameDictJSON(), namePostingsDat(), namePostingsIdx()); err != nil {
		t.Fatalf("build name index: %v", err)
	}
	if err := buildIndexFiles(paths, pathDictJSON(), pathPostingsDat(), pathPostingsIdx()); err != nil {
		t.Fatalf("build path index: %v", err)
	}
}

func TestSearchIndex_KeywordDoesNotMatchPathSegments(t *testing.T) {
	tmpDir := t.TempDir()
	p := &memPebble{m: map[string][]byte{}}
	oldGet := pebGet
	oldIdx := indexDirOverride
	defer func() {
		pebGet = oldGet
		indexDirOverride = oldIdx
	}()
	pebGet = p.get

	metas := []FileMeta{{FileID: 1, Path: "/test/a/b.jpg", Name: "b.jpg", IsDir: false}}
	p.setPathToID(metas[0].Path, metas[0].FileID)
	p.setMeta(metas[0])
	buildTestIndexes(t, filepath.Join(tmpDir, "searchidx"), metas)

	got, err := SearchIndex("a", "", 0, 50)
	if err != nil {
		t.Fatalf("SearchIndex: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestSearchIndex_KeywordMatchesBasename(t *testing.T) {
	tmpDir := t.TempDir()
	p := &memPebble{m: map[string][]byte{}}
	oldGet := pebGet
	oldIdx := indexDirOverride
	defer func() {
		pebGet = oldGet
		indexDirOverride = oldIdx
	}()
	pebGet = p.get

	metas := []FileMeta{{FileID: 1, Path: "/test/a/b.jpg", Name: "b.jpg", IsDir: false}}
	p.setPathToID(metas[0].Path, metas[0].FileID)
	p.setMeta(metas[0])
	buildTestIndexes(t, filepath.Join(tmpDir, "searchidx"), metas)

	got, err := SearchIndex("b", "", 0, 50)
	if err != nil {
		t.Fatalf("SearchIndex: %v", err)
	}
	if len(got) != 1 || got[0] != metas[0].Path {
		t.Fatalf("expected %q, got %v", metas[0].Path, got)
	}
}

func TestSearchIndex_PathExactFile(t *testing.T) {
	tmpDir := t.TempDir()
	p := &memPebble{m: map[string][]byte{}}
	oldGet := pebGet
	oldIdx := indexDirOverride
	defer func() {
		pebGet = oldGet
		indexDirOverride = oldIdx
	}()
	pebGet = p.get

	metas := []FileMeta{{FileID: 10, Path: "/DATA/Videos/IMG_1048.mp4", Name: "IMG_1048.mp4", IsDir: false}}
	p.setPathToID(metas[0].Path, metas[0].FileID)
	p.setMeta(metas[0])
	buildTestIndexes(t, filepath.Join(tmpDir, "searchidx"), metas)

	got, err := SearchIndex("/DATA/Videos/IMG_1048.mp4", "", 0, 50)
	if err != nil {
		t.Fatalf("SearchIndex: %v", err)
	}
	if len(got) != 1 || got[0] != metas[0].Path {
		t.Fatalf("expected %q, got %v", metas[0].Path, got)
	}
}

func TestSearchIndex_PathDirListsDescendants(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "IMG_1048.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatalf("writefile: %v", err)
	}

	got, err := SearchIndex(filepath.ToSlash(root), "", 0, 50)
	if err != nil {
		t.Fatalf("SearchIndex: %v", err)
	}
	wantFile := filepath.ToSlash(filepath.Join(root, "IMG_1048.mp4"))
	wantSubDir := filepath.ToSlash(filepath.Join(root, "Sub"))
	forbiddenNested := filepath.ToSlash(filepath.Join(root, "Sub", "x.txt"))

	if !contains(got, wantFile) {
		t.Fatalf("missing expected file %q, got %v", wantFile, got)
	}
	if !contains(got, wantSubDir) {
		t.Fatalf("missing expected subdir %q, got %v", wantSubDir, got)
	}
	if contains(got, forbiddenNested) {
		t.Fatalf("did not expect nested file %q in %v", forbiddenNested, got)
	}
}

func TestSearchIndex_PathAbsoluteFile_NoIndex(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatalf("writefile: %v", err)
	}

	got, err := SearchIndex(filepath.ToSlash(f), "", 0, 10)
	if err != nil {
		t.Fatalf("SearchIndex: %v", err)
	}
	if len(got) != 1 || got[0] != filepath.ToSlash(f) {
		t.Fatalf("expected %q, got %v", filepath.ToSlash(f), got)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
