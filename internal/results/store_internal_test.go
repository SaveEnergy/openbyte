package results

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestNewConfiguresSQLitePragmasAcrossPool(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "pragma.db"), 10)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	conns := make([]*sql.Conn, 0, sqliteMaxOpenConns)
	for i := 0; i < sqliteMaxOpenConns; i++ {
		conn, err := store.db.Conn(ctx)
		if err != nil {
			t.Fatalf("db.Conn(%d): %v", i, err)
		}
		conns = append(conns, conn)
	}
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()

	for i, conn := range conns {
		var synchronous int
		if err := conn.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil {
			t.Fatalf("conn %d synchronous: %v", i, err)
		}
		if synchronous != 1 {
			t.Fatalf("conn %d PRAGMA synchronous = %d, want 1 (NORMAL)", i, synchronous)
		}

		var busyTimeout int
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatalf("conn %d busy_timeout: %v", i, err)
		}
		if busyTimeout != sqliteBusyTimeout {
			t.Fatalf("conn %d PRAGMA busy_timeout = %d, want %d", i, busyTimeout, sqliteBusyTimeout)
		}
	}
}
