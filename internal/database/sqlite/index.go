package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/norriswu0/bubble-note/internal/notes"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema embed.FS

// Index is a rebuildable SQLite full-text index derived from the note files.
// Files are the source of truth; this index can always be regenerated.
type Index struct {
	db *sql.DB
}

func Open(path string) (*Index, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite index: %w", err)
	}
	index := &Index{db: db}
	if err := index.initialize(); err != nil {
		db.Close()
		return nil, err
	}
	return index, nil
}

func (i *Index) initialize() error {
	data, err := schema.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if _, err := i.db.ExecContext(context.Background(), string(data)); err != nil {
		return fmt.Errorf("initialize schema: %w", err)
	}
	return nil
}

func (i *Index) Close() error { return i.db.Close() }

// Rebuild wipes the index and repopulates it from the given note files.
func (i *Index) Rebuild(entries []notes.FileNote) error {
	tx, err := i.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM note_search`,
		`DELETE FROM note_tags`,
		`DELETE FROM notes`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		if err := upsertTx(tx, entry); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Upsert inserts or replaces a single note in the index.
func (i *Index) Upsert(entry notes.FileNote) error {
	tx, err := i.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := upsertTx(tx, entry); err != nil {
		return err
	}
	return tx.Commit()
}

// Delete removes a note from the index.
func (i *Index) Delete(id string) error {
	tx, err := i.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM note_search WHERE id = ?`,
		`DELETE FROM note_tags WHERE note_id = ?`,
		`DELETE FROM notes WHERE id = ?`,
	} {
		if _, err := tx.Exec(stmt, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Get returns a note and its directory from the index.
func (i *Index) Get(id string) (notes.FileNote, error) {
	row := i.db.QueryRow(`SELECT id, dir, title, content, tags, created_at, updated_at FROM notes WHERE id = ?`, id)
	return scanFileNote(row)
}

// List returns notes matching the filter, newest first.
func (i *Index) List(filter notes.Filter) ([]notes.Note, error) {
	query := `SELECT id, dir, title, content, tags, created_at, updated_at FROM notes WHERE 1=1`
	args := make([]interface{}, 0, 5)
	if filter.Query != "" {
		query += ` AND id IN (SELECT id FROM note_search WHERE note_search MATCH ?)`
		args = append(args, searchQuery(filter.Query))
	}
	if filter.Tag != "" {
		query += ` AND id IN (SELECT note_id FROM note_tags WHERE tag_name = ?)`
		args = append(args, strings.ToLower(strings.TrimSpace(filter.Tag)))
	}
	if filter.From != nil {
		query += ` AND updated_at >= ?`
		args = append(args, filter.From.UTC().Format(time.RFC3339Nano))
	}
	if filter.Through != nil {
		query += ` AND updated_at < ?`
		args = append(args, filter.Through.UTC().Format(time.RFC3339Nano))
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := i.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []notes.Note
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, note)
	}
	return result, rows.Err()
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanFileNote(row rowScanner) (notes.FileNote, error) {
	var note notes.Note
	var dir, tagsJSON, created, updated string
	if err := row.Scan(&note.ID, &dir, &note.Title, &note.Content, &tagsJSON, &created, &updated); err != nil {
		return notes.FileNote{}, err
	}
	note.Tags = parseTags(tagsJSON)
	note.Parent = notes.ParentOf(dir)
	var err error
	note.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return notes.FileNote{}, err
	}
	note.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return notes.FileNote{}, err
	}
	return notes.FileNote{Note: note, Dir: dir}, nil
}

func scanNote(row rowScanner) (notes.Note, error) {
	entry, err := scanFileNote(row)
	return entry.Note, err
}

func upsertTx(tx *sql.Tx, entry notes.FileNote) error {
	tagsJSON, err := json.Marshal(entry.Note.Tags)
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		`DELETE FROM note_search WHERE id = ?`,
		`DELETE FROM note_tags WHERE note_id = ?`,
		`DELETE FROM notes WHERE id = ?`,
	} {
		if _, err := tx.Exec(stmt, entry.Note.ID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO notes (id, dir, title, content, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.Note.ID, entry.Dir, entry.Note.Title, entry.Note.Content, string(tagsJSON),
		entry.Note.CreatedAt.UTC().Format(time.RFC3339Nano), entry.Note.UpdatedAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO note_search (id, title, content) VALUES (?, ?, ?)`,
		entry.Note.ID, entry.Note.Title, entry.Note.Content); err != nil {
		return err
	}
	for _, tag := range entry.Note.Tags {
		if _, err := tx.Exec(`INSERT INTO note_tags (note_id, tag_name) VALUES (?, ?)`, entry.Note.ID, tag); err != nil {
			return err
		}
	}
	return nil
}

func parseTags(raw string) []string {
	if raw == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}

func searchQuery(query string) string {
	terms := strings.Fields(query)
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		quoted = append(quoted, `"`+strings.ReplaceAll(term, `"`, ``)+`"`)
	}
	return strings.Join(quoted, " ")
}
