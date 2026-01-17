package db

import (
	"os"
	"testing"

	"ismartcoding/plainnas/internal/consts"
)

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "plainnas-db-test-*")
	if err != nil {
		panic(err)
	}
	consts.DATA_DIR = tmp
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

func TestTagCountUpdatesOnAddRemove(t *testing.T) {
	id, err := AddOrUpdateTag("", func(tag *Tag) {
		tag.Name = "t"
		tag.Type = 1
	})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}

	if err := SaveTagRelation(id, "k1"); err != nil {
		t.Fatalf("save relation: %v", err)
	}
	got, err := GetTagByID(id)
	if err != nil {
		t.Fatalf("get tag: %v", err)
	}
	if got.Count != 1 {
		t.Fatalf("count after add: got %d want 1", got.Count)
	}

	// Adding same relation again should remain 1.
	if err := SaveTagRelation(id, "k1"); err != nil {
		t.Fatalf("save relation again: %v", err)
	}
	got, _ = GetTagByID(id)
	if got.Count != 1 {
		t.Fatalf("count after duplicate add: got %d want 1", got.Count)
	}

	if err := DeleteTagRelationsByKeysAndTagID([]string{"k1"}, id); err != nil {
		t.Fatalf("delete relation: %v", err)
	}
	got, _ = GetTagByID(id)
	if got.Count != 0 {
		t.Fatalf("count after remove: got %d want 0", got.Count)
	}
}

func TestRebuildAllTagCounts(t *testing.T) {
	id1, err := AddOrUpdateTag("", func(tag *Tag) {
		tag.Name = "a"
		tag.Type = 1
		tag.Count = 999 // intentionally wrong
	})
	if err != nil {
		t.Fatalf("create tag1: %v", err)
	}
	id2, err := AddOrUpdateTag("", func(tag *Tag) {
		tag.Name = "b"
		tag.Type = 1
		tag.Count = 999 // intentionally wrong
	})
	if err != nil {
		t.Fatalf("create tag2: %v", err)
	}

	if err := SaveTagRelation(id1, "k1"); err != nil {
		t.Fatalf("save rel1: %v", err)
	}
	if err := SaveTagRelation(id1, "k2"); err != nil {
		t.Fatalf("save rel2: %v", err)
	}
	if err := SaveTagRelation(id2, "k3"); err != nil {
		t.Fatalf("save rel3: %v", err)
	}

	// Break counts again to ensure rebuild fixes them.
	_, _ = AddOrUpdateTag(id1, func(tag *Tag) { tag.Count = 123 })
	_, _ = AddOrUpdateTag(id2, func(tag *Tag) { tag.Count = 456 })

	if err := RebuildAllTagCounts(); err != nil {
		t.Fatalf("rebuild counts: %v", err)
	}
	got1, _ := GetTagByID(id1)
	got2, _ := GetTagByID(id2)
	if got1.Count != 2 {
		t.Fatalf("tag1 count: got %d want 2", got1.Count)
	}
	if got2.Count != 1 {
		t.Fatalf("tag2 count: got %d want 1", got2.Count)
	}
}
