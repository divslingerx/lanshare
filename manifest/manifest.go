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
			return err
		}
		hash, err := hashFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
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
	tmp, err := os.CreateTemp(filepath.Dir(path), ".manifest-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
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
