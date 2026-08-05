package domain

import "time"

type Note struct {
	ID            string
	Title         string
	Content       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CurrentRevID  string
	Tags          []string
	RevisionCount int
}

type Revision struct {
	ID        string
	NoteID    string
	Title     string
	Content   string
	CreatedAt time.Time
}

type NoteFilter struct {
	Query   string
	Tag     string
	From    *time.Time
	Through *time.Time
}

type NoteStore interface {
	CreateNote(title, content string, tags []string) (Note, error)
	GetNote(id string) (Note, error)
	ListNotes(filter NoteFilter) ([]Note, error)
	SaveNote(id, title, content string, tags []string) (Note, error)
	DeleteNote(id string) error
	ListRevisions(noteID string) ([]Revision, error)
	RestoreRevision(noteID, revisionID string) (Note, error)
	Close() error
}
