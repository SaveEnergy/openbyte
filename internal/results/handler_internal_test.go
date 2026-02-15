package results

import (
	"errors"
	"net/http"
	"testing"
)

func TestMapGetStoreError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantMsg    string
		wantStatus int
	}{
		{
			name:       "retryable error maps to 503",
			err:        errors.Join(ErrStoreRetryable, errors.New("sqlite busy")),
			wantMsg:    "store temporarily unavailable",
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "internal error maps to 500",
			err:        errors.New("db closed"),
			wantMsg:    "internal error",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, gotStatus := mapGetStoreError(tt.err)
			if gotMsg != tt.wantMsg {
				t.Fatalf("message = %q, want %q", gotMsg, tt.wantMsg)
			}
			if gotStatus != tt.wantStatus {
				t.Fatalf("status = %d, want %d", gotStatus, tt.wantStatus)
			}
		})
	}
}
