package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMakeUniquePathIfExists_Dir(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "Videos")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got, err := makeUniquePathIfExists(p, false)
	if err != nil {
		t.Fatalf("makeUniquePathIfExists: %v", err)
	}
	want := filepath.Join(tmp, "Videos (1)")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMakeUniquePathIfExists_File(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "movie.mp4")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := makeUniquePathIfExists(p, true)
	if err != nil {
		t.Fatalf("makeUniquePathIfExists: %v", err)
	}
	want := filepath.Join(tmp, "movie (1).mp4")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	// If (1) exists, next should be (2)
	if err := os.WriteFile(want, []byte("y"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got2, err := makeUniquePathIfExists(p, true)
	if err != nil {
		t.Fatalf("makeUniquePathIfExists: %v", err)
	}
	want2 := filepath.Join(tmp, "movie (2).mp4")
	if got2 != want2 {
		t.Fatalf("got %q want %q", got2, want2)
	}
}
