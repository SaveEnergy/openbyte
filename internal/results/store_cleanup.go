package results

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"
)

const (
	retentionDays   = 90
	cleanupInterval = 1 * time.Hour
)

func (s *Store) cleanup() {
	ctx := context.Background()
	cutoff := time.Now().UTC().Add(-retentionDays * 24 * time.Hour)
	res, err := execWithBusyRetry(ctx, s.db, `DELETE FROM results WHERE created_at < ?`, cutoff)
	if err != nil {
		slog.Warn("results cleanup (age) failed", "error", err)
	} else {
		s.logCleanupCount("results cleanup: removed expired", "count", res)
	}

	// Trim to max count, keeping newest
	if s.maxResults > 0 {
		res, err = execWithBusyRetry(
			ctx,
			s.db,
			`DELETE FROM results
			WHERE id IN (
				SELECT id FROM results
				ORDER BY created_at DESC, id DESC
				LIMIT -1 OFFSET ?
			)`, s.maxResults)
		if err != nil {
			slog.Warn("results cleanup (count) failed", "error", err)
		} else {
			n, rowsErr := res.RowsAffected()
			if rowsErr != nil {
				slog.Warn("results cleanup (count): rows affected failed", "error", rowsErr)
			} else if n > 0 {
				slog.Info("results cleanup: trimmed to max", "removed", n, "max", s.maxResults)
			}
		}
	}
}

func execWithBusyRetry(
	ctx context.Context,
	execer execContexter,
	query string,
	args ...any,
) (sql.Result, error) {
	var (
		res          sql.Result
		err          error
		busyDeadline = time.Now().Add(busyRetryBudget)
	)
	for busyAttempt := 0; ; busyAttempt++ {
		res, err = execer.ExecContext(ctx, query, args...)
		if err == nil {
			return res, nil
		}
		if isBusyError(err) {
			if waitErr := waitForBusyRetry(ctx, busyDeadline, busyAttempt); waitErr != nil {
				if errors.Is(waitErr, errBusyRetryBudget) {
					return nil, err
				}
				return nil, waitErr
			}
			continue
		}
		return nil, err
	}
}

func (s *Store) logCleanupCount(msg string, field string, res sql.Result) {
	n, rowsErr := res.RowsAffected()
	if rowsErr != nil {
		slog.Warn(msg+": rows affected failed", "error", rowsErr)
		return
	}
	if n > 0 {
		slog.Info(msg, field, n)
	}
}

func (s *Store) cleanupLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}
