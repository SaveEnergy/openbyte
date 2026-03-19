package results

import (
	"database/sql"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
)

const (
	retentionDays   = 90
	cleanupInterval = 1 * time.Hour
)

func (s *Store) cleanup() {
	cutoff := time.Now().UTC().Add(-retentionDays * 24 * time.Hour)
	res, err := s.execWithBusyRetry(`DELETE FROM results WHERE created_at < ?`, cutoff)
	if err != nil {
		logging.Warn("results cleanup (age) failed", logging.Field{Key: "error", Value: err})
	} else {
		s.logCleanupCount("results cleanup: removed expired", "count", res)
	}

	// Trim to max count, keeping newest
	if s.maxResults > 0 {
		res, err = s.execWithBusyRetry(
			`DELETE FROM results
			WHERE id IN (
				SELECT id FROM results
				ORDER BY created_at DESC, id DESC
				LIMIT -1 OFFSET ?
			)`, s.maxResults)
		if err != nil {
			logging.Warn("results cleanup (count) failed", logging.Field{Key: "error", Value: err})
		} else {
			n, rowsErr := res.RowsAffected()
			if rowsErr != nil {
				logging.Warn("results cleanup (count): rows affected failed", logging.Field{Key: "error", Value: rowsErr})
			} else if n > 0 {
				logging.Info("results cleanup: trimmed to max",
					logging.Field{Key: "removed", Value: n},
					logging.Field{Key: "max", Value: s.maxResults})
			}
		}
	}
}

func (s *Store) execWithBusyRetry(query string, args ...any) (sql.Result, error) {
	var (
		res sql.Result
		err error
	)
	for busyAttempt := 0; busyAttempt <= maxBusyRetries; busyAttempt++ {
		res, err = s.db.Exec(query, args...)
		if err == nil {
			return res, nil
		}
		if isBusyError(err) && busyAttempt < maxBusyRetries {
			time.Sleep(time.Duration(busyAttempt+1) * busyRetryBackoff)
			continue
		}
		return nil, err
	}
	panic("unreachable")
}

func (s *Store) logCleanupCount(msg string, field string, res sql.Result) {
	n, rowsErr := res.RowsAffected()
	if rowsErr != nil {
		logging.Warn(msg+": rows affected failed", logging.Field{Key: "error", Value: rowsErr})
		return
	}
	if n > 0 {
		logging.Info(msg, logging.Field{Key: field, Value: n})
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
