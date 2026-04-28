# filehub Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a cross-platform LAN file sharing desktop app with folder watch/subscribe, SSE-based change events, mDNS peer discovery, and a React/Wails frontend.

**Architecture:** Each instance runs a Go HTTP server on port 47990. mDNS handles peer discovery. Watched folders broadcast change events over SSE; subscribers pull changed files via HTTP GET. Config and manifests persist in the OS config dir. React frontend communicates exclusively through Wails bindings.

**Tech Stack:** Go 1.23, Wails v2.12, React 18, Vite 3, `fsnotify`, `grandcat/zeroconf`, `cespare/xxhash/v2`

---

## File Structure

```
config/
  config.go           — Config, Folder, Subscription, Change, ChangeEvent types; load/save; FolderID()
  config_test.go
manifest/
  manifest.go         — FileEntry, Manifest; Walk; Update; Changed; Load/Save; ManifestPath()
  manifest_test.go
server/
  sse.go              — Hub: Subscribe/Unsubscribe/Broadcast
  sse_test.go
  server.go           — Server struct; routing; Start/Stop; AddFolder; SetFileState; BroadcastChange
  handlers.go         — handlePeers, handleBrowse, handleFile, handleSince, handleEvents
  handlers_test.go
watcher/
  watcher.go          — Watcher: Watch/Unwatch; fsnotify + 500ms debounce; onChange callback
  watcher_test.go
discovery/
  mdns.go             — Peer struct; Advertiser; Browser
  mdns_test.go
transfer/
  client.go           — Client: PullFile; SincePull
  client_test.go
app.go                — App struct (Wails); wire all packages; startup sequence; all bound methods
frontend/src/
  App.jsx             — 3-panel shell + view router
  App.css             — design tokens + global styles
  components/
    Sidebar.jsx       — nav, peer list, self-identity footer
    FolderList.jsx    — folder cards + mode toggles
    FolderCard.jsx    — single card with toggle and remove
    AddFolderModal.jsx— path input + mode selector
    ActivityPanel.jsx — active transfers + history
    NetworkView.jsx   — peer folder browser + subscribe
    SettingsView.jsx  — display name, port, base storage
  hooks/
    useAppState.js    — Wails binding wrappers + event listeners
```

---

### Task 1: Config system

**Files:** `config/config.go`, `config/config_test.go`

- [ ] **Add xxhash dependency**

```bash
go get github.com/cespare/xxhash/v2
```

Expected: `go.mod` gains `github.com/cespare/xxhash/v2`.

- [ ] **Write failing tests** — create `config/config_test.go`:

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"filehub/config"
)

func TestFolderID(t *testing.T) {
	id := config.FolderID("/home/user/projects")
	if len(id) != 8 {
		t.Fatalf("want 8 chars, got %d: %q", len(id), id)
	}
	if config.FolderID("/home/user/projects") != id {
		t.Fatal("not deterministic")
	}
	if config.FolderID("/home/user/other") == id {
		t.Fatal("collision on different paths")
	}
}

func TestDefault(t *testing.T) {
	cfg := config.Default()
	if cfg.Port != 47990 {
		t.Fatalf("want 47990, got %d", cfg.Port)
	}
	if cfg.DeviceID == "" || cfg.DisplayName == "" {
		t.Fatal("DeviceID and DisplayName must not be empty")
	}
}

func TestLoadSaveRoundtrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Default()
	cfg.DisplayName = "mybox"
	cfg.Port = 9999
	if err := config.Save(cfg, p); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "mybox" || got.Port != 9999 {
		t.Fatalf("roundtrip failed: %+v", got)
	}
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 47990 {
		t.Fatalf("want default port, got %d", cfg.Port)
	}
}
```

- [ ] **Run — expect failure**

```bash
go test ./config/...
```

Expected: `cannot find package "filehub/config"`

- [ ] **Implement `config/config.go`**

```go
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cespare/xxhash/v2"
)

type FolderMode string

const (
	ModeWatch  FolderMode = "watch"
	ModeShared FolderMode = "shared"
)

type Folder struct {
	ID   string     `json:"id"`
	Path string     `json:"path"`
	Mode FolderMode `json:"mode"`
}

type Subscription struct {
	PeerHostname string    `json:"peer_hostname"`
	RemoteFolder string    `json:"remote_folder"` // display path on the remote peer
	FolderID     string    `json:"folder_id"`     // remote folder's ID (for HTTP routes)
	LocalDest    string    `json:"local_dest"`    // "" = use default ~/filehub/{peer}/{folder}/
	LastSyncedAt time.Time `json:"last_synced_at"`
}

type Change struct {
	Path  string `json:"path"`
	Op    string `json:"op"` // "write" | "delete"
	MTime int64  `json:"mtime"`
	Size  int64  `json:"size"`
}

type ChangeEvent struct {
	FolderID string   `json:"folder_id"`
	Changes  []Change `json:"changes"`
}

type Config struct {
	DeviceID      string         `json:"device_id"`
	DisplayName   string         `json:"display_name"`
	Port          int            `json:"port"`
	BaseStorage   string         `json:"base_storage"`
	Folders       []Folder       `json:"folders"`
	Subscriptions []Subscription `json:"subscriptions"`
}

func FolderID(absPath string) string {
	h := xxhash.Sum64([]byte(absPath))
	return fmt.Sprintf("%016x", h)[:8]
}

func Default() Config {
	hostname, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	return Config{
		DeviceID:      hostname,
		DisplayName:   hostname,
		Port:          47990,
		BaseStorage:   filepath.Join(home, "filehub"),
		Folders:       []Folder{},
		Subscriptions: []Subscription{},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	return cfg, json.Unmarshal(data, &cfg)
}

func Save(cfg Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "filehub", "config.json"), nil
}
```

- [ ] **Run — expect pass**

```bash
go test ./config/...
```

Expected: `ok  filehub/config`

- [ ] **Commit**

```bash
git add config/
git commit -m "feat: config types, load/save, FolderID generation"
```

---

### Task 2: File manifest

**Files:** `manifest/manifest.go`, `manifest/manifest_test.go`

- [ ] **Write failing tests** — create `manifest/manifest_test.go`:

```go
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
```

- [ ] **Run — expect failure**

```bash
go test ./manifest/...
```

Expected: `cannot find package "filehub/manifest"`

- [ ] **Implement `manifest/manifest.go`**

```go
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cespare/xxhash/v2"
)

type FileEntry struct {
	MTime  int64  `json:"mtime"`
	Size   int64  `json:"size"`
	XXHash string `json:"xxhash"`
}

type Manifest struct {
	FolderID string               `json:"folder_id"`
	Files    map[string]FileEntry `json:"files"`
}

func New(folderID string) *Manifest {
	return &Manifest{FolderID: folderID, Files: make(map[string]FileEntry)}
}

// Changed returns true when mtime or size differs from the stored entry.
func Changed(entry FileEntry, info os.FileInfo) bool {
	return entry.MTime != info.ModTime().Unix() || entry.Size != info.Size()
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := xxhash.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%016x", h.Sum64()), nil
}

// Walk builds a Manifest by stat-walking root. Symlinks are skipped.
func Walk(folderID, root string) (*Manifest, error) {
	m := New(folderID)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		hash, err := hashFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		m.Files[rel] = FileEntry{
			MTime:  info.ModTime().Unix(),
			Size:   info.Size(),
			XXHash: hash,
		}
		return nil
	})
	return m, err
}

// Update refreshes one file's entry; deletes it if the file is gone.
func (m *Manifest) Update(relPath, absPath string) error {
	info, err := os.Stat(absPath)
	if errors.Is(err, os.ErrNotExist) {
		delete(m.Files, relPath)
		return nil
	}
	if err != nil {
		return err
	}
	if entry, ok := m.Files[relPath]; ok && !Changed(entry, info) {
		return nil // fast path: unchanged
	}
	hash, err := hashFile(absPath)
	if err != nil {
		return err
	}
	m.Files[relPath] = FileEntry{
		MTime:  info.ModTime().Unix(),
		Size:   info.Size(),
		XXHash: hash,
	}
	return nil
}

func (m *Manifest) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Files == nil {
		m.Files = make(map[string]FileEntry)
	}
	return &m, nil
}

// ManifestPath returns the canonical path for a manifest in the config dir.
func ManifestPath(configDir, folderID string) string {
	return filepath.Join(configDir, "filehub", "manifests", folderID+".json")
}
```

- [ ] **Run — expect pass**

```bash
go test ./manifest/...
```

Expected: `ok  filehub/manifest`

- [ ] **Commit**

```bash
git add manifest/
git commit -m "feat: file manifest with Walk, xxhash change detection, load/save"
```

---

### Task 3: SSE hub

**Files:** `server/sse.go`, `server/sse_test.go`

- [ ] **Write failing tests** — create `server/sse_test.go`:

```go
package server_test

import (
	"testing"
	"time"
	"filehub/server"
)

func TestHubBroadcastReceived(t *testing.T) {
	hub := server.NewHub()
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	msg := []byte(`{"folder_id":"abc","changes":[]}`)
	hub.Broadcast(msg)

	select {
	case got := <-ch:
		if string(got) != string(msg) {
			t.Fatalf("want %s, got %s", msg, got)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout: message not received")
	}
}

func TestHubUnsubscribeClosesChannel(t *testing.T) {
	hub := server.NewHub()
	ch := hub.Subscribe()
	hub.Unsubscribe(ch)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected closed channel")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("channel not closed")
	}
}

func TestHubBroadcastToMultiple(t *testing.T) {
	hub := server.NewHub()
	ch1 := hub.Subscribe()
	ch2 := hub.Subscribe()
	defer hub.Unsubscribe(ch1)
	defer hub.Unsubscribe(ch2)

	hub.Broadcast([]byte("hello"))

	for _, ch := range []chan []byte{ch1, ch2} {
		select {
		case got := <-ch:
			if string(got) != "hello" {
				t.Fatalf("want hello, got %s", got)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout")
		}
	}
}
```

- [ ] **Run — expect failure**

```bash
go test ./server/...
```

Expected: `cannot find package "filehub/server"`

- [ ] **Implement `server/sse.go`**

```go
package server

import "sync"

// Hub manages SSE subscribers for one watched folder.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: make(map[chan []byte]struct{})}
}

func (h *Hub) Subscribe() chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.subscribers, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *Hub) Broadcast(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers {
		select {
		case ch <- data:
		default: // drop if subscriber buffer is full
		}
	}
}
```

- [ ] **Run — expect pass**

```bash
go test ./server/...
```

Expected: `ok  filehub/server`

- [ ] **Commit**

```bash
git add server/sse.go server/sse_test.go
git commit -m "feat: SSE hub for broadcasting change events"
```

---

### Task 4: HTTP server and endpoints

**Files:** `server/server.go`, `server/handlers.go`, `server/handlers_test.go`

- [ ] **Write failing tests** — create `server/handlers_test.go`:

```go
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
		"readme.txt": {Path: "readme.txt", Op: "write", MTime: time.Now().Unix(), Size: 10},
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
```

- [ ] **Run — expect failure**

```bash
go test ./server/...
```

Expected: undefined: `server.New`, `server.PeersResponse`, etc.

- [ ] **Implement `server/server.go`**

```go
package server

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"filehub/config"
)

type PeersResponse struct {
	DeviceID    string          `json:"device_id"`
	DisplayName string          `json:"display_name"`
	Folders     []config.Folder `json:"folders"`
}

type SinceResponse struct {
	Changes []config.Change `json:"changes"`
}

type BrowseEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
}

type BrowseResponse struct {
	Entries []BrowseEntry `json:"entries"`
}

type Server struct {
	mu         sync.RWMutex
	cfg        config.Config
	mux        *http.ServeMux
	hubs       map[string]*Hub                   // folderID -> Hub
	folders    map[string]config.Folder           // folderID -> Folder
	fileState  map[string]map[string]config.Change // folderID -> relPath -> Change
	httpServer *http.Server
}

func New(cfg config.Config) *Server {
	s := &Server{
		cfg:       cfg,
		mux:       http.NewServeMux(),
		hubs:      make(map[string]*Hub),
		folders:   make(map[string]config.Folder),
		fileState: make(map[string]map[string]config.Change),
	}
	s.mux.HandleFunc("GET /peers", s.handlePeers)
	s.mux.HandleFunc("GET /browse/{folder_id}/", s.handleBrowse)
	s.mux.HandleFunc("GET /files/{folder_id}/{path...}", s.handleFile)
	s.mux.HandleFunc("GET /since", s.handleSince)
	s.mux.HandleFunc("GET /events/{folder_id}", s.handleEvents)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) Start(port int) error {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	s.httpServer = &http.Server{
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // no timeout: SSE streams are long-lived
	}
	go s.httpServer.Serve(ln)
	return nil
}

func (s *Server) Stop() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
}

func (s *Server) AddFolder(f config.Folder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.folders[f.ID] = f
	if f.Mode == config.ModeWatch {
		s.hubs[f.ID] = NewHub()
	}
}

func (s *Server) RemoveFolder(folderID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.folders, folderID)
	delete(s.hubs, folderID)
	delete(s.fileState, folderID)
}

func (s *Server) SetFileState(folderID string, state map[string]config.Change) {
	s.mu.Lock()
	s.fileState[folderID] = state
	s.mu.Unlock()
}

func (s *Server) BroadcastChange(event config.ChangeEvent) {
	s.mu.RLock()
	hub := s.hubs[event.FolderID]
	s.mu.RUnlock()
	if hub == nil {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	hub.Broadcast(data)
}

func (s *Server) UpdateConfig(cfg config.Config) {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
}
```

- [ ] **Implement `server/handlers.go`**

```go
package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"filehub/config"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := s.cfg
	folders := make([]config.Folder, 0, len(s.folders))
	for _, f := range s.folders {
		folders = append(folders, f)
	}
	s.mu.RUnlock()
	writeJSON(w, PeersResponse{
		DeviceID:    cfg.DeviceID,
		DisplayName: cfg.DisplayName,
		Folders:     folders,
	})
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	folderID := r.PathValue("folder_id")
	s.mu.RLock()
	folder, ok := s.folders[folderID]
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	entries, err := os.ReadDir(folder.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := BrowseResponse{}
	for _, e := range entries {
		info, _ := e.Info()
		if info == nil {
			continue
		}
		resp.Entries = append(resp.Entries, BrowseEntry{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  info.Size(),
			MTime: info.ModTime().Unix(),
		})
	}
	writeJSON(w, resp)
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	folderID := r.PathValue("folder_id")
	relPath := r.PathValue("path")

	s.mu.RLock()
	folder, ok := s.folders[folderID]
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Prevent path traversal
	clean := path.Clean("/" + relPath)
	absPath := filepath.Join(folder.Path, filepath.FromSlash(clean))
	if !strings.HasPrefix(absPath, folder.Path+string(filepath.Separator)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, absPath)
}

func (s *Server) handleSince(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder")
	tStr := r.URL.Query().Get("t")
	if folderID == "" || tStr == "" {
		http.Error(w, "missing folder or t", http.StatusBadRequest)
		return
	}
	since, err := time.Parse(time.RFC3339, tStr)
	if err != nil {
		http.Error(w, "invalid t", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	state := s.fileState[folderID]
	s.mu.RUnlock()

	var changes []config.Change
	for _, c := range state {
		if c.MTime > since.Unix() {
			changes = append(changes, c)
		}
	}
	if changes == nil {
		changes = []config.Change{}
	}
	writeJSON(w, SinceResponse{Changes: changes})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	folderID := r.PathValue("folder_id")
	s.mu.RLock()
	hub := s.hubs[folderID]
	s.mu.RUnlock()
	if hub == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	// Send a comment to establish the connection
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

// scanDirEntries is a helper used only in handleBrowse for subdirectories.
func scanDirEntries(root string) ([]BrowseEntry, error) {
	var out []BrowseEntry
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == root {
			return err
		}
		info, _ := d.Info()
		if info == nil {
			return nil
		}
		out = append(out, BrowseEntry{
			Name:  d.Name(),
			IsDir: d.IsDir(),
			Size:  info.Size(),
			MTime: info.ModTime().Unix(),
		})
		if d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	return out, err
}
```

- [ ] **Run — expect pass**

```bash
go test ./server/...
```

Expected: `ok  filehub/server`

- [ ] **Commit**

```bash
git add server/
git commit -m "feat: HTTP server with peers, browse, files, since, and SSE endpoints"
```

---

### Task 5: File watcher

**Files:** `watcher/watcher.go`, `watcher/watcher_test.go`

- [ ] **Add fsnotify dependency**

```bash
go get github.com/fsnotify/fsnotify
```

- [ ] **Write failing tests** — create `watcher/watcher_test.go`:

```go
package watcher_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"filehub/config"
	"filehub/manifest"
	"filehub/watcher"
)

func TestWatcherDetectsWrite(t *testing.T) {
	dir := t.TempDir()
	m := manifest.New("test1")

	var gotChanges []config.Change
	done := make(chan struct{})

	w, err := watcher.New(func(folderID string, changes []config.Change) {
		gotChanges = changes
		close(done)
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

	if len(gotChanges) == 0 {
		t.Fatal("expected at least one change")
	}
	if gotChanges[0].Op != "write" {
		t.Fatalf("expected write op, got %s", gotChanges[0].Op)
	}
}
```

- [ ] **Run — expect failure**

```bash
go test ./watcher/...
```

Expected: `cannot find package "filehub/watcher"`

- [ ] **Implement `watcher/watcher.go`**

```go
package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"filehub/config"
	"filehub/manifest"
)

type Watcher struct {
	fsw      *fsnotify.Watcher
	onChange func(folderID string, changes []config.Change)
	roots    map[string]string         // folderID -> absPath
	manifests map[string]*manifest.Manifest
	pending  map[string]map[string]fsnotify.Op // folderID -> relPath -> op
	timer    *time.Timer
	mu       sync.Mutex
	delay    time.Duration
}

func New(onChange func(folderID string, changes []config.Change)) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		fsw:       fsw,
		onChange:  onChange,
		roots:     make(map[string]string),
		manifests: make(map[string]*manifest.Manifest),
		pending:   make(map[string]map[string]fsnotify.Op),
		delay:     500 * time.Millisecond,
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) Watch(folderID, root string, m *manifest.Manifest) error {
	w.mu.Lock()
	w.roots[folderID] = root
	w.manifests[folderID] = m
	w.mu.Unlock()
	return w.fsw.Add(root)
}

func (w *Watcher) Unwatch(folderID string) {
	w.mu.Lock()
	root := w.roots[folderID]
	delete(w.roots, folderID)
	delete(w.manifests, folderID)
	delete(w.pending, folderID)
	w.mu.Unlock()
	if root != "" {
		w.fsw.Remove(root)
	}
}

func (w *Watcher) Close() {
	w.fsw.Close()
}

func (w *Watcher) loop() {
	for {
		select {
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	if event.Op&(fsnotify.Chmod) != 0 {
		return // ignore permission-only changes
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	var folderID, root string
	for fid, r := range w.roots {
		if len(event.Name) >= len(r) && event.Name[:len(r)] == r {
			folderID, root = fid, r
			break
		}
	}
	if folderID == "" {
		return
	}

	rel, err := filepath.Rel(root, event.Name)
	if err != nil {
		return
	}
	if rel == "." {
		return
	}

	if w.pending[folderID] == nil {
		w.pending[folderID] = make(map[string]fsnotify.Op)
	}
	w.pending[folderID][rel] = event.Op

	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(w.delay, w.fire)
}

func (w *Watcher) fire() {
	w.mu.Lock()
	batch := w.pending
	w.pending = make(map[string]map[string]fsnotify.Op)
	roots := make(map[string]string, len(w.roots))
	for k, v := range w.roots {
		roots[k] = v
	}
	manifests := make(map[string]*manifest.Manifest, len(w.manifests))
	for k, v := range w.manifests {
		manifests[k] = v
	}
	w.mu.Unlock()

	for folderID, files := range batch {
		root := roots[folderID]
		m := manifests[folderID]
		var changes []config.Change
		for rel, op := range files {
			abs := filepath.Join(root, rel)
			if op&fsnotify.Remove != 0 {
				if m != nil {
					delete(m.Files, rel)
				}
				changes = append(changes, config.Change{
					Path: filepath.ToSlash(rel),
					Op:   "delete",
				})
				continue
			}
			info, err := os.Stat(abs)
			if err != nil {
				continue
			}
			if m != nil {
				m.Update(rel, abs)
			}
			changes = append(changes, config.Change{
				Path:  filepath.ToSlash(rel),
				Op:    "write",
				MTime: info.ModTime().Unix(),
				Size:  info.Size(),
			})
		}
		if len(changes) > 0 {
			w.onChange(folderID, changes)
		}
	}
}
```

- [ ] **Run — expect pass**

```bash
go test ./watcher/...
```

Expected: `ok  filehub/watcher`

- [ ] **Commit**

```bash
git add watcher/
git commit -m "feat: file watcher with fsnotify and 500ms debounce"
```

---

### Task 6: mDNS discovery

**Files:** `discovery/mdns.go`, `discovery/mdns_test.go`

- [ ] **Add zeroconf dependency**

```bash
go get github.com/grandcat/zeroconf
```

- [ ] **Write tests** — create `discovery/mdns_test.go`:

```go
package discovery_test

import (
	"testing"
	"filehub/discovery"
)

func TestPeerString(t *testing.T) {
	p := discovery.Peer{Hostname: "myhost", DisplayName: "My Host", Addr: "192.168.1.5", Port: 47990}
	if p.Hostname == "" {
		t.Fatal("empty hostname")
	}
	if p.Addr == "" {
		t.Fatal("empty addr")
	}
}

func TestNewBrowser(t *testing.T) {
	b := discovery.NewBrowser(func(p discovery.Peer) {}, func(hostname string) {})
	if b == nil {
		t.Fatal("expected non-nil browser")
	}
}
```

- [ ] **Run — expect failure**

```bash
go test ./discovery/...
```

Expected: `cannot find package "filehub/discovery"`

- [ ] **Implement `discovery/mdns.go`**

```go
package discovery

import (
	"context"
	"log"
	"net"
	"strings"

	"github.com/grandcat/zeroconf"
)

const serviceType = "_filehub._tcp"
const domain = "local."

type Peer struct {
	Hostname    string
	DisplayName string
	Addr        string
	Port        int
}

// Advertiser announces this device on the LAN.
type Advertiser struct {
	server *zeroconf.Server
}

func NewAdvertiser(hostname, displayName string, port int) (*Advertiser, error) {
	txt := []string{
		"hostname=" + hostname,
		"v=1",
	}
	srv, err := zeroconf.Register(displayName, serviceType, domain, port, txt, nil)
	if err != nil {
		return nil, err
	}
	return &Advertiser{server: srv}, nil
}

func (a *Advertiser) Stop() {
	a.server.Shutdown()
}

// Browser discovers filehub peers on the LAN.
type Browser struct {
	cancel context.CancelFunc
}

func NewBrowser(onFound func(Peer), onLost func(hostname string)) *Browser {
	ctx, cancel := context.WithCancel(context.Background())
	b := &Browser{cancel: cancel}
	go b.run(ctx, onFound, onLost)
	return b
}

func (b *Browser) Stop() { b.cancel() }

func (b *Browser) run(ctx context.Context, onFound func(Peer), onLost func(string)) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Printf("mdns: resolver error: %v", err)
		return
	}
	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		if err := resolver.Browse(ctx, serviceType, domain, entries); err != nil {
			log.Printf("mdns: browse error: %v", err)
		}
	}()

	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				return
			}
			if entry == nil {
				continue
			}
			p := entryToPeer(entry)
			if p.Addr == "" {
				continue
			}
			onFound(p)
		case <-ctx.Done():
			return
		}
	}
}

func entryToPeer(e *zeroconf.ServiceEntry) Peer {
	hostname := e.HostName
	displayName := e.Instance
	for _, txt := range e.Text {
		if strings.HasPrefix(txt, "hostname=") {
			hostname = strings.TrimPrefix(txt, "hostname=")
		}
	}
	var addr string
	if len(e.AddrIPv4) > 0 {
		addr = e.AddrIPv4[0].String()
	} else if len(e.AddrIPv6) > 0 {
		addr = e.AddrIPv6[0].String()
	}
	_ = net.IP{} // ensure net imported
	return Peer{
		Hostname:    hostname,
		DisplayName: displayName,
		Addr:        addr,
		Port:        e.Port,
	}
}
```

- [ ] **Run — expect pass**

```bash
go test ./discovery/...
```

Expected: `ok  filehub/discovery`

- [ ] **Commit**

```bash
git add discovery/
git commit -m "feat: mDNS advertiser and browser for LAN peer discovery"
```

---

### Task 7: Transfer client

**Files:** `transfer/client.go`, `transfer/client_test.go`

- [ ] **Write failing tests** — create `transfer/client_test.go`:

```go
package transfer_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"filehub/config"
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
	changes := []config.Change{
		{Path: "subdir/file.txt", Op: "write", MTime: 1714299120, Size: 42},
	}
	fileContent := []byte("file content here")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/since" {
			w.Header().Set("Content-Type", "application/json")
			import_json := `{"changes":[{"path":"subdir/file.txt","op":"write","mtime":1714299120,"size":42}]}`
			w.Write([]byte(import_json))
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
	if len(pulled) != len(changes) {
		t.Fatalf("want %d pulled, got %d", len(changes), len(pulled))
	}
}
```

- [ ] **Run — expect failure**

```bash
go test ./transfer/...
```

Expected: `cannot find package "filehub/transfer"`

- [ ] **Implement `transfer/client.go`**

```go
package transfer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"filehub/config"
)

type sinceResponse struct {
	Changes []config.Change `json:"changes"`
}

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// PullFile downloads a single file from url and writes it to destPath.
func (c *Client) PullFile(url, destPath string) error {
	resp, err := c.http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pull %s: status %d", url, resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	// Write to temp file then rename to avoid partial writes.
	tmp := destPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	return os.Rename(tmp, destPath)
}

// SincePull calls /since on baseURL, then pulls each changed file into destDir.
// Returns the list of changes that were successfully pulled.
func (c *Client) SincePull(baseURL, folderID, since, destDir string) ([]config.Change, error) {
	url := fmt.Sprintf("%s/since?folder=%s&t=%s", baseURL, folderID, since)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("since %s: status %d", url, resp.StatusCode)
	}

	var sr sinceResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	var pulled []config.Change
	for _, ch := range sr.Changes {
		if ch.Op == "delete" {
			os.Remove(filepath.Join(destDir, filepath.FromSlash(ch.Path)))
			pulled = append(pulled, ch)
			continue
		}
		fileURL := fmt.Sprintf("%s/files/%s/%s", baseURL, folderID, ch.Path)
		dest := filepath.Join(destDir, filepath.FromSlash(ch.Path))
		if err := c.PullFile(fileURL, dest); err != nil {
			continue // log and skip; will retry on next sync
		}
		pulled = append(pulled, ch)
	}
	return pulled, nil
}
```

- [ ] **Run — expect pass**

```bash
go test ./transfer/...
```

Expected: `ok  filehub/transfer`

- [ ] **Commit**

```bash
git add transfer/
git commit -m "feat: transfer client with PullFile and SincePull"
```

---

### Task 8: App bindings and startup

**Files:** `app.go` (replace existing)

No unit tests for this task — it wires packages together and is validated via `wails dev` in the next task.

- [ ] **Replace `app.go`**

```go
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"filehub/config"
	"filehub/discovery"
	"filehub/manifest"
	"filehub/server"
	"filehub/transfer"
	"filehub/watcher"
)

type App struct {
	ctx        context.Context
	cfg        config.Config
	cfgPath    string
	srv        *server.Server
	wtch       *watcher.Watcher
	browser    *discovery.Browser
	advertiser *discovery.Advertiser
	client     *transfer.Client
	peers      map[string]discovery.Peer // hostname -> Peer
	manifests  map[string]*manifest.Manifest
	mu         sync.RWMutex
}

func NewApp() *App {
	return &App{
		peers:     make(map[string]discovery.Peer),
		manifests: make(map[string]*manifest.Manifest),
		client:    transfer.NewClient(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfgPath, err := config.ConfigPath()
	if err != nil {
		log.Fatalf("config path: %v", err)
	}
	a.cfgPath = cfgPath

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	a.cfg = cfg

	a.srv = server.New(cfg)
	for _, f := range cfg.Folders {
		a.srv.AddFolder(f)
	}

	a.wtch, err = watcher.New(a.onFolderChange)
	if err != nil {
		log.Fatalf("watcher: %v", err)
	}

	configDir, _ := os.UserConfigDir()
	for _, f := range cfg.Folders {
		if f.Mode != config.ModeWatch {
			continue
		}
		m, _ := manifest.Load(manifest.ManifestPath(configDir, f.ID))
		if m == nil {
			m = manifest.New(f.ID)
		}
		a.manifests[f.ID] = m
		a.wtch.Watch(f.ID, f.Path, m)
	}

	if err := a.srv.Start(cfg.Port); err != nil {
		wailsRuntime.MessageDialog(ctx, wailsRuntime.MessageDialogOptions{
			Type:    wailsRuntime.ErrorDialog,
			Title:   "Port conflict",
			Message: fmt.Sprintf("Cannot listen on port %d. Change it in Settings.", cfg.Port),
		})
	}

	a.advertiser, _ = discovery.NewAdvertiser(cfg.DeviceID, cfg.DisplayName, cfg.Port)

	a.browser = discovery.NewBrowser(
		func(p discovery.Peer) {
			if p.Hostname == cfg.DeviceID {
				return // skip self
			}
			a.mu.Lock()
			a.peers[p.Hostname] = p
			a.mu.Unlock()
			wailsRuntime.EventsEmit(ctx, "peer:online", p)
			go a.catchUp(p)
			go a.connectSSE(p)
		},
		func(hostname string) {
			a.mu.Lock()
			delete(a.peers, hostname)
			a.mu.Unlock()
			wailsRuntime.EventsEmit(ctx, "peer:offline", hostname)
		},
	)

	go a.startupScan()
}

func (a *App) shutdown(ctx context.Context) {
	if a.srv != nil {
		a.srv.Stop()
	}
	if a.browser != nil {
		a.browser.Stop()
	}
	if a.advertiser != nil {
		a.advertiser.Stop()
	}
	if a.wtch != nil {
		a.wtch.Close()
	}
}

func (a *App) onFolderChange(folderID string, changes []config.Change) {
	// Update server file state for /since queries
	a.mu.RLock()
	m := a.manifests[folderID]
	a.mu.RUnlock()

	if m != nil {
		state := make(map[string]config.Change, len(m.Files))
		for rel, entry := range m.Files {
			state[rel] = config.Change{
				Path:  rel,
				Op:    "write",
				MTime: entry.MTime,
				Size:  entry.Size,
			}
		}
		a.srv.SetFileState(folderID, state)

		configDir, _ := os.UserConfigDir()
		m.Save(manifest.ManifestPath(configDir, folderID))
	}

	event := config.ChangeEvent{FolderID: folderID, Changes: changes}
	a.srv.BroadcastChange(event)
	wailsRuntime.EventsEmit(a.ctx, "folder:changed", event)
}

// connectSSE opens a persistent SSE connection to peer p for each subscription and
// pulls files as change events arrive.
func (a *App) connectSSE(p discovery.Peer) {
	a.mu.RLock()
	subs := make([]config.Subscription, len(a.cfg.Subscriptions))
	copy(subs, a.cfg.Subscriptions)
	a.mu.RUnlock()

	baseURL := fmt.Sprintf("http://%s:%d", p.Addr, p.Port)
	for _, sub := range subs {
		if sub.PeerHostname != p.Hostname {
			continue
		}
		go func(sub config.Subscription) {
			resp, err := http.Get(baseURL + "/events/" + sub.FolderID)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			dest := sub.LocalDest
			if dest == "" {
				dest = filepath.Join(a.cfg.BaseStorage, sub.PeerHostname, filepath.Base(sub.RemoteFolder))
			}

			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				var event config.ChangeEvent
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
					continue
				}
				for _, ch := range event.Changes {
					if ch.Op == "delete" {
						os.Remove(filepath.Join(dest, filepath.FromSlash(ch.Path)))
						continue
					}
					fileURL := fmt.Sprintf("%s/files/%s/%s", baseURL, sub.FolderID, ch.Path)
					a.client.PullFile(fileURL, filepath.Join(dest, filepath.FromSlash(ch.Path)))
				}
				a.mu.Lock()
				for i, s := range a.cfg.Subscriptions {
					if s.PeerHostname == sub.PeerHostname && s.FolderID == sub.FolderID {
						a.cfg.Subscriptions[i].LastSyncedAt = time.Now()
					}
				}
				a.mu.Unlock()
				a.saveConfig()
			}
		}(sub)
	}
}

func (a *App) catchUp(p discovery.Peer) {
	a.mu.RLock()
	subs := make([]config.Subscription, len(a.cfg.Subscriptions))
	copy(subs, a.cfg.Subscriptions)
	a.mu.RUnlock()

	baseURL := fmt.Sprintf("http://%s:%d", p.Addr, p.Port)
	for i, sub := range subs {
		if sub.PeerHostname != p.Hostname {
			continue
		}
		dest := sub.LocalDest
		if dest == "" {
			dest = filepath.Join(a.cfg.BaseStorage, sub.PeerHostname, filepath.Base(sub.RemoteFolder))
		}
		pulled, err := a.client.SincePull(baseURL, sub.FolderID, sub.LastSyncedAt.Format(time.RFC3339), dest)
		if err != nil {
			log.Printf("catch-up %s/%s: %v", p.Hostname, sub.FolderID, err)
			continue
		}
		if len(pulled) > 0 {
			a.mu.Lock()
			a.cfg.Subscriptions[i].LastSyncedAt = time.Now()
			a.mu.Unlock()
			a.saveConfig()
		}
		wailsRuntime.EventsEmit(a.ctx, "subscription:synced", sub.FolderID)
	}
}

func (a *App) startupScan() {
	configDir, _ := os.UserConfigDir()
	a.mu.RLock()
	folders := make([]config.Folder, len(a.cfg.Folders))
	copy(folders, a.cfg.Folders)
	a.mu.RUnlock()

	for _, f := range folders {
		if f.Mode != config.ModeWatch {
			continue
		}
		m, _ := manifest.Walk(f.ID, f.Path)
		if m == nil {
			continue
		}
		saved, _ := manifest.Load(manifest.ManifestPath(configDir, f.ID))
		var offlineChanges int
		if saved != nil {
			for rel, entry := range m.Files {
				if old, ok := saved.Files[rel]; !ok || old.XXHash != entry.XXHash {
					offlineChanges++
				}
			}
		}
		a.mu.Lock()
		a.manifests[f.ID] = m
		a.mu.Unlock()
		m.Save(manifest.ManifestPath(configDir, f.ID))

		if offlineChanges > 0 {
			wailsRuntime.EventsEmit(a.ctx, "folder:offlineChanges", map[string]any{
				"folderID": f.ID,
				"count":    offlineChanges,
			})
		}
	}
}

func (a *App) saveConfig() {
	a.mu.RLock()
	cfg := a.cfg
	path := a.cfgPath
	a.mu.RUnlock()
	if err := config.Save(cfg, path); err != nil {
		log.Printf("save config: %v", err)
	}
}

// ── Wails-bound methods ────────────────────────────────

func (a *App) GetConfig() config.Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

func (a *App) GetFolders() []config.Folder {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.Folders
}

func (a *App) AddFolder(path string, mode string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	id := config.FolderID(absPath)
	f := config.Folder{ID: id, Path: absPath, Mode: config.FolderMode(mode)}

	a.mu.Lock()
	for _, existing := range a.cfg.Folders {
		if existing.ID == id {
			a.mu.Unlock()
			return fmt.Errorf("folder already added")
		}
	}
	a.cfg.Folders = append(a.cfg.Folders, f)
	a.mu.Unlock()

	a.srv.AddFolder(f)
	if f.Mode == config.ModeWatch {
		m := manifest.New(id)
		a.mu.Lock()
		a.manifests[id] = m
		a.mu.Unlock()
		a.wtch.Watch(id, absPath, m)
	}
	a.saveConfig()
	return nil
}

func (a *App) RemoveFolder(folderID string) error {
	a.mu.Lock()
	folders := a.cfg.Folders[:0]
	for _, f := range a.cfg.Folders {
		if f.ID != folderID {
			folders = append(folders, f)
		}
	}
	a.cfg.Folders = folders
	delete(a.manifests, folderID)
	a.mu.Unlock()

	a.srv.RemoveFolder(folderID)
	a.wtch.Unwatch(folderID)
	a.saveConfig()
	return nil
}

func (a *App) SetFolderMode(folderID string, mode string) error {
	a.mu.Lock()
	for i, f := range a.cfg.Folders {
		if f.ID == folderID {
			a.cfg.Folders[i].Mode = config.FolderMode(mode)
			break
		}
	}
	a.mu.Unlock()
	a.saveConfig()
	return nil
}

func (a *App) GetPeers() []discovery.Peer {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]discovery.Peer, 0, len(a.peers))
	for _, p := range a.peers {
		out = append(out, p)
	}
	return out
}

func (a *App) GetSubscriptions() []config.Subscription {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.Subscriptions
}

func (a *App) Subscribe(peerHostname, folderID, localDest string) error {
	a.mu.Lock()
	for _, s := range a.cfg.Subscriptions {
		if s.PeerHostname == peerHostname && s.FolderID == folderID {
			a.mu.Unlock()
			return fmt.Errorf("already subscribed")
		}
	}
	a.cfg.Subscriptions = append(a.cfg.Subscriptions, config.Subscription{
		PeerHostname: peerHostname,
		FolderID:     folderID,
		LocalDest:    localDest,
		LastSyncedAt: time.Time{},
	})
	a.mu.Unlock()
	a.saveConfig()
	return nil
}

func (a *App) Unsubscribe(peerHostname, folderID string) error {
	a.mu.Lock()
	subs := a.cfg.Subscriptions[:0]
	for _, s := range a.cfg.Subscriptions {
		if s.PeerHostname != peerHostname || s.FolderID != folderID {
			subs = append(subs, s)
		}
	}
	a.cfg.Subscriptions = subs
	a.mu.Unlock()
	a.saveConfig()
	return nil
}

func (a *App) UpdateDisplayName(name string) error {
	a.mu.Lock()
	a.cfg.DisplayName = name
	a.mu.Unlock()
	a.srv.UpdateConfig(a.cfg)
	a.saveConfig()
	return nil
}

func (a *App) UpdatePort(port int) error {
	a.mu.Lock()
	a.cfg.Port = port
	a.mu.Unlock()
	a.saveConfig()
	return nil
}

func (a *App) UpdateBaseStorage(path string) error {
	a.mu.Lock()
	a.cfg.BaseStorage = path
	a.mu.Unlock()
	a.saveConfig()
	return nil
}

func (a *App) OpenFolderDialog() (string, error) {
	return wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Folder",
	})
}
```

- [ ] **Verify it compiles**

```bash
go build ./...
```

Expected: no errors. If there are import errors, run `go mod tidy`.

- [ ] **Commit**

```bash
git add app.go
git commit -m "feat: App wiring with startup sequence, all Wails bindings"
```

---

### Task 9: Frontend base layout and design system

**Files:** `frontend/src/App.jsx`, `frontend/src/App.css`

- [ ] **Install no new dependencies** — Vite + React are already in place. Verify:

```bash
cd frontend && pnpm install
```

- [ ] **Replace `frontend/src/App.css`** with the design system from the mockup:

```css
/* @import must be first — before any rules */
@import url('https://fonts.googleapis.com/css2?family=Syne:wght@600;700;800&family=DM+Mono:wght@400;500&family=DM+Sans:wght@400;500;600&display=swap');

*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  --bg:         #09090d;
  --surface:    #111217;
  --surface-2:  #17191f;
  --surface-3:  #1e2028;
  --border:     #252833;
  --border-2:   #1a1c24;
  --text-1:     #e1e4ee;
  --text-2:     #878fa8;
  --text-3:     #484f64;
  --accent:     #7c6fff;
  --accent-bg:  rgba(124,111,255,0.10);
  --watch:      #32d07a;
  --watch-bg:   rgba(50,208,122,0.10);
  --shared:     #f5a623;
  --shared-bg:  rgba(245,166,35,0.10);
  --danger:     #ef5858;
  --r:          10px;
  --r-sm:       6px;
}

/* @import removed from here — it must be at the top of the file (see above) */

body {
  font-family: 'DM Sans', sans-serif;
  background: var(--bg);
  color: var(--text-1);
  display: flex;
  height: 100vh;
  overflow: hidden;
  font-size: 14px;
  line-height: 1.5;
  background-image: radial-gradient(circle, #1e2028 1px, transparent 1px);
  background-size: 28px 28px;
  background-attachment: fixed;
}

.btn {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 7px 14px; border-radius: var(--r-sm);
  font-size: 13px; font-weight: 500; font-family: 'DM Sans', sans-serif;
  cursor: pointer; border: none; transition: all 0.12s; white-space: nowrap;
}
.btn-primary { background: var(--accent); color: #fff; }
.btn-primary:hover { filter: brightness(1.12); }
.btn-ghost { background: transparent; color: var(--text-2); border: 1px solid var(--border); }
.btn-ghost:hover { background: var(--surface-2); color: var(--text-1); }
.btn-danger { background: transparent; color: var(--danger); border: 1px solid rgba(239,88,88,0.3); }
.btn-danger:hover { background: rgba(239,88,88,0.12); }

::-webkit-scrollbar { width: 3px; }
::-webkit-scrollbar-thumb { background: var(--surface-3); border-radius: 2px; }

.sec-label {
  font-family: 'DM Mono', monospace;
  font-size: 9px; font-weight: 500;
  letter-spacing: 1.5px; text-transform: uppercase;
  color: var(--text-3);
}
```

- [ ] **Replace `frontend/src/App.jsx`**

```jsx
import { useState, useEffect } from 'react';
import './App.css';
import Sidebar from './components/Sidebar';
import FolderList from './components/FolderList';
import ActivityPanel from './components/ActivityPanel';
import NetworkView from './components/NetworkView';
import SettingsView from './components/SettingsView';
import { useAppState } from './hooks/useAppState';

export default function App() {
  const [view, setView] = useState('folders');
  const state = useAppState();

  return (
    <div style={{ display: 'flex', height: '100vh', overflow: 'hidden' }}>
      <Sidebar
        activeView={view}
        onNavChange={setView}
        peers={state.peers}
        config={state.config}
        folderCount={state.folders.length}
      />

      <main style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', background: 'transparent' }}>
        {view === 'folders' && (
          <FolderList
            folders={state.folders}
            onAddFolder={state.addFolder}
            onRemoveFolder={state.removeFolder}
            onSetMode={state.setFolderMode}
          />
        )}
        {view === 'network' && (
          <NetworkView
            peers={state.peers}
            subscriptions={state.subscriptions}
            onSubscribe={state.subscribe}
            onUnsubscribe={state.unsubscribe}
          />
        )}
        {view === 'settings' && (
          <SettingsView config={state.config} onUpdate={state.updateConfig} />
        )}
      </main>

      <ActivityPanel transfers={state.transfers} />
    </div>
  );
}
```

- [ ] **Create `frontend/src/hooks/useAppState.js`**

```js
import { useState, useEffect, useCallback } from 'react';
import {
  GetFolders, AddFolder, RemoveFolder, SetFolderMode,
  GetPeers, GetSubscriptions, Subscribe, Unsubscribe,
  GetConfig, UpdateDisplayName, UpdatePort, UpdateBaseStorage,
  OpenFolderDialog,
} from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';

export function useAppState() {
  const [folders, setFolders] = useState([]);
  const [peers, setPeers] = useState([]);
  const [subscriptions, setSubscriptions] = useState([]);
  const [config, setConfig] = useState({});
  const [transfers, setTransfers] = useState([]);

  useEffect(() => {
    GetFolders().then(setFolders);
    GetPeers().then(setPeers);
    GetSubscriptions().then(setSubscriptions);
    GetConfig().then(setConfig);

    const offOnline  = EventsOn('peer:online',  p => setPeers(ps => [...ps.filter(x => x.Hostname !== p.Hostname), p]));
    const offOffline = EventsOn('peer:offline', h => setPeers(ps => ps.filter(p => p.Hostname !== h)));
    const offChanged = EventsOn('folder:changed', () => GetFolders().then(setFolders));
    const offProgress = EventsOn('transfer:progress', t => {
      setTransfers(ts => ts.map(x => x.id === t.id ? { ...x, ...t } : x));
    });
    const offDone = EventsOn('transfer:complete', t => {
      setTransfers(ts => [{ ...t, done: true }, ...ts.filter(x => x.id !== t.id)].slice(0, 20));
    });

    return () => { offOnline(); offOffline(); offChanged(); offProgress(); offDone(); };
  }, []);

  const addFolder = useCallback(async (path, mode) => {
    await AddFolder(path, mode);
    setFolders(await GetFolders());
  }, []);

  const removeFolder = useCallback(async (id) => {
    await RemoveFolder(id);
    setFolders(await GetFolders());
  }, []);

  const setFolderMode = useCallback(async (id, mode) => {
    await SetFolderMode(id, mode);
    setFolders(await GetFolders());
  }, []);

  const subscribe = useCallback(async (peerHostname, folderID, localDest) => {
    await Subscribe(peerHostname, folderID, localDest || '');
    setSubscriptions(await GetSubscriptions());
  }, []);

  const unsubscribe = useCallback(async (peerHostname, folderID) => {
    await Unsubscribe(peerHostname, folderID);
    setSubscriptions(await GetSubscriptions());
  }, []);

  const updateConfig = useCallback(async (changes) => {
    if (changes.displayName !== undefined) await UpdateDisplayName(changes.displayName);
    if (changes.port !== undefined) await UpdatePort(changes.port);
    if (changes.baseStorage !== undefined) await UpdateBaseStorage(changes.baseStorage);
    setConfig(await GetConfig());
  }, []);

  const openFolderDialog = useCallback(() => OpenFolderDialog(), []);

  return {
    folders, peers, subscriptions, config, transfers,
    addFolder, removeFolder, setFolderMode,
    subscribe, unsubscribe, updateConfig, openFolderDialog,
  };
}
```

- [ ] **Verify dev server starts**

```bash
wails dev
```

Expected: app window opens (blank layout, no errors in terminal).

- [ ] **Commit**

```bash
git add frontend/src/App.jsx frontend/src/App.css frontend/src/hooks/
git commit -m "feat: frontend layout shell and Wails state hook"
```

---

### Task 10: My Folders UI

**Files:** `frontend/src/components/Sidebar.jsx`, `frontend/src/components/FolderList.jsx`, `frontend/src/components/FolderCard.jsx`, `frontend/src/components/AddFolderModal.jsx`

- [ ] **Create `frontend/src/components/Sidebar.jsx`**

```jsx
const pulseStyle = (color) => ({
  width: 7, height: 7, borderRadius: '50%',
  background: color || 'var(--watch)',
  flexShrink: 0,
  animation: 'heartbeat 2.4s ease-in-out infinite',
});

const styles = `
@keyframes heartbeat {
  0%,100% { box-shadow: 0 0 0 0 rgba(50,208,122,0.5); }
  40%      { box-shadow: 0 0 0 5px rgba(50,208,122,0); }
}`;

export default function Sidebar({ activeView, onNavChange, peers, config, folderCount }) {
  return (
    <aside style={{ width: 238, minWidth: 238, background: 'var(--surface)', borderRight: '1px solid var(--border)', display: 'flex', flexDirection: 'column' }}>
      <style>{styles}</style>

      {/* Logo */}
      <div style={{ padding: '18px 18px 14px', borderBottom: '1px solid var(--border-2)', display: 'flex', alignItems: 'center', gap: 10 }}>
        <div style={{ width: 30, height: 30, borderRadius: 8, background: 'var(--accent)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.5"><circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/></svg>
        </div>
        <span style={{ fontFamily: 'Syne, sans-serif', fontSize: 16, fontWeight: 700, letterSpacing: '-0.4px' }}>filehub</span>
        <span style={{ marginLeft: 'auto', fontFamily: 'DM Mono, monospace', fontSize: 9, color: 'var(--text-3)', letterSpacing: '0.8px' }}>v0.1</span>
      </div>

      {/* Nav */}
      <nav style={{ padding: '10px 8px', borderBottom: '1px solid var(--border-2)' }}>
        {[
          { id: 'folders', label: 'My Folders', badge: folderCount },
          { id: 'network', label: 'Network',    badge: peers.length },
          { id: 'settings', label: 'Settings',  badge: null },
        ].map(({ id, label, badge }) => (
          <div
            key={id}
            onClick={() => onNavChange(id)}
            style={{
              display: 'flex', alignItems: 'center', gap: 9,
              padding: '7px 10px', borderRadius: 'var(--r-sm)',
              cursor: 'pointer', fontSize: 13, fontWeight: 500,
              background: activeView === id ? 'var(--accent-bg)' : 'transparent',
              color: activeView === id ? 'var(--accent)' : 'var(--text-2)',
              transition: 'background 0.12s, color 0.12s',
              userSelect: 'none',
            }}
          >
            <span style={{ flex: 1 }}>{label}</span>
            {badge != null && (
              <span style={{
                fontFamily: 'DM Mono, monospace', fontSize: 10,
                background: activeView === id ? 'var(--accent)' : 'var(--surface-3)',
                color: activeView === id ? '#fff' : 'var(--text-3)',
                padding: '1px 6px', borderRadius: 20,
              }}>{badge}</span>
            )}
          </div>
        ))}
      </nav>

      {/* Peers */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '12px 8px 0' }}>
        <div className="sec-label" style={{ padding: '0 10px', marginBottom: 6 }}>On this network</div>
        {peers.length === 0 && (
          <p style={{ padding: '8px 10px', fontSize: 12, color: 'var(--text-3)' }}>Scanning…</p>
        )}
        {peers.map(p => (
          <div key={p.Hostname} style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '7px 10px', borderRadius: 'var(--r-sm)' }}>
            <div style={pulseStyle('var(--watch)')} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 12.5, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.DisplayName || p.Hostname}</div>
              <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10, color: 'var(--text-3)' }}>{p.Addr}</div>
            </div>
          </div>
        ))}
      </div>

      {/* Self identity */}
      <div style={{ padding: '12px 18px', borderTop: '1px solid var(--border-2)', display: 'flex', alignItems: 'center', gap: 9 }}>
        <div style={pulseStyle('var(--watch)')} />
        <div>
          <div style={{ fontSize: 11.5, fontWeight: 600 }}>{config.DisplayName || config.DeviceID}</div>
          <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10, color: 'var(--text-3)' }}>:{config.Port}</div>
        </div>
      </div>
    </aside>
  );
}
```

- [ ] **Create `frontend/src/components/FolderCard.jsx`**

```jsx
export default function FolderCard({ folder, onRemove, onSetMode }) {
  const isShared = folder.Mode === 'shared';
  const modeColor = isShared ? 'var(--shared)' : 'var(--watch)';
  const modeBg    = isShared ? 'var(--shared-bg)' : 'var(--watch-bg)';

  return (
    <div style={{
      display: 'flex', alignItems: 'stretch',
      background: 'var(--surface)', border: '1px solid var(--border)',
      borderRadius: 'var(--r)', overflow: 'hidden',
      transition: 'border-color 0.18s, box-shadow 0.18s',
      animation: 'slideIn 0.18s ease-out',
    }}>
      <style>{`@keyframes slideIn { from { opacity:0; transform:translateY(6px); } to { opacity:1; transform:translateY(0); } }`}</style>
      {/* Mode accent bar */}
      <div style={{ width: 3, flexShrink: 0, background: modeColor, transition: 'background 0.3s' }} />

      <div style={{ flex: 1, display: 'flex', alignItems: 'center', gap: 14, padding: '13px 16px', minWidth: 0 }}>
        {/* Icon */}
        <div style={{ width: 36, height: 36, borderRadius: 8, background: modeBg, display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
          <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke={modeColor} strokeWidth="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>
        </div>

        {/* Info */}
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 12, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={folder.Path}>
            {folder.Path}
          </div>
          <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10.5, color: 'var(--text-3)', marginTop: 3 }}>
            {folder.Mode}
          </div>
        </div>
      </div>

      {/* Actions */}
      <div style={{ display: 'flex', alignItems: 'center', paddingRight: 12, gap: 6 }}>
        {/* Mode toggle */}
        <div style={{ display: 'flex', background: 'var(--surface-2)', border: '1px solid var(--border-2)', borderRadius: 20, padding: 3, gap: 2 }}>
          {['watch', 'shared'].map(m => (
            <button
              key={m}
              onClick={() => onSetMode(folder.ID, m)}
              style={{
                padding: '4px 11px', borderRadius: 18, fontSize: 11.5, fontWeight: 500,
                cursor: 'pointer', border: folder.Mode === m ? `1px solid ${m === 'watch' ? 'rgba(50,208,122,0.18)' : 'rgba(245,166,35,0.18)'}` : 'none',
                background: folder.Mode === m ? (m === 'watch' ? 'var(--watch-bg)' : 'var(--shared-bg)') : 'transparent',
                color: folder.Mode === m ? (m === 'watch' ? 'var(--watch)' : 'var(--shared)') : 'var(--text-3)',
                fontFamily: 'DM Sans, sans-serif', transition: 'all 0.18s', whiteSpace: 'nowrap',
              }}
            >
              {m.charAt(0).toUpperCase() + m.slice(1)}
            </button>
          ))}
        </div>

        {/* Remove */}
        <button
          onClick={() => onRemove(folder.ID)}
          title="Remove folder"
          style={{ width: 30, height: 30, borderRadius: 'var(--r-sm)', background: 'transparent', border: 'none', color: 'var(--text-3)', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', transition: 'all 0.12s' }}
          onMouseEnter={e => { e.currentTarget.style.background = 'rgba(239,88,88,0.12)'; e.currentTarget.style.color = 'var(--danger)'; }}
          onMouseLeave={e => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--text-3)'; }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Create `frontend/src/components/AddFolderModal.jsx`**

```jsx
import { useState } from 'react';
import { OpenFolderDialog } from '../../wailsjs/go/main/App';

export default function AddFolderModal({ onAdd, onClose }) {
  const [path, setPath] = useState('');
  const [mode, setMode] = useState('watch');

  const browse = async () => {
    const selected = await OpenFolderDialog();
    if (selected) setPath(selected);
  };

  const submit = () => {
    if (!path.trim()) return;
    onAdd(path.trim(), mode);
    onClose();
  };

  return (
    <div
      style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.65)', backdropFilter: 'blur(6px)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 200 }}
      onClick={e => e.target === e.currentTarget && onClose()}
    >
      <div style={{ background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 14, padding: 24, width: 430, boxShadow: '0 32px 80px rgba(0,0,0,0.6)' }}>
        <div style={{ fontFamily: 'Syne, sans-serif', fontSize: 16, fontWeight: 700, marginBottom: 20 }}>Add Folder</div>

        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--text-2)', textTransform: 'uppercase', letterSpacing: '0.6px', marginBottom: 6 }}>Folder Path</label>
          <div style={{ display: 'flex', gap: 6 }}>
            <input
              value={path}
              onChange={e => setPath(e.target.value)}
              placeholder="Select or paste a folder path"
              style={{ flex: 1, padding: '9px 12px', background: 'var(--surface-2)', border: '1px solid var(--border)', borderRadius: 'var(--r-sm)', color: 'var(--text-1)', fontFamily: 'DM Mono, monospace', fontSize: 12, outline: 'none' }}
            />
            <button className="btn btn-ghost" onClick={browse}>Browse…</button>
          </div>
        </div>

        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--text-2)', textTransform: 'uppercase', letterSpacing: '0.6px', marginBottom: 6 }}>Mode</label>
          <div style={{ display: 'flex', gap: 8 }}>
            {[
              { id: 'watch',  title: 'Watch',  desc: 'Monitor for changes and notify peers in real time' },
              { id: 'shared', title: 'Shared', desc: 'Serve files for browsing and download on demand' },
            ].map(({ id, title, desc }) => (
              <div
                key={id}
                onClick={() => setMode(id)}
                style={{
                  flex: 1, padding: '12px 14px', borderRadius: 'var(--r-sm)', cursor: 'pointer',
                  background: mode === id ? (id === 'watch' ? 'var(--watch-bg)' : 'var(--shared-bg)') : 'var(--surface-2)',
                  border: `1.5px solid ${mode === id ? (id === 'watch' ? 'var(--watch)' : 'var(--shared)') : 'var(--border-2)'}`,
                  transition: 'all 0.15s',
                }}
              >
                <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4, color: mode === id ? (id === 'watch' ? 'var(--watch)' : 'var(--shared)') : 'var(--text-2)' }}>{title}</div>
                <div style={{ fontSize: 11.5, color: 'var(--text-3)', lineHeight: 1.5 }}>{desc}</div>
              </div>
            ))}
          </div>
        </div>

        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 22 }}>
          <button className="btn btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" onClick={submit}>Add Folder</button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Create `frontend/src/components/FolderList.jsx`**

```jsx
import { useState } from 'react';
import FolderCard from './FolderCard';
import AddFolderModal from './AddFolderModal';

export default function FolderList({ folders, onAddFolder, onRemoveFolder, onSetMode }) {
  const [showModal, setShowModal] = useState(false);

  const watching = folders.filter(f => f.Mode === 'watch').length;
  const shared   = folders.filter(f => f.Mode === 'shared').length;

  return (
    <>
      {/* Topbar */}
      <div style={{ padding: '18px 24px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: 12, background: 'var(--surface)' }}>
        <div>
          <div style={{ fontFamily: 'Syne, sans-serif', fontSize: 17, fontWeight: 700, letterSpacing: '-0.4px' }}>My Folders</div>
          <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10.5, color: 'var(--text-3)', marginTop: 2 }}>
            {folders.length} folder{folders.length !== 1 ? 's' : ''} · {watching} watching · {shared} shared
          </div>
        </div>
        <div style={{ flex: 1 }} />
        <button className="btn btn-primary" onClick={() => setShowModal(true)}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          Add Folder
        </button>
      </div>

      {/* List */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '16px 24px', display: 'flex', flexDirection: 'column', gap: 7 }}>
        {folders.length === 0 && (
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 10, color: 'var(--text-3)', textAlign: 'center', padding: 48 }}>
            <div style={{ width: 52, height: 52, borderRadius: 14, background: 'var(--surface-2)', display: 'flex', alignItems: 'center', justifyContent: 'center', marginBottom: 4 }}>
              <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="var(--text-3)" strokeWidth="1.5"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>
            </div>
            <div style={{ fontFamily: 'Syne, sans-serif', fontSize: 15, fontWeight: 700, color: 'var(--text-2)' }}>No folders added</div>
            <div style={{ fontSize: 13, color: 'var(--text-3)', maxWidth: 240, lineHeight: 1.65 }}>
              Add a folder to start sharing files with other devices on your local network.
            </div>
          </div>
        )}
        {folders.map(f => (
          <FolderCard key={f.ID} folder={f} onRemove={onRemoveFolder} onSetMode={onSetMode} />
        ))}
      </div>

      {showModal && (
        <AddFolderModal
          onAdd={onAddFolder}
          onClose={() => setShowModal(false)}
        />
      )}
    </>
  );
}
```

- [ ] **Verify in wails dev** — add a folder via the UI, toggle mode, remove it. All actions should persist after closing Settings.

- [ ] **Commit**

```bash
git add frontend/src/components/
git commit -m "feat: My Folders panel with add, remove, mode toggle"
```

---

### Task 11: Activity panel

**Files:** `frontend/src/components/ActivityPanel.jsx`

- [ ] **Create `frontend/src/components/ActivityPanel.jsx`**

```jsx
export default function ActivityPanel({ transfers }) {
  const active    = transfers.filter(t => !t.done);
  const completed = transfers.filter(t =>  t.done);

  return (
    <aside style={{ width: 276, minWidth: 276, background: 'var(--surface)', borderLeft: '1px solid var(--border)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <div style={{ padding: '18px 18px 14px', borderBottom: '1px solid var(--border-2)' }}>
        <div style={{ fontFamily: 'Syne, sans-serif', fontSize: 14, fontWeight: 700 }}>Activity</div>
        <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10, color: 'var(--text-3)', marginTop: 1 }}>
          {active.length} active · {completed.length} completed
        </div>
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '12px 10px', display: 'flex', flexDirection: 'column', gap: 7 }}>
        {active.length > 0 && (
          <>
            <div className="sec-label" style={{ padding: '0 2px' }}>Active</div>
            {active.map(t => (
              <div key={t.id} style={{ background: 'var(--surface-2)', border: '1px solid var(--border-2)', borderRadius: 'var(--r-sm)', padding: '11px 12px' }}>
                <div style={{ display: 'flex', alignItems: 'flex-start', gap: 9, marginBottom: 9 }}>
                  <div style={{ width: 26, height: 26, borderRadius: 6, display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0, background: t.direction === 'download' ? 'var(--watch-bg)' : 'var(--shared-bg)', color: t.direction === 'download' ? 'var(--watch)' : 'var(--shared)' }}>
                    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
                      {t.direction === 'download'
                        ? <><polyline points="8 17 12 21 16 17"/><line x1="12" y1="12" x2="12" y2="21"/><path d="M20.88 18.09A5 5 0 0018 9h-1.26A8 8 0 103 16.29"/></>
                        : <><polyline points="16 17 12 13 8 17"/><line x1="12" y1="13" x2="12" y2="21"/><path d="M20.88 18.09A5 5 0 0018 9h-1.26A8 8 0 103 16.29"/></>}
                    </svg>
                  </div>
                  <div>
                    <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 11, lineHeight: 1.4, wordBreak: 'break-all' }}>{t.filename}</div>
                    <div style={{ fontSize: 10.5, color: 'var(--text-3)', marginTop: 1 }}>{t.direction === 'download' ? 'from' : 'to'} {t.peer}</div>
                  </div>
                </div>
                <div style={{ height: 3, background: 'var(--surface-3)', borderRadius: 2, overflow: 'hidden', marginBottom: 6 }}>
                  <div style={{ height: '100%', borderRadius: 2, background: t.direction === 'download' ? 'var(--watch)' : 'var(--shared)', width: `${t.pct || 0}%`, transition: 'width 0.3s' }} />
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span style={{ fontFamily: 'DM Mono, monospace', fontSize: 10, color: 'var(--text-3)' }}>{t.speed || '—'}</span>
                  <span style={{ fontFamily: 'DM Mono, monospace', fontSize: 10, color: 'var(--text-2)', fontWeight: 500 }}>{t.pct || 0}%</span>
                </div>
              </div>
            ))}
          </>
        )}

        {completed.length > 0 && (
          <>
            <div className="sec-label" style={{ padding: '6px 2px 0' }}>Completed</div>
            {completed.slice(0, 10).map((t, i) => (
              <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '9px 11px', background: 'var(--surface-2)', border: '1px solid var(--border-2)', borderRadius: 'var(--r-sm)' }}>
                <div style={{ width: 22, height: 22, borderRadius: '50%', background: 'var(--watch-bg)', color: 'var(--watch)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3"><polyline points="20 6 9 17 4 12"/></svg>
                </div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 11, color: 'var(--text-2)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{t.filename}</div>
                  <div style={{ fontSize: 10.5, color: 'var(--text-3)' }}>{t.direction === 'download' ? 'from' : 'to'} {t.peer}</div>
                </div>
              </div>
            ))}
          </>
        )}

        {transfers.length === 0 && (
          <p style={{ padding: '12px 4px', fontSize: 12, color: 'var(--text-3)' }}>No activity yet.</p>
        )}
      </div>

      {/* Status */}
      <div style={{ padding: '10px 18px', borderTop: '1px solid var(--border-2)', display: 'flex', alignItems: 'center', gap: 8 }}>
        <div style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--watch)', animation: 'heartbeat 2.4s ease-in-out infinite' }} />
        <span style={{ fontFamily: 'DM Mono, monospace', fontSize: 10.5, color: 'var(--text-3)' }}>Listening</span>
      </div>
    </aside>
  );
}
```

- [ ] **Commit**

```bash
git add frontend/src/components/ActivityPanel.jsx
git commit -m "feat: activity panel for transfer progress and history"
```

---

### Task 12: Network view and Settings

**Files:** `frontend/src/components/NetworkView.jsx`, `frontend/src/components/SettingsView.jsx`

- [ ] **Create `frontend/src/components/NetworkView.jsx`**

```jsx
export default function NetworkView({ peers, subscriptions, onSubscribe, onUnsubscribe }) {
  const isSubscribed = (peerHostname, folderID) =>
    subscriptions.some(s => s.PeerHostname === peerHostname && s.FolderID === folderID);

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <div style={{ padding: '18px 24px', borderBottom: '1px solid var(--border)', background: 'var(--surface)' }}>
        <div style={{ fontFamily: 'Syne, sans-serif', fontSize: 17, fontWeight: 700, letterSpacing: '-0.4px' }}>Network</div>
        <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10.5, color: 'var(--text-3)', marginTop: 2 }}>{peers.length} device{peers.length !== 1 ? 's' : ''} found</div>
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '16px 24px', display: 'flex', flexDirection: 'column', gap: 12 }}>
        {peers.length === 0 && (
          <p style={{ color: 'var(--text-3)', fontSize: 13, marginTop: 24, textAlign: 'center' }}>
            No other filehub devices found on this network.
          </p>
        )}
        {peers.map(peer => (
          <div key={peer.Hostname} style={{ background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 'var(--r)', overflow: 'hidden' }}>
            <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border-2)', display: 'flex', alignItems: 'center', gap: 10 }}>
              <div style={{ width: 7, height: 7, borderRadius: '50%', background: 'var(--watch)', flexShrink: 0 }} />
              <div>
                <div style={{ fontWeight: 600, fontSize: 13 }}>{peer.DisplayName || peer.Hostname}</div>
                <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10, color: 'var(--text-3)' }}>{peer.Addr}:{peer.Port}</div>
              </div>
            </div>
            {(peer.Folders || []).filter(f => f.Mode === 'watch').map(f => (
              <div key={f.ID} style={{ padding: '10px 16px', display: 'flex', alignItems: 'center', gap: 12, borderBottom: '1px solid var(--border-2)' }}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--watch)" strokeWidth="2"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>
                <span style={{ fontFamily: 'DM Mono, monospace', fontSize: 11, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{f.Path}</span>
                <button
                  className={isSubscribed(peer.Hostname, f.ID) ? 'btn btn-ghost' : 'btn btn-primary'}
                  style={{ fontSize: 11, padding: '4px 10px' }}
                  onClick={() => isSubscribed(peer.Hostname, f.ID)
                    ? onUnsubscribe(peer.Hostname, f.ID)
                    : onSubscribe(peer.Hostname, f.ID, '')}
                >
                  {isSubscribed(peer.Hostname, f.ID) ? 'Unsubscribe' : 'Subscribe'}
                </button>
              </div>
            ))}
            {(peer.Folders || []).filter(f => f.Mode === 'watch').length === 0 && (
              <div style={{ padding: '10px 16px', fontSize: 12, color: 'var(--text-3)' }}>No watch folders on this device.</div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Create `frontend/src/components/SettingsView.jsx`**

```jsx
import { useState, useEffect } from 'react';

export default function SettingsView({ config, onUpdate }) {
  const [displayName, setDisplayName] = useState('');
  const [port, setPort] = useState('');
  const [baseStorage, setBaseStorage] = useState('');

  useEffect(() => {
    setDisplayName(config.DisplayName || '');
    setPort(String(config.Port || 47990));
    setBaseStorage(config.BaseStorage || '');
  }, [config]);

  const save = () => {
    const p = parseInt(port, 10);
    onUpdate({
      displayName,
      port: isNaN(p) ? config.Port : p,
      baseStorage,
    });
  };

  const field = (label, value, onChange, placeholder) => (
    <div style={{ marginBottom: 20 }}>
      <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--text-2)', textTransform: 'uppercase', letterSpacing: '0.6px', marginBottom: 6 }}>{label}</label>
      <input
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder={placeholder}
        style={{ width: '100%', padding: '9px 12px', background: 'var(--surface-2)', border: '1px solid var(--border)', borderRadius: 'var(--r-sm)', color: 'var(--text-1)', fontFamily: 'DM Mono, monospace', fontSize: 12, outline: 'none' }}
      />
    </div>
  );

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <div style={{ padding: '18px 24px', borderBottom: '1px solid var(--border)', background: 'var(--surface)' }}>
        <div style={{ fontFamily: 'Syne, sans-serif', fontSize: 17, fontWeight: 700, letterSpacing: '-0.4px' }}>Settings</div>
        <div style={{ fontFamily: 'DM Mono, monospace', fontSize: 10.5, color: 'var(--text-3)', marginTop: 2 }}>Device ID: {config.DeviceID}</div>
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '24px', maxWidth: 480 }}>
        {field('Display Name', displayName, setDisplayName, config.DeviceID)}
        {field('Port', port, setPort, '47990')}
        {field('Base Storage Path', baseStorage, setBaseStorage, '~/filehub')}

        <button className="btn btn-primary" onClick={save}>Save Settings</button>

        <div style={{ marginTop: 32, padding: 16, background: 'var(--surface-2)', border: '1px solid var(--border-2)', borderRadius: 'var(--r-sm)' }}>
          <div className="sec-label" style={{ marginBottom: 8 }}>About</div>
          <div style={{ fontSize: 12, color: 'var(--text-3)', lineHeight: 1.7 }}>
            filehub v0.1 — LAN file sharing<br />
            Port changes take effect on next restart.
          </div>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Verify all views work in `wails dev`** — navigate to Network, Settings; check that saving settings persists after restart.

- [ ] **Commit**

```bash
git add frontend/src/components/NetworkView.jsx frontend/src/components/SettingsView.jsx
git commit -m "feat: Network view and Settings panel"
```

---

### Task 13: End-to-end smoke test and final build

- [ ] **Run all Go tests**

```bash
go test ./...
```

Expected: all packages pass. Fix any failures before continuing.

- [ ] **Verify `wails dev` with two instances** — open the app on two machines (or two user accounts on the same machine with different ports set). Confirm:
  - Both appear in each other's peer list within ~5 seconds.
  - Adding a Watch folder on Machine A shows up in Machine B's Network view.
  - Subscribing on B triggers a catch-up pull of existing files.
  - Writing a file to the watched folder on A causes B to receive it within ~1 second.

- [ ] **Build release binary**

```bash
wails build
```

Expected: binary in `build/bin/`. Test on target OS(es).

- [ ] **Final commit**

```bash
git add -A
git commit -m "feat: filehub v0.1 complete"
```

---

## Quick Reference

| Command | Purpose |
|---------|---------|
| `go test ./...` | Run all backend tests |
| `wails dev` | Dev mode (hot reload) |
| `wails build` | Production binary |
| `go get <pkg>` | Add dependency |
| `go mod tidy` | Clean up go.mod/go.sum |
