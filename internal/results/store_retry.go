package results

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	busyRetryInitialBackoff = 25 * time.Millisecond
	busyRetryMaxBackoff     = 250 * time.Millisecond
	// Preserve the previous tolerance of four SQLite attempts at five seconds each.
	busyRetryBudget = 20 * time.Second
)

var errBusyRetryBudget = errors.New("busy retry budget exhausted")

// retryBusy owns the retry policy for reads, writes, and maintenance. The caller
// supplies the deadline so ID collisions cannot restart a save's busy budget.
func retryBusy(ctx context.Context, deadline time.Time, operation func() error) error {
	for attempt := 0; ; attempt++ {
		err := operation()
		if err == nil || !isBusyError(err) {
			return err
		}
		if waitErr := waitForBusyRetry(ctx, deadline, attempt); waitErr != nil {
			if errors.Is(waitErr, errBusyRetryBudget) {
				return fmt.Errorf("%w: %w", ErrStoreRetryable, err)
			}
			return waitErr
		}
	}
}

func execWithBusyRetry(ctx context.Context, execer execContexter, query string, args ...any) (sql.Result, error) {
	var res sql.Result
	err := retryBusy(ctx, time.Now().Add(busyRetryBudget), func() error {
		var err error
		res, err = execer.ExecContext(ctx, query, args...)
		return err
	})
	if err != nil {
		return nil, err
	}
	return res, nil
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
