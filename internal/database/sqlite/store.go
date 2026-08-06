package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"fmt"
	"strings"
	"time"

	"github.com/norriswu0/bubble-note/internal/notes"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema embed.FS

type Store struct {
	db                *sql.DB
	revisionRetention int
}

func Open(path string, revisionRetention int) (*Store, error) {
	if revisionRetention < 1 {
		return nil, fmt.Errorf("revision retention must be at least 1")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	store := &Store{db: db, revisionRetention: revisionRetention}
	if err := store.initialize(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) initialize() error {
	data, err := schema.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if _, err := s.db.ExecContext(context.Background(), string(data)); err != nil {
		return fmt.Errorf("initialize schema: %w", err)
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) SetRevisionRetention(retention int) error {
	if retention < 14 {
		return fmt.Errorf("revision retention must be at least 14")
	}
	s.revisionRetention = retention
	return nil
}

func (s *Store) CreateNote(title, content string, tags []string) (notes.Note, error) {
	return s.saveNewNote(title, content, tags)
}

func (s *Store) saveNewNote(title, content string, tags []string) (notes.Note, error) {
	now := time.Now().UTC()
	noteID := newID()
	revisionID := revisionID(noteID, now)
	tx, err := s.db.Begin()
	if err != nil {
		return notes.Note{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO notes (id, title, current_revision_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, noteID, title, revisionID, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		return notes.Note{}, err
	}
	if err := insertRevision(tx, revisionID, noteID, title, content, now); err != nil {
		return notes.Note{}, err
	}
	if err := replaceTags(tx, noteID, tags); err != nil {
		return notes.Note{}, err
	}
	if err := setSyncStatusTx(tx, noteID, notes.SyncLocalOnly, ""); err != nil {
		return notes.Note{}, err
	}
	if err := tx.Commit(); err != nil {
		return notes.Note{}, err
	}
	return s.GetNote(noteID)
}

func (s *Store) GetNote(id string) (notes.Note, error) {
	row := s.db.QueryRow(`SELECT n.id, n.title, r.content, n.created_at, n.updated_at, n.current_revision_id,
		(SELECT COUNT(*) FROM revisions WHERE note_id = n.id), COALESCE(ns.status, ?)
		FROM notes n JOIN revisions r ON r.id = n.current_revision_id
		LEFT JOIN note_sync ns ON ns.note_id = n.id
		WHERE n.id = ? AND n.deleted_at IS NULL`, notes.SyncLocalOnly, id)
	var note notes.Note
	var created, updated string
	if err := row.Scan(&note.ID, &note.Title, &note.Content, &created, &updated, &note.CurrentRevID, &note.RevisionCount, &note.SyncStatus); err != nil {
		return notes.Note{}, err
	}
	var err error
	note.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return notes.Note{}, err
	}
	note.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return notes.Note{}, err
	}
	note.Tags, err = s.tags(id)
	if err != nil {
		return notes.Note{}, err
	}
	return note, nil
}

func (s *Store) ListNotes(filter notes.Filter) ([]notes.Note, error) {
	query := `SELECT n.id, n.title, r.content, n.created_at, n.updated_at, n.current_revision_id,
		(SELECT COUNT(*) FROM revisions WHERE note_id = n.id), COALESCE(ns.status, ?)
		FROM notes n JOIN revisions r ON r.id = n.current_revision_id
		LEFT JOIN note_sync ns ON ns.note_id = n.id
		WHERE n.deleted_at IS NULL`
	args := make([]interface{}, 0, 5)
	args = append(args, notes.SyncLocalOnly)
	if filter.Query != "" {
		query += ` AND n.id IN (SELECT note_id FROM revision_search WHERE revision_search MATCH ?)`
		args = append(args, searchQuery(filter.Query))
	}
	if filter.Tag != "" {
		query += ` AND n.id IN (SELECT note_id FROM note_tags WHERE tag_name = ?)`
		args = append(args, strings.ToLower(strings.TrimSpace(filter.Tag)))
	}
	if filter.From != nil {
		query += ` AND n.updated_at >= ?`
		args = append(args, filter.From.UTC().Format(time.RFC3339Nano))
	}
	if filter.Through != nil {
		query += ` AND n.updated_at < ?`
		args = append(args, filter.Through.UTC().Format(time.RFC3339Nano))
	}
	query += ` ORDER BY n.updated_at DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []notes.Note
	for rows.Next() {
		var note notes.Note
		var created, updated string
		if err := rows.Scan(&note.ID, &note.Title, &note.Content, &created, &updated, &note.CurrentRevID, &note.RevisionCount, &note.SyncStatus); err != nil {
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
		note.Tags, err = s.tags(note.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, note)
	}
	return result, rows.Err()
}

func (s *Store) SaveNote(id, title, content string, tags []string) (notes.Note, error) {
	now := time.Now().UTC()
	revID := revisionID(id, now)
	tx, err := s.db.Begin()
	if err != nil {
		return notes.Note{}, err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM notes WHERE id = ? AND deleted_at IS NULL`, id).Scan(&exists); err != nil {
		return notes.Note{}, err
	}
	if exists == 0 {
		return notes.Note{}, sql.ErrNoRows
	}
	if _, err := tx.Exec(`INSERT INTO revisions (id, note_id, title, content, created_at) VALUES (?, ?, ?, ?, ?)`, revID, id, title, content, now.Format(time.RFC3339Nano)); err != nil {
		return notes.Note{}, err
	}
	if _, err := tx.Exec(`INSERT INTO revision_search (revision_id, note_id, title, content) VALUES (?, ?, ?, ?)`, revID, id, title, content); err != nil {
		return notes.Note{}, err
	}
	if _, err := tx.Exec(`UPDATE notes SET title = ?, current_revision_id = ?, updated_at = ? WHERE id = ?`, title, revID, now.Format(time.RFC3339Nano), id); err != nil {
		return notes.Note{}, err
	}
	if err := replaceTags(tx, id, tags); err != nil {
		return notes.Note{}, err
	}
	if err := setSyncStatusTx(tx, id, notes.SyncLocalOnly, ""); err != nil {
		return notes.Note{}, err
	}
	if err := pruneRevisions(tx, id, s.revisionRetention); err != nil {
		return notes.Note{}, err
	}
	if err := tx.Commit(); err != nil {
		return notes.Note{}, err
	}
	return s.GetNote(id)
}

func (s *Store) DeleteNote(id string) error {
	result, err := s.db.Exec(`UPDATE notes SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteNoteAtomic(id string, deleteRemote func() error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE notes SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	if deleteRemote != nil {
		if err := deleteRemote(); err != nil {
			return fmt.Errorf("delete remote note: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) SetSyncStatus(noteID string, status notes.SyncStatus, etag string) error {
	_, err := s.db.Exec(`INSERT INTO note_sync (note_id, status, remote_etag) VALUES (?, ?, ?)
		ON CONFLICT(note_id) DO UPDATE SET status = excluded.status, remote_etag = excluded.remote_etag`, noteID, status, etag)
	return err
}

func (s *Store) SyncETag(noteID string) (string, error) {
	var etag string
	err := s.db.QueryRow(`SELECT remote_etag FROM note_sync WHERE note_id = ?`, noteID).Scan(&etag)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return etag, err
}

func (s *Store) ApplyRemoteSnapshot(noteID string, snapshot notes.SyncSnapshot) (notes.Note, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return notes.Note{}, err
	}
	defer tx.Rollback()
	for _, revision := range snapshot.Revisions {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO revisions (id, note_id, title, content, created_at) VALUES (?, ?, ?, ?, ?)`, revision.ID, noteID, revision.Title, revision.Content, revision.CreatedAt.UTC().Format(time.RFC3339Nano)); err != nil {
			return notes.Note{}, err
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO revision_search (revision_id, note_id, title, content) VALUES (?, ?, ?, ?)`, revision.ID, noteID, revision.Title, revision.Content); err != nil {
			return notes.Note{}, err
		}
	}
	if _, err := tx.Exec(`UPDATE notes SET title = ?, current_revision_id = ?, updated_at = ? WHERE id = ?`, snapshot.Note.Title, snapshot.Note.CurrentRevID, snapshot.Note.UpdatedAt.UTC().Format(time.RFC3339Nano), noteID); err != nil {
		return notes.Note{}, err
	}
	if err := replaceTags(tx, noteID, snapshot.Note.Tags); err != nil {
		return notes.Note{}, err
	}
	if err := pruneRevisions(tx, noteID, s.revisionRetention); err != nil {
		return notes.Note{}, err
	}
	if err := setSyncStatusTx(tx, noteID, notes.SyncSynced, ""); err != nil {
		return notes.Note{}, err
	}
	if err := tx.Commit(); err != nil {
		return notes.Note{}, err
	}
	return s.GetNote(noteID)
}

func (s *Store) CopyRemoteSnapshot(snapshot notes.SyncSnapshot) (notes.Note, error) {
	return s.CreateNote("Copy of "+snapshot.Note.Title, snapshot.Note.Content, snapshot.Note.Tags)
}

func (s *Store) ListRevisions(noteID string) ([]notes.Revision, error) {
	rows, err := s.db.Query(`SELECT id, note_id, title, content, created_at FROM revisions WHERE note_id = ? ORDER BY created_at DESC`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var revisions []notes.Revision
	for rows.Next() {
		var revision notes.Revision
		var created string
		if err := rows.Scan(&revision.ID, &revision.NoteID, &revision.Title, &revision.Content, &created); err != nil {
			return nil, err
		}
		revision.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}
	return revisions, rows.Err()
}

func (s *Store) RestoreRevision(noteID, revisionID string) (notes.Note, error) {
	row := s.db.QueryRow(`SELECT title, content FROM revisions WHERE id = ? AND note_id = ?`, revisionID, noteID)
	var title, content string
	if err := row.Scan(&title, &content); err != nil {
		return notes.Note{}, err
	}
	note, err := s.GetNote(noteID)
	if err != nil {
		return notes.Note{}, err
	}
	return s.SaveNote(noteID, title, content, note.Tags)
}

func (s *Store) tags(noteID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT tag_name FROM note_tags WHERE note_id = ? ORDER BY tag_name`, noteID)
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

func insertRevision(tx *sql.Tx, revisionID, noteID, title, content string, now time.Time) error {
	if _, err := tx.Exec(`INSERT INTO revisions (id, note_id, title, content, created_at) VALUES (?, ?, ?, ?, ?)`, revisionID, noteID, title, content, now.Format(time.RFC3339Nano)); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO revision_search (revision_id, note_id, title, content) VALUES (?, ?, ?, ?)`, revisionID, noteID, title, content)
	return err
}

func replaceTags(tx *sql.Tx, noteID string, tags []string) error {
	if _, err := tx.Exec(`DELETE FROM note_tags WHERE note_id = ?`, noteID); err != nil {
		return err
	}
	for _, tag := range notes.NormalizeTags(tags) {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO tags (name) VALUES (?)`, tag); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO note_tags (note_id, tag_name) VALUES (?, ?)`, noteID, tag); err != nil {
			return err
		}
	}
	return nil
}

func pruneRevisions(tx *sql.Tx, noteID string, keep int) error {
	_, err := tx.Exec(`DELETE FROM revision_search WHERE revision_id IN (
		SELECT id FROM revisions WHERE note_id = ? ORDER BY created_at DESC LIMIT -1 OFFSET ?
	)`, noteID, keep)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`DELETE FROM revisions WHERE note_id = ? AND id NOT IN (
		SELECT id FROM revisions WHERE note_id = ? ORDER BY created_at DESC LIMIT ?
	)`, noteID, noteID, keep)
	return err
}

func setSyncStatusTx(tx *sql.Tx, noteID string, status notes.SyncStatus, etag string) error {
	_, err := tx.Exec(`INSERT INTO note_sync (note_id, status, remote_etag) VALUES (?, ?, ?)
		ON CONFLICT(note_id) DO UPDATE SET status = excluded.status, remote_etag = excluded.remote_etag`, noteID, status, etag)
	return err
}

func revisionID(noteID string, now time.Time) string {
	return fmt.Sprintf("%s-rev-%s-%s", noteID, now.Format("20060102T150405.000000000Z"), newID()[:8])
}

func searchQuery(query string) string {
	terms := strings.Fields(query)
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		quoted = append(quoted, `"`+strings.ReplaceAll(term, `"`, ``)+`"`)
	}
	return strings.Join(quoted, " ")
}

func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("generate identifier: %v", err))
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

var _ notes.Repository = (*Store)(nil)
