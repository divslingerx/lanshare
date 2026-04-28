package watcher_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"filehub/config"
	"filehub/manifest"
	"filehub/watcher"
)

func TestWatcherDetectsWrite(t *testing.T) {
	dir := t.TempDir()
	m := manifest.New("test1")

	var mu sync.Mutex
	var gotChanges []config.Change
	done := make(chan struct{})
	once := sync.Once{}

	w, err := watcher.New(func(folderID string, changes []config.Change) {
		once.Do(func() {
			mu.Lock()
			gotChanges = changes
			mu.Unlock()
			close(done)
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Watch("test1", dir, m); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)
	os.WriteFile(filepath.Join(dir, "newfile.txt"), []byte("hello"), 0o644)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: no change event received")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(gotChanges) == 0 {
		t.Fatal("expected at least one change")
	}
	if gotChanges[0].Op != config.OpWrite {
		t.Fatalf("expected write op, got %s", gotChanges[0].Op)
	}
}
