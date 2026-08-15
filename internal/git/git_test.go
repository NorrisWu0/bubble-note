package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatusDetectsNonRepo(t *testing.T) {
	status, err := Status(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if status.IsRepo {
		t.Fatal("temp dir should not be a git repo")
	}
}

func TestInitAndStatus(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	status, err := Status(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !status.IsRepo {
		t.Fatal("dir should be a git repo after init")
	}
	if status.Dirty {
		t.Fatal("fresh repo should be clean")
	}
	if err := os.WriteFile(filepath.Join(dir, "note.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err = Status(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Dirty {
		t.Fatal("repo should be dirty after adding an untracked file")
	}
}

func TestParseBranch(t *testing.T) {
	tests := map[string]string{
		"## main":                         "main",
		"## main...origin/main":           "main",
		"## main...origin/main [ahead 1]": "main",
		"## feature/x [behind 2]":         "feature/x",
	}
	for line, want := range tests {
		if got := parseBranch(line); got != want {
			t.Fatalf("parseBranch(%q) = %q, want %q", line, got, want)
		}
	}
}
