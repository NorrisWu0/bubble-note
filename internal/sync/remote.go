package sync

import (
	"context"

	"github.com/norriswu0/bubble-note/internal/domain"
)

// RemoteStore is reserved for optional S3-compatible synchronization.
// Local operation must never depend on an implementation of this interface.
type RemoteStore interface {
	Pull(ctx context.Context) ([]domain.Revision, error)
	Push(ctx context.Context, revisions []domain.Revision) error
}
