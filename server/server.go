package server

import (
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
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
	hubs       map[string]*Hub                    // folderID -> Hub
	folders    map[string]config.Folder            // folderID -> Folder
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
	f.Path = filepath.Clean(f.Path)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.folders[f.ID] = f
	s.hubs[f.ID] = NewHub()
}

func (s *Server) RemoveFolder(folderID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if hub, ok := s.hubs[folderID]; ok {
		hub.Close()
	}
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
