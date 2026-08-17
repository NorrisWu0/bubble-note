package store

import (
	"fmt"
	"path/filepath"
	"strings"
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

func (s *Store) CreateNote(parent, title, content string, tags []string) (notes.Note, error) {
	now := time.Now().UTC()
	note := notes.Note{
		ID:        notes.NewID(),
		Title:     title,
		Content:   content,
		Tags:      notes.NormalizeTags(tags),
		CreatedAt: now,
		UpdatedAt: now,
	}
	dir := joinDir(parent, s.files.Slug(parent, title, note.ID))
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
	newDir := joinDir(parentDir(existing.Dir), s.files.Slug(parentDir(existing.Dir), title, id))
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

// MoveNote relocates a note to a new parent directory, keeping its title and
// leaf directory name unchanged.
func (s *Store) MoveNote(id, parent string) (notes.Note, error) {
	existing, err := s.index.Get(id)
	if err != nil {
		return notes.Note{}, err
	}
	note := existing.Note
	leaf := s.files.Slug(parent, note.Title, id)
	newDir := joinDir(parent, leaf)
	if existing.Dir != newDir {
		if err := s.files.Rename(existing.Dir, newDir); err != nil {
			return notes.Note{}, err
		}
	}
	note.Parent = parent
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

// parentDir returns the parent path of a note directory, or "" at the root.
func parentDir(dir string) string {
	if idx := strings.LastIndex(dir, string(filepath.Separator)); idx >= 0 {
		return dir[:idx]
	}
	return ""
}

// joinDir joins a parent path and a leaf name into a relative directory path.
func joinDir(parent, leaf string) string {
	if parent == "" {
		return leaf
	}
	return filepath.Join(parent, leaf)
}

var _ notes.Repository = (*Store)(nil)
