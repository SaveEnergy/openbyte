package results

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	busyRetryInitialBackoff = 25 * time.Millisecond
	busyRetryMaxBackoff     = 250 * time.Millisecond
	// Preserve the previous tolerance of four SQLite attempts at five seconds each.
	busyRetryBudget = 20 * time.Second
)

var errBusyRetryBudget = errors.New("busy retry budget exhausted")

func (s *Store) Save(ctx context.Context, r Result) (string, error) {
	now := time.Now().UTC()
	busyDeadline := time.Now().Add(busyRetryBudget)
	for range maxIDRetries {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		id, err := generateID()
		if err != nil {
			return "", fmt.Errorf("generate id: %w", err)
		}

		uniqueConflict, insertErr := s.insertResultWithRetry(ctx, id, r, now, busyDeadline)
		if insertErr == nil {
			return id, nil
		}
		if uniqueConflict {
			continue
		}
		return "", insertErr
	}
	return "", fmt.Errorf("failed to generate unique ID after %d attempts", maxIDRetries)
}

func (s *Store) insertResultWithRetry(
	ctx context.Context,
	id string,
	r Result,
	now time.Time,
	busyDeadline time.Time,
) (uniqueConflict bool, err error) {
	for busyAttempt := 0; ; busyAttempt++ {
		_, err = s.db.ExecContext(
			ctx,
			`INSERT INTO results (id, download_mbps, upload_mbps, latency_ms, jitter_ms,
				loaded_latency_ms, bufferbloat_grade, ipv4, ipv6, server_name, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, r.DownloadMbps, r.UploadMbps, r.LatencyMs, r.JitterMs,
			r.LoadedLatencyMs, r.BufferbloatGrade, r.IPv4, r.IPv6, r.ServerName,
			now,
		)
		if err == nil {
			return false, nil
		}
		if isUniqueViolation(err) {
			return true, nil
		}
		if isBusyError(err) {
			if waitErr := waitForBusyRetry(ctx, busyDeadline, busyAttempt); waitErr != nil {
				if errors.Is(waitErr, errBusyRetryBudget) {
					return false, fmt.Errorf("%w: insert result: %w", ErrStoreRetryable, err)
				}
				return false, fmt.Errorf("insert result: %w", waitErr)
			}
			continue
		}
		return false, fmt.Errorf("insert result: %w", err)
	}
}

func isUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
	}
	return strings.Contains(err.Error(), "UNIQUE constraint")
}

func isBusyError(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		code := sqliteErr.Code()
		return code == sqlite3.SQLITE_BUSY || code == sqlite3.SQLITE_LOCKED
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "sqlite_busy")
}

func (s *Store) Get(ctx context.Context, id string) (*Result, error) {
	busyDeadline := time.Now().Add(busyRetryBudget)
	for busyAttempt := 0; ; busyAttempt++ {
		var r Result
		err := s.db.QueryRowContext(
			ctx,
			`SELECT id, download_mbps, upload_mbps, latency_ms, jitter_ms,
				loaded_latency_ms, bufferbloat_grade, ipv4, ipv6, server_name, created_at
			FROM results WHERE id = ?`, id,
		).Scan(&r.ID, &r.DownloadMbps, &r.UploadMbps, &r.LatencyMs, &r.JitterMs,
			&r.LoadedLatencyMs, &r.BufferbloatGrade, &r.IPv4, &r.IPv6, &r.ServerName,
			&r.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err == nil {
			return &r, nil
		}
		if isBusyError(err) {
			if waitErr := waitForBusyRetry(ctx, busyDeadline, busyAttempt); waitErr != nil {
				if errors.Is(waitErr, errBusyRetryBudget) {
					return nil, fmt.Errorf("%w: query result: %w", ErrStoreRetryable, err)
				}
				return nil, fmt.Errorf("query result: %w", waitErr)
			}
			continue
		}
		return nil, fmt.Errorf("query result: %w", err)
	}
}

func waitForBusyRetry(ctx context.Context, deadline time.Time, busyAttempt int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	remaining := time.Until(deadline)
	if remaining <= 0 {
		return errBusyRetryBudget
	}

	delay := busyRetryDelay(busyAttempt)
	budgetExpires := delay >= remaining
	if budgetExpires {
		delay = remaining
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		if err := ctx.Err(); err != nil {
			return err
		}
		if budgetExpires {
			return errBusyRetryBudget
		}
		return nil
	}
}

func busyRetryDelay(busyAttempt int) time.Duration {
	if busyAttempt >= 4 {
		return busyRetryMaxBackoff
	}
	return busyRetryInitialBackoff << busyAttempt
}
