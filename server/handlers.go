package server

import (
	"encoding/json"
	"fmt"
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
	if resp.Entries == nil {
		resp.Entries = []BrowseEntry{}
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
	_, ok := s.folders[folderID]
	state := s.fileState[folderID]
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	changes := make([]config.Change, 0)
	for _, c := range state {
		if c.MTime > since.Unix() {
			changes = append(changes, c)
		}
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

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

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
