package helpers

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCountDirEntriesFast_Small(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		p := filepath.Join(dir, fmt.Sprintf("f%02d", i))
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	count, fuzzy, err := countDirEntriesFast(dir, 10)
	if err != nil {
		t.Fatalf("countDirEntriesFast: %v", err)
	}
	if fuzzy {
		t.Fatalf("expected fuzzy=false")
	}
	if count != 3 {
		t.Fatalf("expected count=3, got %d", count)
	}
}

func TestCountDirEntriesFast_Threshold(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 11; i++ {
		p := filepath.Join(dir, fmt.Sprintf("f%02d", i))
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	count, fuzzy, err := countDirEntriesFast(dir, 10)
	if err != nil {
		t.Fatalf("countDirEntriesFast: %v", err)
	}
	if !fuzzy {
		t.Fatalf("expected fuzzy=true")
	}
	if count != 10 {
		t.Fatalf("expected count=10, got %d", count)
	}
}
