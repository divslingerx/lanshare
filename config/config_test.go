package config_test

import (
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
