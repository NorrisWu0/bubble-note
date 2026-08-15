package files

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/norriswu0/bubble-note/internal/notes"
)

func TestWriteReadRoundtrip(t *testing.T) {
	store := New(t.TempDir())
	now := time.Now().UTC().Truncate(time.Nanosecond)
	note := notes.Note{ID: "a", Title: "Quiet Cat", Content: "# hello\nbody", Tags: []string{"Work", "urgent"}, CreatedAt: now, UpdatedAt: now}
	if err := store.Write("quiet-cat", note); err != nil {
		t.Fatal(err)
	}
	read, err := store.Read("quiet-cat")
	if err != nil {
		t.Fatal(err)
	}
	if read.ID != "a" || read.Title != "Quiet Cat" || read.Content != "# hello\nbody" {
		t.Fatalf("read = %+v", read)
	}
	if len(read.Tags) != 2 || read.Tags[0] != "work" || read.Tags[1] != "urgent" {
		t.Fatalf("tags = %v", read.Tags)
	}
	if !read.UpdatedAt.Equal(now) {
		t.Fatalf("updated = %v, want %v", read.UpdatedAt, now)
	}
	if _, err := os.Stat(filepath.Join(store.root, "quiet-cat", "README.md")); err != nil {
		t.Fatal("README.md missing")
	}
	if _, err := os.Stat(filepath.Join(store.root, "quiet-cat", "manifest.json")); err != nil {
		t.Fatal("manifest.json missing")
	}
}

func TestSlugCollisionAppendsSuffix(t *testing.T) {
	store := New(t.TempDir())
	now := time.Now().UTC()
	if err := store.Write("quiet-cat", notes.Note{ID: "a", Title: "quiet cat", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if got := store.Slug("quiet cat", "b"); got != "quiet-cat-2" {
		t.Fatalf("slug = %q, want quiet-cat-2", got)
	}
	if got := store.Slug("quiet cat", "a"); got != "quiet-cat" {
		t.Fatalf("slug for own note = %q, want quiet-cat", got)
	}
}

func TestScanSkipsNonNoteDirectories(t *testing.T) {
	store := New(t.TempDir())
	now := time.Now().UTC()
	if err := store.Write("note", notes.Note{ID: "a", Title: "note", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(store.root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := store.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Note.ID != "a" {
		t.Fatalf("scan = %+v", entries)
	}
}

func TestRemoveDeletesDirectory(t *testing.T) {
	store := New(t.TempDir())
	now := time.Now().UTC()
	if err := store.Write("note", notes.Note{ID: "a", Title: "note", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove("note"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(store.root, "note")); !os.IsNotExist(err) {
		t.Fatal("directory should be removed")
	}
}
