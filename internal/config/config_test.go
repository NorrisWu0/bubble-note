package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsWhenConfigIsMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RevisionRetention != DefaultRevisionRetention {
		t.Fatalf("retention = %d, want %d", cfg.RevisionRetention, DefaultRevisionRetention)
	}
}

func TestLoadConfiguredRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("revision_retention: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RevisionRetention != 3 {
		t.Fatalf("retention = %d, want 3", cfg.RevisionRetention)
	}
}

func TestLoadRejectsInvalidRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("revision_retention: 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid retention error")
	}
}

func TestLoadThemeDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme.Preset != "catppuccin" || cfg.Theme.Flavor != "mocha" {
		t.Fatalf("theme = %+v, want catppuccin mocha", cfg.Theme)
	}
}

func TestLoadIndentationSetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("indent_spaces: 4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IndentSpaces != 4 {
		t.Fatalf("indent spaces = %d, want 4", cfg.IndentSpaces)
	}
}

func TestLoadOrCreateWritesDefaultConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bubble-note", "config.yaml")
	cfg, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RevisionRetention != DefaultRevisionRetention || cfg.IndentSpaces != DefaultIndentSpaces {
		t.Fatalf("config = %+v, want default values", cfg)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("default config was not written: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Theme.Preset != "catppuccin" || loaded.Theme.Flavor != "mocha" {
		t.Fatalf("loaded theme = %+v, want Catppuccin Mocha", loaded.Theme)
	}
}
