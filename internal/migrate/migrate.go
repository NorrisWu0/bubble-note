package migrate

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/norriswu0/bubble-note/internal/notes"
	"github.com/norriswu0/bubble-note/internal/notes/files"
	_ "modernc.org/sqlite"
)

// LegacyNote is a note read from the old SQLite database schema.
type LegacyNote struct {
	ID        string
	Title     string
	Content   string
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ReadLegacy reads the latest content of every non-deleted note from the legacy
// SQLite database (the pre-files schema).
func ReadLegacy(dbPath string) ([]LegacyNote, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open legacy database: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT n.id, n.title, r.content, n.created_at, n.updated_at
		FROM notes n
		JOIN revisions r ON r.id = n.current_revision_id
		WHERE n.deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("read legacy notes: %w", err)
	}
	defer rows.Close()

	var result []LegacyNote
	for rows.Next() {
		var note LegacyNote
		var created, updated string
		if err := rows.Scan(&note.ID, &note.Title, &note.Content, &created, &updated); err != nil {
			return nil, err
		}
		note.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, err
		}
		note.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
		if err != nil {
			return nil, err
		}
		note.Tags, err = legacyTags(db, note.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, note)
	}
	return result, rows.Err()
}

func legacyTags(db *sql.DB, noteID string) ([]string, error) {
	rows, err := db.Query(`SELECT tag_name FROM note_tags WHERE note_id = ? ORDER BY tag_name`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// Export writes legacy notes into the notes directory, preserving their IDs and
// timestamps. Existing note directories are skipped so a re-run never overwrites
// newer files. It returns the number exported and the number skipped.
func Export(notesDir string, legacyNotes []LegacyNote) (exported, skipped int, err error) {
	store := files.New(notesDir)
	if err := store.Ensure(); err != nil {
		return 0, 0, err
	}
	for _, legacy := range legacyNotes {
		dir := store.Slug(legacy.Title, legacy.ID)
		if store.Exists(dir) {
			skipped++
			continue
		}
		note := notes.Note{
			ID:        legacy.ID,
			Title:     legacy.Title,
			Content:   legacy.Content,
			Tags:      legacy.Tags,
			CreatedAt: legacy.CreatedAt,
			UpdatedAt: legacy.UpdatedAt,
		}
		if err := store.Write(dir, note); err != nil {
			return exported, skipped, fmt.Errorf("export note %q: %w", legacy.ID, err)
		}
		exported++
	}
	return exported, skipped, nil
}
