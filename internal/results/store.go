package results

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/saveenergy/openbyte/internal/logging"
)

const (
	retentionDays   = 90
	cleanupInterval = 1 * time.Hour
	idLength        = 8
	idCharset       = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

type Result struct {
	ID               string    `json:"id"`
	DownloadMbps     float64   `json:"download_mbps"`
	UploadMbps       float64   `json:"upload_mbps"`
	LatencyMs        float64   `json:"latency_ms"`
	JitterMs         float64   `json:"jitter_ms"`
	LoadedLatencyMs  float64   `json:"loaded_latency_ms"`
	BufferbloatGrade string    `json:"bufferbloat_grade"`
	IPv4             string    `json:"ipv4"`
	IPv6             string    `json:"ipv6"`
	ServerName       string    `json:"server_name"`
	CreatedAt        time.Time `json:"created_at"`
}

type Store struct {
	db         *sql.DB
	maxResults int
	stopCh     chan struct{}
	wg         sync.WaitGroup
	closeOnce  sync.Once
}

func New(dbPath string, maxResults int) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(2)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	// modernc.org/sqlite requires explicit PRAGMAs (not query-string params)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	s := &Store{
		db:         db,
		maxResults: maxResults,
		stopCh:     make(chan struct{}),
	}

	s.cleanup()

	s.wg.Add(1)
	go s.cleanupLoop()

	return s, nil
}

func (s *Store) Close() {
	s.closeOnce.Do(func() {
		close(s.stopCh)
		s.wg.Wait()
		if err := s.db.Close(); err != nil {
			logging.Warn("results store: close failed", logging.Field{Key: "error", Value: err})
		}
	})
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS results (
		id TEXT PRIMARY KEY,
		download_mbps REAL NOT NULL,
		upload_mbps REAL NOT NULL,
		latency_ms REAL NOT NULL,
		jitter_ms REAL NOT NULL,
		loaded_latency_ms REAL NOT NULL DEFAULT 0,
		bufferbloat_grade TEXT NOT NULL DEFAULT '',
		ipv4 TEXT NOT NULL DEFAULT '',
		ipv6 TEXT NOT NULL DEFAULT '',
		server_name TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_results_created_at ON results(created_at)`)
	return err
}

const maxIDRetries = 5

func (s *Store) Save(r Result) (string, error) {
	now := time.Now().UTC()
	for attempt := 0; attempt < maxIDRetries; attempt++ {
		id, err := generateID()
		if err != nil {
			return "", fmt.Errorf("generate id: %w", err)
		}

		_, err = s.db.Exec(
			`INSERT INTO results (id, download_mbps, upload_mbps, latency_ms, jitter_ms,
				loaded_latency_ms, bufferbloat_grade, ipv4, ipv6, server_name, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, r.DownloadMbps, r.UploadMbps, r.LatencyMs, r.JitterMs,
			r.LoadedLatencyMs, r.BufferbloatGrade, r.IPv4, r.IPv6, r.ServerName,
			now,
		)
		if err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return "", fmt.Errorf("insert result: %w", err)
		}
		return id, nil
	}
	return "", fmt.Errorf("failed to generate unique ID after %d attempts", maxIDRetries)
}

func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint")
}

func (s *Store) Get(id string) (*Result, error) {
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
	if err != nil {
		return nil, fmt.Errorf("query result: %w", err)
	}
	return &r, nil
}

func (s *Store) cleanup() {
	cutoff := time.Now().UTC().Add(-retentionDays * 24 * time.Hour)
	res, err := s.db.Exec(`DELETE FROM results WHERE created_at < ?`, cutoff)
	if err != nil {
		logging.Warn("results cleanup (age) failed", logging.Field{Key: "error", Value: err})
	} else if n, _ := res.RowsAffected(); n > 0 {
		logging.Info("results cleanup: removed expired",
			logging.Field{Key: "count", Value: n})
	}

	// Trim to max count, keeping newest
	if s.maxResults > 0 {
		res, err = s.db.Exec(
			`DELETE FROM results WHERE id NOT IN (
				SELECT id FROM results ORDER BY created_at DESC LIMIT ?
			)`, s.maxResults)
		if err != nil {
			logging.Warn("results cleanup (count) failed", logging.Field{Key: "error", Value: err})
		} else if n, _ := res.RowsAffected(); n > 0 {
			logging.Info("results cleanup: trimmed to max",
				logging.Field{Key: "removed", Value: n},
				logging.Field{Key: "max", Value: s.maxResults})
		}
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

func generateID() (string, error) {
	var entropy [idLength]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	b := make([]byte, idLength)
	for i, v := range entropy {
		b[i] = idCharset[int(v)%len(idCharset)]
	}
	return string(b), nil
}
