package storage

import (
	"context"

	"github.com/norriswu0/bubble-note/internal/notes"
)

// RemoteStore is reserved for optional S3-compatible synchronization.
// Local operation must never depend on an implementation of this interface.
type RemoteStore interface {
	Pull(ctx context.Context) ([]notes.Revision, error)
	Push(ctx context.Context, revisions []notes.Revision) error
}
