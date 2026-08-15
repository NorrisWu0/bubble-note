package sqlite

import (
	"testing"
	"time"

	"github.com/norriswu0/bubble-note/internal/notes"
)

func TestIndexRebuildsAndSearches(t *testing.T) {
	index, err := Open(t.TempDir() + "/index.db")
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()

	now := time.Now().UTC()
	entries := []notes.FileNote{
		{Note: notes.Note{ID: "a", Title: "tea", Content: "buy jasmine pearls", Tags: []string{"shopping"}, CreatedAt: now, UpdatedAt: now}, Dir: "tea"},
		{Note: notes.Note{ID: "b", Title: "work", Content: "finish report", Tags: []string{"work"}, CreatedAt: now, UpdatedAt: now.Add(time.Second)}, Dir: "work"},
	}
	if err := index.Rebuild(entries); err != nil {
		t.Fatal(err)
	}

	results, err := index.List(notes.Filter{Query: "jasmine"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != "a" {
		t.Fatalf("content search returned %+v", results)
	}

	results, err = index.List(notes.Filter{Tag: "work"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != "b" {
		t.Fatalf("tag search returned %+v", results)
	}

	if results[0].Title != "work" {
		t.Fatalf("newest note = %+v, want work", results[0])
	}
}

func TestIndexUpsertAndDelete(t *testing.T) {
	index, err := Open(t.TempDir() + "/index.db")
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()

	now := time.Now().UTC()
	entry := notes.FileNote{Note: notes.Note{ID: "a", Title: "one", Content: "body", CreatedAt: now, UpdatedAt: now}, Dir: "one"}
	if err := index.Upsert(entry); err != nil {
		t.Fatal(err)
	}
	entry.Note.Title = "renamed"
	entry.Dir = "renamed"
	if err := index.Upsert(entry); err != nil {
		t.Fatal(err)
	}
	got, err := index.Get("a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Note.Title != "renamed" || got.Dir != "renamed" {
		t.Fatalf("get = %+v, want renamed", got)
	}
	if err := index.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := index.Get("a"); err == nil {
		t.Fatal("expected note to be deleted")
	}
}
