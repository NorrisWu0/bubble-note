package storage

import (
	"context"
	"errors"

	"github.com/norriswu0/bubble-note/internal/notes"
)

var ErrConflict = errors.New("remote note conflict")

type State int

const (
	NotConfigured State = iota
	Checking
	Connected
	Unauthorized
	Unavailable
)

// ConnectionChecker is the first seam for optional remote storage.
// Local operation must never depend on an implementation of this interface.
type ConnectionChecker interface {
	CheckConnection(ctx context.Context) error
}

type NoteSyncer interface {
	SyncNote(ctx context.Context, note notes.Note, revisions []notes.Revision, expectedETag string) (etag string, err error)
	PullNote(ctx context.Context, noteID string) (notes.SyncSnapshot, string, error)
	DeleteNote(ctx context.Context, noteID string) error
}
