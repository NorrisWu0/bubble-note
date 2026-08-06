package sqlite

import (
	"testing"

	"github.com/norriswu0/bubble-note/internal/notes"
)

func TestSavePrunesOldRevisions(t *testing.T) {
	store, err := Open(t.TempDir()+"/notes.db", 2)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	note, err := store.CreateNote("first", "one", []string{"Work"})
	if err != nil {
		t.Fatal(err)
	}
	for i, content := range []string{"two", "three", "four"} {
		note, err = store.SaveNote(note.ID, "updated", content, []string{"work"})
		if err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}
	revisions, err := store.ListRevisions(note.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 {
		t.Fatalf("revision count = %d, want 2", len(revisions))
	}
	if revisions[0].Content != "four" {
		t.Fatalf("latest content = %q, want four", revisions[0].Content)
	}
}

func TestListNotesSearchesContentAndTags(t *testing.T) {
	store, err := Open(t.TempDir()+"/notes.db", 14)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.CreateNote("tea", "buy jasmine pearls", []string{"shopping"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateNote("work", "finish report", []string{"work"}); err != nil {
		t.Fatal(err)
	}
	results, err := store.ListNotes(notes.Filter{Query: "jasmine"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "tea" {
		t.Fatalf("content search returned %+v", results)
	}
	results, err = store.ListNotes(notes.Filter{Tag: "work"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "work" {
		t.Fatalf("tag search returned %+v", results)
	}
}

func TestNoteSyncStatusPersistsAndDefaultsLocalOnly(t *testing.T) {
	store, err := Open(t.TempDir()+"/notes.db", 14)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	note, err := store.CreateNote("sync", "body", nil)
	if err != nil {
		t.Fatal(err)
	}
	if note.SyncStatus != notes.SyncLocalOnly {
		t.Fatalf("new note status = %q, want local-only", note.SyncStatus)
	}
	if err := store.SetSyncStatus(note.ID, notes.SyncSynced, "etag"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetNote(note.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SyncStatus != notes.SyncSynced {
		t.Fatalf("loaded note status = %q, want synced", loaded.SyncStatus)
	}
}

func TestDeleteAtomicRollsBackWhenRemoteDeleteFails(t *testing.T) {
	store, err := Open(t.TempDir()+"/notes.db", 14)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	note, err := store.CreateNote("keep", "body", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteNoteAtomic(note.ID, func() error { return assertError{} }); err == nil {
		t.Fatal("expected remote deletion error")
	}
	if _, err := store.GetNote(note.ID); err != nil {
		t.Fatalf("note should remain after rollback: %v", err)
	}
}

type assertError struct{}

func (assertError) Error() string { return "remote unavailable" }
