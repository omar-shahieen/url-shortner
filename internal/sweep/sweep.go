// Package sweep provides a background goroutine that periodically purges
// expired URLs from the SQLite database.
//
// Lazy expiry-on-read (handled by the service layer) already guarantees
// correctness — expired URLs are never served. This sweep is storage hygiene:
// it reclaims disk space and keeps the table small so bloom-filter rebuild
// times stay low.
package sweep

import (
	"context"
	"log/slog"
	"time"
)

// Purger is implemented by any repository that can delete its own expired rows.
type Purger interface {
	DeleteExpired(ctx context.Context) (int64, error)
}

// Run starts a background goroutine that calls p.DeleteExpired on every tick
// of interval. It exits cleanly when ctx is cancelled.
// The caller should call this in a goroutine: go sweep.Run(ctx, repo, ...).
func Run(ctx context.Context, p Purger, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("expiry sweep: stopping", slog.String("reason", ctx.Err().Error()))
			return
		case <-ticker.C:
			n, err := p.DeleteExpired(ctx)
			if err != nil {
				logger.Error("expiry sweep: delete failed", slog.Any("error", err))
				continue
			}
			if n > 0 {
				logger.Info("expiry sweep: purged expired URLs", slog.Int64("count", n))
			}
		}
	}
}
