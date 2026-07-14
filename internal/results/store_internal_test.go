package results

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestStoreOperationsHonorContextWhileWaitingForConnection(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "context.db"), 10)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer store.Close()

	heldConns := make([]*sql.Conn, 0, sqliteMaxOpenConns)
	for i := 0; i < sqliteMaxOpenConns; i++ {
		conn, connErr := store.db.Conn(context.Background())
		if connErr != nil {
			t.Fatalf("db.Conn(%d): %v", i, connErr)
		}
		heldConns = append(heldConns, conn)
	}
	defer func() {
		for _, conn := range heldConns {
			_ = conn.Close()
		}
	}()

	tests := []struct {
		name string
		run  func(context.Context) error
	}{
		{
			name: "save",
			run: func(ctx context.Context) error {
				_, saveErr := store.Save(ctx, Result{DownloadMbps: 1})
				return saveErr
			},
		},
		{
			name: "get",
			run: func(ctx context.Context) error {
				_, getErr := store.Get(ctx, "abcd1234")
				return getErr
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			errCh := make(chan error, 1)
			go func() {
				errCh <- tt.run(ctx)
			}()

			select {
			case err := <-errCh:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("error = %v, want context deadline exceeded", err)
				}
			case <-time.After(time.Second):
				t.Fatal("operation did not stop after context deadline")
			}
		})
	}
}

func TestBusyRetryWaitHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := waitForBusyRetry(ctx, time.Now().Add(time.Hour), 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForBusyRetry error = %v, want context canceled", err)
	}
}

func TestBusyRetryWaitReportsExpiredBudget(t *testing.T) {
	err := waitForBusyRetry(context.Background(), time.Now().Add(-time.Nanosecond), 0)
	if !errors.Is(err, errBusyRetryBudget) {
		t.Fatalf("waitForBusyRetry error = %v, want busy retry budget exhausted", err)
	}
}

func TestBusyRetryDelayIsCapped(t *testing.T) {
	if delay := busyRetryDelay(100); delay != busyRetryMaxBackoff {
		t.Fatalf("busyRetryDelay(100) = %v, want %v", delay, busyRetryMaxBackoff)
	}
}

func TestMigrateRetriesBusyError(t *testing.T) {
	execer := &busyOnceExecer{busyQuery: "CREATE TABLE"}

	if err := migrate(execer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(execer.queries) != 3 {
		t.Fatalf("ExecContext calls = %d, want 3", len(execer.queries))
	}
	if execer.queries[0] != execer.queries[1] {
		t.Fatal("migration did not retry the busy statement")
	}
}

func TestConfigureSQLitePragmasRetriesBusyAfterDisablingDriverWait(t *testing.T) {
	execer := &busyOnceExecer{busyQuery: "PRAGMA journal_mode=WAL"}

	if err := configureSQLitePragmas(context.Background(), execer); err != nil {
		t.Fatalf("configureSQLitePragmas: %v", err)
	}
	wantQueries := []string{
		"PRAGMA busy_timeout=0",
		"PRAGMA journal_mode=WAL",
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
	}
	if len(execer.queries) != len(wantQueries) {
		t.Fatalf("ExecContext calls = %d, want %d", len(execer.queries), len(wantQueries))
	}
	for i, want := range wantQueries {
		if got := execer.queries[i]; got != want {
			t.Fatalf("query %d = %q, want %q", i, got, want)
		}
	}
}

type busyOnceExecer struct {
	busyQuery    string
	busyReturned bool
	queries      []string
}

func (e *busyOnceExecer) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	e.queries = append(e.queries, query)
	if !e.busyReturned && strings.Contains(query, e.busyQuery) {
		e.busyReturned = true
		return nil, errors.New("database is locked")
	}
	return nil, nil
}
