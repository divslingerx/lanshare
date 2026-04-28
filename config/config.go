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

type ChangeOp string

const (
	OpWrite  ChangeOp = "write"
	OpDelete ChangeOp = "delete"
)

type Change struct {
	Path  string   `json:"path"`
	Op    ChangeOp `json:"op"`
	MTime int64    `json:"mtime"`
	Size  int64    `json:"size"`
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
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.json")
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

func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "filehub", "config.json"), nil
}
