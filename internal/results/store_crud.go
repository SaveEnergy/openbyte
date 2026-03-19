package results

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	maxBusyRetries   = 3
	busyRetryBackoff = 25 * time.Millisecond
)

func (s *Store) Save(r Result) (string, error) {
	now := time.Now().UTC()
	for range maxIDRetries {
		id, err := generateID()
		if err != nil {
			return "", fmt.Errorf("generate id: %w", err)
		}

		uniqueConflict, insertErr := s.insertResultWithRetry(id, r, now)
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

func (s *Store) insertResultWithRetry(id string, r Result, now time.Time) (uniqueConflict bool, err error) {
	for busyAttempt := 0; busyAttempt <= maxBusyRetries; busyAttempt++ {
		_, err = s.db.Exec(
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
		if isBusyError(err) && busyAttempt < maxBusyRetries {
			time.Sleep(time.Duration(busyAttempt+1) * busyRetryBackoff)
			continue
		}
		if isBusyError(err) {
			return false, fmt.Errorf("%w: insert result: %w", ErrStoreRetryable, err)
		}
		return false, fmt.Errorf("insert result: %w", err)
	}
	panic("unreachable")
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

func (s *Store) Get(id string) (*Result, error) {
	for busyAttempt := 0; busyAttempt <= maxBusyRetries; busyAttempt++ {
		var r Result
		err := s.db.QueryRow(
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
		if isBusyError(err) && busyAttempt < maxBusyRetries {
			time.Sleep(time.Duration(busyAttempt+1) * busyRetryBackoff)
			continue
		}
		if isBusyError(err) {
			return nil, fmt.Errorf("%w: query result: %w", ErrStoreRetryable, err)
		}
		return nil, fmt.Errorf("query result: %w", err)
	}
	panic("unreachable")
}
