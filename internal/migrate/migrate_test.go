package migrate

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func seedLegacyDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notes.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE notes (id TEXT PRIMARY KEY, title TEXT NOT NULL, current_revision_id TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT)`,
		`CREATE TABLE revisions (id TEXT PRIMARY KEY, note_id TEXT NOT NULL, title TEXT NOT NULL, content TEXT NOT NULL, created_at TEXT NOT NULL)`,
		`CREATE TABLE note_tags (note_id TEXT NOT NULL, tag_name TEXT NOT NULL, PRIMARY KEY (note_id, tag_name))`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	created := time.Now().UTC().Format(time.RFC3339Nano)
	updated := time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano)
	rows := [][]interface{}{
		{"note-1", "First Note", "rev-1", created, updated, nil},
		{"note-2", "First Note", "rev-2", created, updated, nil},
	}
	for _, row := range rows {
		if _, err := db.Exec(`INSERT INTO notes (id, title, current_revision_id, created_at, updated_at, deleted_at) VALUES (?, ?, ?, ?, ?, ?)`, row...); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range [][]interface{}{
		{"rev-1", "note-1", "First Note", "# body one", created},
		{"rev-2", "note-2", "First Note", "# body two", created},
	} {
		if _, err := db.Exec(`INSERT INTO revisions (id, note_id, title, content, created_at) VALUES (?, ?, ?, ?, ?)`, row...); err != nil {
			t.Fatal(err)
		}
	}
	for _, tag := range []struct{ note, tag string }{
		{"note-1", "work"},
		{"note-2", "personal"},
	} {
		if _, err := db.Exec(`INSERT INTO note_tags (note_id, tag_name) VALUES (?, ?)`, tag.note, tag.tag); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestReadLegacyAndExport(t *testing.T) {
	dbPath := seedLegacyDB(t)
	notesDir := filepath.Join(t.TempDir(), "notes")

	legacy, err := ReadLegacy(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(legacy) != 2 {
		t.Fatalf("legacy notes = %d, want 2", len(legacy))
	}

	exported, skipped, err := Export(notesDir, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if exported != 2 || skipped != 0 {
		t.Fatalf("exported = %d, skipped = %d, want 2/0", exported, skipped)
	}

	body, err := os.ReadFile(filepath.Join(notesDir, "first-note", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "# body one" {
		t.Fatalf("body = %q, want # body one", string(body))
	}
}

func TestExportSkipsExistingDirectories(t *testing.T) {
	dbPath := seedLegacyDB(t)
	notesDir := filepath.Join(t.TempDir(), "notes")

	legacy, err := ReadLegacy(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := Export(notesDir, legacy); err != nil {
		t.Fatal(err)
	}
	exported, skipped, err := Export(notesDir, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if exported != 0 || skipped != 2 {
		t.Fatalf("second run exported = %d, skipped = %d, want 0/2", exported, skipped)
	}
}

func TestReadLegacyRejectsMissingFile(t *testing.T) {
	if _, err := ReadLegacy(filepath.Join(t.TempDir(), "missing.db")); err == nil {
		t.Fatal("expected an error for a missing database")
	}
}

func TestReadLegacyRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.db")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLegacy(path); err == nil {
		t.Fatal("expected an error for an empty database")
	}
}
