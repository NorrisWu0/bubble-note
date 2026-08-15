package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/norriswu0/bubble-note/internal/notes"
)

func TestCreateSaveDeleteRoundtrip(t *testing.T) {
	root := t.TempDir()
	s, err := New(filepath.Join(root, "notes"), filepath.Join(root, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	note, err := s.CreateNote("My Note", "# body", []string{"Work"})
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

	note, err := s.CreateNote("Note", "before", nil)
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
