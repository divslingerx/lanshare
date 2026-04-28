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
	peers      map[string]discovery.Peer
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
				return
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
	a.mu.RLock()
	m := a.manifests[folderID]
	a.mu.RUnlock()

	if m != nil {
		state := make(map[string]config.Change, len(m.Files))
		for rel, entry := range m.Files {
			state[rel] = config.Change{
				Path:  rel,
				Op:    config.OpWrite,
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
					if ch.Op == config.OpDelete {
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
