package store

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/norriswu0/bubble-note/internal/database/sqlite"
	"github.com/norriswu0/bubble-note/internal/notes"
	"github.com/norriswu0/bubble-note/internal/notes/files"
)

// Store combines the file-backed source of truth with a derived SQLite index.
type Store struct {
	files *files.Store
	index *sqlite.Index
}

func New(root, indexPath string) (*Store, error) {
	filesStore := files.New(root)
	if err := filesStore.Ensure(); err != nil {
		return nil, fmt.Errorf("create notes directory: %w", err)
	}
	index, err := sqlite.Open(indexPath)
	if err != nil {
		return nil, err
	}
	store := &Store{files: filesStore, index: index}
	if err := store.Refresh(); err != nil {
		index.Close()
		return nil, fmt.Errorf("index note files: %w", err)
	}
	return store, nil
}

func (s *Store) Close() error { return s.index.Close() }

// NotesDir returns the root directory holding the note files.
func (s *Store) NotesDir() string { return s.files.Root() }

// Refresh rescans the note files and rebuilds the index from them.
func (s *Store) Refresh() error {
	entries, err := s.files.Scan()
	if err != nil {
		return err
	}
	return s.index.Rebuild(entries)
}

// Path returns the path to a note's markdown body for external editing.
func (s *Store) Path(id string) (string, error) {
	entry, err := s.index.Get(id)
	if err != nil {
		return "", err
	}
	return s.files.Path(entry.Dir), nil
}

// Reload re-reads a note from disk (after external edits) and re-indexes it,
// bumping its updated timestamp.
func (s *Store) Reload(id string) (notes.Note, error) {
	entry, err := s.index.Get(id)
	if err != nil {
		return notes.Note{}, err
	}
	note, err := s.files.Read(entry.Dir)
	if err != nil {
		return notes.Note{}, err
	}
	note.UpdatedAt = time.Now().UTC()
	if err := s.files.Write(entry.Dir, note); err != nil {
		return notes.Note{}, err
	}
	if err := s.index.Upsert(notes.FileNote{Note: note, Dir: entry.Dir}); err != nil {
		return notes.Note{}, err
	}
	return note, nil
}

func (s *Store) ListNotes(filter notes.Filter) ([]notes.Note, error) {
	return s.index.List(filter)
}

func (s *Store) GetNote(id string) (notes.Note, error) {
	entry, err := s.index.Get(id)
	return entry.Note, err
}

func (s *Store) CreateNote(title, content string, tags []string) (notes.Note, error) {
	now := time.Now().UTC()
	note := notes.Note{
		ID:        newID(),
		Title:     title,
		Content:   content,
		Tags:      notes.NormalizeTags(tags),
		CreatedAt: now,
		UpdatedAt: now,
	}
	dir := s.files.Slug(title, note.ID)
	if err := s.files.Write(dir, note); err != nil {
		return notes.Note{}, err
	}
	if err := s.index.Upsert(notes.FileNote{Note: note, Dir: dir}); err != nil {
		return notes.Note{}, err
	}
	return note, nil
}

func (s *Store) SaveNote(id, title, content string, tags []string) (notes.Note, error) {
	existing, err := s.index.Get(id)
	if err != nil {
		return notes.Note{}, err
	}
	note := existing.Note
	note.Title = title
	note.Content = content
	note.Tags = notes.NormalizeTags(tags)
	note.UpdatedAt = time.Now().UTC()
	newDir := s.files.Slug(title, id)
	if existing.Dir != newDir {
		if err := s.files.Rename(existing.Dir, newDir); err != nil {
			return notes.Note{}, err
		}
	}
	if err := s.files.Write(newDir, note); err != nil {
		return notes.Note{}, err
	}
	if err := s.index.Upsert(notes.FileNote{Note: note, Dir: newDir}); err != nil {
		return notes.Note{}, err
	}
	return note, nil
}

func (s *Store) DeleteNote(id string) error {
	entry, err := s.index.Get(id)
	if err != nil {
		return err
	}
	if err := s.files.Remove(entry.Dir); err != nil {
		return err
	}
	return s.index.Delete(id)
}

func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("generate identifier: %v", err))
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

var _ notes.Repository = (*Store)(nil)
