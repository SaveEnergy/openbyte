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

		insertErr := s.insertResultWithRetry(ctx, id, r, now, busyDeadline)
		if insertErr == nil {
			return id, nil
		}
		if isUniqueViolation(insertErr) {
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
) error {
	err := retryBusy(ctx, busyDeadline, func() error {
		_, err := s.db.ExecContext(
			ctx,
			`INSERT INTO results (id, download_mbps, upload_mbps, latency_ms, jitter_ms,
				loaded_latency_ms, bufferbloat_grade, ipv4, ipv6, server_name, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, r.DownloadMbps, r.UploadMbps, r.LatencyMs, r.JitterMs,
			r.LoadedLatencyMs, r.BufferbloatGrade, r.IPv4, r.IPv6, r.ServerName,
			now,
		)
		return err
	})
	if err != nil {
		return fmt.Errorf("insert result: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		code := sqliteErr.Code()
		return code == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY || code == sqlite3.SQLITE_CONSTRAINT_UNIQUE
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
	var r Result
	err := retryBusy(ctx, time.Now().Add(busyRetryBudget), func() error {
		return s.db.QueryRowContext(
			ctx,
			`SELECT id, download_mbps, upload_mbps, latency_ms, jitter_ms,
				loaded_latency_ms, bufferbloat_grade, ipv4, ipv6, server_name, created_at
			FROM results WHERE id = ?`, id,
		).Scan(&r.ID, &r.DownloadMbps, &r.UploadMbps, &r.LatencyMs, &r.JitterMs,
			&r.LoadedLatencyMs, &r.BufferbloatGrade, &r.IPv4, &r.IPv6, &r.ServerName,
			&r.CreatedAt)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query result: %w", err)
	}
	return &r, nil
}
