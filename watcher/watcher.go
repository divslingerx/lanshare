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
	fsw       *fsnotify.Watcher
	onChange  func(folderID string, changes []config.Change)
	roots     map[string]string
	manifests map[string]*manifest.Manifest
	pending   map[string]map[string]fsnotify.Op
	timer     *time.Timer
	mu        sync.Mutex
	delay     time.Duration
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
	if event.Op&fsnotify.Chmod != 0 && event.Op == fsnotify.Chmod {
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
					Op:   config.OpDelete,
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
				Op:    config.OpWrite,
				MTime: info.ModTime().Unix(),
				Size:  info.Size(),
			})
		}
		if len(changes) > 0 {
			w.onChange(folderID, changes)
		}
	}
}
