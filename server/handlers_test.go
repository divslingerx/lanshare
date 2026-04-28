package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"filehub/config"
	"filehub/server"
)

func newTestServer(t *testing.T) *server.Server {
	t.Helper()
	cfg := config.Default()
	cfg.DeviceID = "test-host"
	cfg.DisplayName = "Test Machine"
	return server.New(cfg)
}

func TestPeersEndpoint(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/peers", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var resp server.PeersResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.DeviceID != "test-host" {
		t.Fatalf("want test-host, got %s", resp.DeviceID)
	}
}

func TestFileEndpoint(t *testing.T) {
	s := newTestServer(t)
	dir := t.TempDir()
	content := []byte("hello world")
	os.WriteFile(filepath.Join(dir, "hello.txt"), content, 0o644)

	fid := config.FolderID(dir)
	s.AddFolder(config.Folder{ID: fid, Path: dir, Mode: config.ModeShared})

	req := httptest.NewRequest("GET", "/files/"+fid+"/hello.txt", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body)
	}
	got, _ := io.ReadAll(w.Body)
	if string(got) != string(content) {
		t.Fatalf("content mismatch")
	}
}

func TestSinceEndpoint(t *testing.T) {
	s := newTestServer(t)
	dir := t.TempDir()
	fid := config.FolderID(dir)
	s.AddFolder(config.Folder{ID: fid, Path: dir, Mode: config.ModeWatch})

	// Simulate a file that changed recently
	s.SetFileState(fid, map[string]config.Change{
		"readme.txt": {Path: "readme.txt", Op: config.OpWrite, MTime: time.Now().Unix(), Size: 10},
	})

	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/since?folder="+fid+"&t="+past, nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var resp server.SinceResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Changes) != 1 {
		t.Fatalf("want 1 change, got %d", len(resp.Changes))
	}
}
