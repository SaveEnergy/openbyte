package results

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Registers sqlite driver used by sql.Open("sqlite", ...).

	"github.com/saveenergy/openbyte/internal/logging"
)

var ErrStoreRetryable = errors.New("results store retryable")

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

	db.SetMaxOpenConns(sqliteMaxOpenConns)
	db.SetMaxIdleConns(sqliteMaxOpenConns)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := configureSQLitePool(db, sqliteMaxOpenConns); err != nil {
		db.Close()
		return nil, err
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
