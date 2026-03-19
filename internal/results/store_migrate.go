package results

import (
	"context"
	"database/sql"
	"fmt"
)

const (
	sqliteMaxOpenConns = 3
	sqliteBusyTimeout  = 5000
)

type execContexter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
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

func configureSQLitePool(db *sql.DB, count int) error {
	ctx := context.Background()
	conns := make([]*sql.Conn, 0, count)
	defer func() {
		for _, conn := range conns {
			if conn != nil {
				_ = conn.Close()
			}
		}
	}()
	for i := 0; i < count; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			return fmt.Errorf("open sqlite connection %d: %w", i+1, err)
		}
		conns = append(conns, conn)
		if err := configureSQLitePragmas(ctx, conn); err != nil {
			return err
		}
	}
	return nil
}

func configureSQLitePragmas(ctx context.Context, execer execContexter) error {
	if _, err := execer.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := execer.ExecContext(ctx, "PRAGMA synchronous=NORMAL"); err != nil {
		return fmt.Errorf("set synchronous mode: %w", err)
	}
	if _, err := execer.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d", sqliteBusyTimeout)); err != nil {
		return fmt.Errorf("set busy_timeout: %w", err)
	}
	return nil
}
