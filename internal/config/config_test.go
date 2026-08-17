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
	if cfg.Storage.Prefix != "" || cfg.Storage.PathStyle {
		t.Fatalf("storage defaults = %+v, want empty prefix and virtual-hosted addressing", cfg.Storage)
	}
}

func TestLoadConfiguredRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("revision_retention: 14\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RevisionRetention != 14 {
		t.Fatalf("retention = %d, want 14", cfg.RevisionRetention)
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

func TestLoadRejectsRetentionBelowMinimum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("revision_retention: 13\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected retention minimum error")
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

func TestLoadAllowsIncompleteStorageForSettingsEditing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("storage:\n  bucket: bubble-note\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal("incomplete storage should remain editable: ", err)
	}
}

func TestLoadMigratesLegacySyncNamespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := "sync:\n  region: ap-southeast-2\n  bucket: bubble-note\n  access_key_id: access\n  secret_access_key: secret\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.Bucket != "bubble-note" || cfg.Storage.Region != "ap-southeast-2" {
		t.Fatalf("storage = %+v, want migrated legacy values", cfg.Storage)
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

func TestEditorAndGitClientDefaults(t *testing.T) {
	cfg := Default()
	if got := cfg.GitClientCommand(); got != "lazygit" {
		t.Fatalf("git client = %q, want lazygit", got)
	}
	if got := cfg.EditorCommand(); got != "nvim" {
		t.Fatalf("editor = %q, want nvim fallback", got)
	}
	cfg.Editor = "code -w"
	if got := cfg.EditorCommand(); got != "code -w" {
		t.Fatalf("editor = %q, want code -w", got)
	}
}

func TestNotesDirectoryResolvesDefault(t *testing.T) {
	cfg := Default()
	dir, err := cfg.NotesDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" || !filepath.IsAbs(dir) {
		t.Fatalf("notes dir = %q, want absolute default", dir)
	}
	cfg.NotesDir = "/tmp/notes"
	dir, err = cfg.NotesDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/tmp/notes" {
		t.Fatalf("notes dir = %q, want /tmp/notes", dir)
	}
}
