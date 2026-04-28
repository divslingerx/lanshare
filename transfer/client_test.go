package transfer_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"filehub/transfer"
)

func TestPullFile(t *testing.T) {
	content := []byte("transferred content")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer ts.Close()

	destDir := t.TempDir()
	c := transfer.NewClient()
	err := c.PullFile(ts.URL+"/files/abc/hello.txt", filepath.Join(destDir, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(destDir, "hello.txt"))
	if string(got) != string(content) {
		t.Fatalf("content mismatch: got %q", got)
	}
}

func TestSincePull(t *testing.T) {
	fileContent := []byte("file content here")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/since" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"changes":[{"path":"subdir/file.txt","op":"write","mtime":1714299120,"size":42}]}`))
			return
		}
		w.Write(fileContent)
	}))
	defer ts.Close()

	destDir := t.TempDir()
	c := transfer.NewClient()
	pulled, err := c.SincePull(ts.URL, "folder1", "2020-01-01T00:00:00Z", destDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(pulled) != 1 {
		t.Fatalf("want 1 pulled, got %d", len(pulled))
	}
	// Verify the file was actually written
	got, err := os.ReadFile(filepath.Join(destDir, "subdir", "file.txt"))
	if err != nil {
		t.Fatalf("expected file to be written: %v", err)
	}
	if string(got) != string(fileContent) {
		t.Fatalf("content mismatch")
	}
}
