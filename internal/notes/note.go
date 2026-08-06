package notes

import "time"

// Note is the business representation of a note and its current revision.
type Note struct {
	ID            string
	Title         string
	Content       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CurrentRevID  string
	Tags          []string
	RevisionCount int
	SyncStatus    SyncStatus
}

type SyncStatus string

const (
	SyncLocalOnly  SyncStatus = "local-only"
	SyncSynced     SyncStatus = "synced"
	SyncConflicted SyncStatus = "conflicted"
)

type Revision struct {
	ID        string
	NoteID    string
	Title     string
	Content   string
	CreatedAt time.Time
}

type SyncSnapshot struct {
	Note      Note
	Revisions []Revision
}

type Filter struct {
	Query   string
	Tag     string
	From    *time.Time
	Through *time.Time
}

type Repository interface {
	CreateNote(title, content string, tags []string) (Note, error)
	GetNote(id string) (Note, error)
	ListNotes(filter Filter) ([]Note, error)
	SaveNote(id, title, content string, tags []string) (Note, error)
	DeleteNote(id string) error
	ListRevisions(noteID string) ([]Revision, error)
	RestoreRevision(noteID, revisionID string) (Note, error)
}
