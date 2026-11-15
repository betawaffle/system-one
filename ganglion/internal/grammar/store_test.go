package grammar

import (
	"os"
	"path/filepath"
	"testing"
)

type typ struct {
	Name   string `json:"name"`
	Parent string `json:"parent"`
}

func TestVersions(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := Meta{By: "test"}
	v1, err := s.Put("type:book", "type", typ{"book", "artifact"}, m)
	if err != nil {
		t.Fatal(err)
	}
	if v1.Why != "create" || v1.Parent != "" {
		t.Fatalf("first version: %+v", v1)
	}
	again, _ := s.Put("type:book", "type", typ{"book", "artifact"}, m)
	if again.Hash != v1.Hash || len(s.History("type:book")) != 1 {
		t.Fatal("same content should be a no-op")
	}
	v2, _ := s.Put("type:book", "type", typ{"book", "physical"}, m)
	if v2.Parent != v1.Hash || v2.Why != "edit" {
		t.Fatalf("second version: %+v", v2)
	}
	if _, err := s.Put("frame:book", "type", typ{}, m); err == nil {
		t.Fatal("id/kind mismatch should fail")
	}

	// the id isn't in the hashed content (07/O2a): same content, same hash
	other, _ := s.Put("type:tome", "type", typ{"book", "physical"}, m)
	if other.Hash != v2.Hash {
		t.Fatal("identical content should share a hash")
	}

	snap1, _ := s.TakeSnapshot("base0")
	if err := s.Retire("type:tome", m); err != nil {
		t.Fatal(err)
	}
	snap2, _ := s.TakeSnapshot("base0")
	if snap1 == snap2 {
		t.Fatal("retiring should change the snapshot")
	}

	// reopening rebuilds the same heads from the files alone
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got typ
	if _, err := r.Get("type:book", &got); err != nil || got.Parent != "physical" {
		t.Fatalf("reopened head: %+v %v", got, err)
	}
	if _, ok := r.Head("type:tome"); ok {
		t.Fatal("retired id should have no head")
	}
	if snap, _ := r.TakeSnapshot("base0"); snap != snap2 {
		t.Fatal("snapshot should be reproducible after reopen")
	}
}

func TestTornLine(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	s.Put("type:cup", "type", typ{"cup", "artifact"}, Meta{By: "test"})
	f, _ := os.OpenFile(filepath.Join(dir, "versions.jsonl"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"schema":1,"id":"type:x"`) // crash mid-append
	f.Close()
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Head("type:cup"); !ok {
		t.Fatal("good lines should survive a torn last line")
	}
	r.Put("type:box", "type", typ{"box", "artifact"}, Meta{By: "test"})
	r2, _ := Open(dir)
	if _, ok := r2.Head("type:box"); !ok {
		t.Fatal("a write after a torn line should land on its own line")
	}
}

func TestEvents(t *testing.T) {
	s, _ := Open(t.TempDir())
	id, err := s.AddEvent("sentence", "test", "ganglion/write", map[string]any{"text": "Mara gave Tom a book.", "how": "typed"})
	if err != nil || len(id) != 16 {
		t.Fatalf("event: %q %v", id, err)
	}
}
