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
