package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/norriswu0/bubble-note/internal/notes"
	"github.com/norriswu0/bubble-note/internal/notes/files"
)

func TestCreateSaveDeleteRoundtrip(t *testing.T) {
	root := t.TempDir()
	s, err := New(filepath.Join(root, "notes"), filepath.Join(root, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	note, err := s.CreateNote("", "My Note", "# body", []string{"Work"})
	if err != nil {
		t.Fatal(err)
	}
	if note.ID == "" || note.Title != "My Note" {
		t.Fatalf("created note = %+v", note)
	}
	if _, err := os.Stat(filepath.Join(root, "notes", "my-note", "README.md")); err != nil {
		t.Fatal("note file missing")
	}

	saved, err := s.SaveNote(note.ID, "Renamed", "updated body", []string{"work", "urgent"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Title != "Renamed" || saved.Content != "updated body" {
		t.Fatalf("saved note = %+v", saved)
	}
	if _, err := os.Stat(filepath.Join(root, "notes", "renamed", "README.md")); err != nil {
		t.Fatal("renamed directory missing")
	}
	if _, err := os.Stat(filepath.Join(root, "notes", "my-note")); !os.IsNotExist(err) {
		t.Fatal("old directory should be gone after rename")
	}

	list, err := s.ListNotes(notes.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Title != "Renamed" {
		t.Fatalf("list = %+v", list)
	}

	if err := s.DeleteNote(note.ID); err != nil {
		t.Fatal(err)
	}
	list, err = s.ListNotes(notes.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("list after delete = %+v, want empty", list)
	}
}

func TestReloadPicksUpExternalEdits(t *testing.T) {
	root := t.TempDir()
	s, err := New(filepath.Join(root, "notes"), filepath.Join(root, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	note, err := s.CreateNote("", "Note", "before", nil)
	if err != nil {
		t.Fatal(err)
	}
	path, err := s.Path(note.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("after external edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.Reload(note.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Content != "after external edit" {
		t.Fatalf("reloaded content = %q", reloaded.Content)
	}
	list, err := s.ListNotes(notes.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Content != "after external edit" {
		t.Fatalf("indexed content = %q", list[0].Content)
	}
}

func TestSaveNoteRenamesLeafPreservingParent(t *testing.T) {
	root := t.TempDir()
	notesDir := filepath.Join(root, "notes")
	now := time.Now().UTC()
	if err := files.New(notesDir).Write("docs/github", notes.Note{ID: "n1", Title: "github", Content: "body", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	s, err := New(notesDir, filepath.Join(root, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	saved, err := s.SaveNote("n1", "github api", "body", nil)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Title != "github api" {
		t.Fatalf("title = %q, want github api", saved.Title)
	}
	if _, err := os.Stat(filepath.Join(notesDir, "docs", "github-api", "README.md")); err != nil {
		t.Fatal("leaf should be renamed under docs/")
	}
	if _, err := os.Stat(filepath.Join(notesDir, "docs", "github")); !os.IsNotExist(err) {
		t.Fatal("old leaf should be gone after rename")
	}
}

func TestCreateNoteInNestedParent(t *testing.T) {
	root := t.TempDir()
	s, err := New(filepath.Join(root, "notes"), filepath.Join(root, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	note, err := s.CreateNote("journal/bubble-note", "nested notes", "# body", nil)
	if err != nil {
		t.Fatal(err)
	}
	if note.Title != "nested notes" {
		t.Fatalf("title = %q", note.Title)
	}
	if _, err := os.Stat(filepath.Join(root, "notes", "journal", "bubble-note", "nested-notes", "README.md")); err != nil {
		t.Fatal("note should be created at journal/bubble-note/nested-notes")
	}
}

func TestMoveNoteRelocatesDirectory(t *testing.T) {
	root := t.TempDir()
	s, err := New(filepath.Join(root, "notes"), filepath.Join(root, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	note, err := s.CreateNote("journal", "note", "body", nil)
	if err != nil {
		t.Fatal(err)
	}

	moved, err := s.MoveNote(note.ID, "docs/github")
	if err != nil {
		t.Fatal(err)
	}
	if moved.Parent != "docs/github" {
		t.Fatalf("parent = %q, want docs/github", moved.Parent)
	}
	if _, err := os.Stat(filepath.Join(root, "notes", "docs", "github", "note", "README.md")); err != nil {
		t.Fatal("note should be moved to docs/github/note")
	}
	if _, err := os.Stat(filepath.Join(root, "notes", "journal", "note")); !os.IsNotExist(err) {
		t.Fatal("old location should be gone")
	}

	got, err := s.GetNote(note.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Parent != "docs/github" {
		t.Fatalf("indexed parent = %q, want docs/github", got.Parent)
	}
}
