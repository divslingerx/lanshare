package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestBrowseEndpoint(t *testing.T) {
	t.Run("shared folder returns 200 with entries", func(t *testing.T) {
		s := newTestServer(t)
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "data.txt"), []byte("content"), 0o644)

		fid := config.FolderID(dir)
		s.AddFolder(config.Folder{ID: fid, Path: dir, Mode: config.ModeShared})

		req := httptest.NewRequest("GET", "/browse/"+fid+"/", nil)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", w.Code, w.Body)
		}
		var resp server.BrowseResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Entries) != 1 {
			t.Fatalf("want 1 entry, got %d", len(resp.Entries))
		}
		if resp.Entries[0].Name != "data.txt" {
			t.Fatalf("want entry name data.txt, got %s", resp.Entries[0].Name)
		}
	})

	t.Run("watch folder returns 403", func(t *testing.T) {
		s := newTestServer(t)
		dir := t.TempDir()

		fid := config.FolderID(dir)
		s.AddFolder(config.Folder{ID: fid, Path: dir, Mode: config.ModeWatch})

		req := httptest.NewRequest("GET", "/browse/"+fid+"/", nil)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("want 403, got %d", w.Code)
		}
	})
}

func TestEventsEndpoint(t *testing.T) {
	s := newTestServer(t)
	dir := t.TempDir()
	fid := config.FolderID(dir)
	s.AddFolder(config.Folder{ID: fid, Path: dir, Mode: config.ModeWatch})

	pr, pw := io.Pipe()
	w := &pipeResponseWriter{pw: pw, header: make(http.Header), code: http.StatusOK}
	req, cancel := makeStreamRequest("/events/" + fid)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.ServeHTTP(w, req)
	}()

	readWithTimeout := func(label string) (string, error) {
		ch := make(chan string, 1)
		errCh := make(chan error, 1)
		go func() {
			buf := make([]byte, 256)
			n, err := pr.Read(buf)
			if err != nil {
				errCh <- err
				return
			}
			ch <- string(buf[:n])
		}()
		select {
		case s := <-ch:
			return s, nil
		case err := <-errCh:
			return "", err
		case <-time.After(2 * time.Second):
			return "", fmt.Errorf("timeout waiting for %s", label)
		}
	}

	// Read the connected ping
	got, err := readWithTimeout("connected ping")
	if err != nil {
		t.Fatalf("reading connected ping: %v", err)
	}
	if !strings.Contains(got, ": connected") {
		t.Fatalf("expected connected ping, got %q", got)
	}

	// Broadcast a change and read it
	s.BroadcastChange(config.ChangeEvent{FolderID: fid, Changes: []config.Change{{Path: "a.txt", Op: config.OpWrite}}})
	got, err = readWithTimeout("broadcast")
	if err != nil {
		t.Fatalf("reading broadcast: %v", err)
	}
	if !strings.Contains(got, "a.txt") {
		t.Fatalf("expected a.txt in broadcast, got %q", got)
	}

	cancel()
	<-done
}

// pipeResponseWriter implements http.ResponseWriter + http.Flusher via an io.PipeWriter.
type pipeResponseWriter struct {
	pw     *io.PipeWriter
	header http.Header
	code   int
}

func (p *pipeResponseWriter) Header() http.Header         { return p.header }
func (p *pipeResponseWriter) WriteHeader(code int)        { p.code = code }
func (p *pipeResponseWriter) Write(b []byte) (int, error) { return p.pw.Write(b) }
func (p *pipeResponseWriter) Flush()                      {}

func makeStreamRequest(path string) (*http.Request, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", path, nil).WithContext(ctx)
	return req, cancel
}
