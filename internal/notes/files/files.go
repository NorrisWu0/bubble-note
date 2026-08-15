package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/norriswu0/bubble-note/internal/notes"
)

const (
	manifestName = "manifest.json"
	contentName  = "README.md"
)

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// manifest is the metadata sidecar persisted next to each note's markdown body.
type manifest struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// Store reads and writes notes as directories of markdown files. It is the
// source of truth; the SQLite index is derived from these files.
type Store struct {
	root string
}

func New(root string) *Store { return &Store{root: root} }

func (s *Store) Root() string { return s.root }

// Ensure creates the notes directory if it does not exist.
func (s *Store) Ensure() error { return os.MkdirAll(s.root, 0o755) }

// Slug derives a directory name from a title. If a directory already exists and
// belongs to a different note, a numeric suffix is appended for uniqueness.
func (s *Store) Slug(title, excludeID string) string {
	base := sanitize(title)
	if base == "" {
		base = "note"
	}
	slug := base
	for n := 2; ; n++ {
		id, err := manifestID(filepath.Join(s.root, slug))
		if err != nil || id == excludeID {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, n)
	}
}

// Exists reports whether a note directory already exists.
func (s *Store) Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(s.root, dir))
	return err == nil
}

// Path returns the full path to a note's markdown body.
func (s *Store) Path(dir string) string {
	return filepath.Join(s.root, dir, contentName)
}

// Write persists a note's body and metadata into the given directory.
func (s *Store) Write(dir string, note notes.Note) error {
	path := filepath.Join(s.root, dir)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create note directory: %w", err)
	}
	m := manifest{
		ID:        note.ID,
		Title:     note.Title,
		Tags:      notes.NormalizeTags(note.Tags),
		CreatedAt: note.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: note.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if err := writeJSON(filepath.Join(path, manifestName), m); err != nil {
		return err
	}
	return writeFile(filepath.Join(path, contentName), []byte(note.Content))
}

// Rename moves a note directory, handling title changes.
func (s *Store) Rename(oldDir, newDir string) error {
	if oldDir == newDir {
		return nil
	}
	return os.Rename(filepath.Join(s.root, oldDir), filepath.Join(s.root, newDir))
}

// Remove deletes a note directory.
func (s *Store) Remove(dir string) error {
	return os.RemoveAll(filepath.Join(s.root, dir))
}

// Read loads a note from its directory.
func (s *Store) Read(dir string) (notes.Note, error) {
	m, err := readManifest(filepath.Join(s.root, dir))
	if err != nil {
		return notes.Note{}, err
	}
	content, err := os.ReadFile(filepath.Join(s.root, dir, contentName))
	if err != nil {
		return notes.Note{}, fmt.Errorf("read note content: %w", err)
	}
	created, err := time.Parse(time.RFC3339Nano, m.CreatedAt)
	if err != nil {
		return notes.Note{}, err
	}
	updated, err := time.Parse(time.RFC3339Nano, m.UpdatedAt)
	if err != nil {
		return notes.Note{}, err
	}
	return notes.Note{
		ID:        m.ID,
		Title:     m.Title,
		Content:   string(content),
		Tags:      m.Tags,
		CreatedAt: created,
		UpdatedAt: updated,
	}, nil
}

// Scan walks the notes directory and returns every note found, newest first.
// Directories without a manifest are ignored (for example .git).
func (s *Store) Scan() ([]notes.FileNote, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var result []notes.FileNote
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		note, err := s.Read(entry.Name())
		if err != nil {
			continue
		}
		result = append(result, notes.FileNote{Note: note, Dir: entry.Name()})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Note.UpdatedAt.After(result[j].Note.UpdatedAt)
	})
	return result, nil
}

func sanitize(title string) string {
	lower := strings.ToLower(strings.TrimSpace(title))
	slug := slugPattern.ReplaceAllString(lower, "-")
	slug = strings.Trim(slug, "-")
	return slug
}

func manifestID(dir string) (string, error) {
	m, err := readManifest(dir)
	if err != nil {
		return "", err
	}
	return m.ID, nil
}

func readManifest(dir string) (manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return manifest{}, err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, err
	}
	return m, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append(data, '\n'))
}

func writeFile(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
