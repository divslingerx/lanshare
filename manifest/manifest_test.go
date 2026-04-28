package manifest_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"filehub/manifest"
)

func TestWalk(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0o644)

	m, err := manifest.Walk("folder1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 2 {
		t.Fatalf("want 2 files, got %d", len(m.Files))
	}
	e, ok := m.Files["a.txt"]
	if !ok {
		t.Fatal("a.txt missing")
	}
	if e.Size != 5 {
		t.Fatalf("want size 5, got %d", e.Size)
	}
}

func TestChanged(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	os.WriteFile(p, []byte("v1"), 0o644)

	m, _ := manifest.Walk("x", dir)
	entry := m.Files["f.txt"]

	info, _ := os.Stat(p)
	if manifest.Changed(entry, info) {
		t.Fatal("unchanged file should not be detected as changed")
	}

	time.Sleep(20 * time.Millisecond)
	os.WriteFile(p, []byte("v2-longer"), 0o644)
	info2, _ := os.Stat(p)
	if !manifest.Changed(entry, info2) {
		t.Fatal("modified file should be detected as changed")
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.txt"), []byte("data"), 0o644)
	m, _ := manifest.Walk("abc12345", dir)

	savePath := filepath.Join(t.TempDir(), "abc12345.json")
	if err := m.Save(savePath); err != nil {
		t.Fatal(err)
	}
	m2, err := manifest.Load(savePath)
	if err != nil {
		t.Fatal(err)
	}
	if m2 == nil {
		t.Fatal("loaded nil manifest")
	}
	if m2.FolderID != "abc12345" {
		t.Fatalf("want abc12345, got %s", m2.FolderID)
	}
	if len(m2.Files) != 1 {
		t.Fatalf("want 1, got %d", len(m2.Files))
	}
}

func TestLoadMissingReturnsNil(t *testing.T) {
	m, err := manifest.Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatal("want nil for missing file")
	}
}
