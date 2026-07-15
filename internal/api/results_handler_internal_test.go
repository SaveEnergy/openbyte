package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/saveenergy/openbyte/internal/results"
)

func TestValidResultID(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		id   string
		want bool
	}{
		{id: "a1b2c3d4", want: true},
		{id: "ABCD1234", want: true},
		{id: "a1b2c3d", want: false},
		{id: "a1b2c3d!", want: false},
		{id: "a1b2c3ä4", want: false},
	} {
		if got := validResultID(test.id); got != test.want {
			t.Errorf("validResultID(%q) = %t, want %t", test.id, got, test.want)
		}
	}
}

func TestRetryableStoreErrorsMapToServiceUnavailable(t *testing.T) {
	err := errors.Join(results.ErrStoreRetryable, errors.New("sqlite busy"))
	for name, mapError := range map[string]func(error) (string, int){
		"get":  mapGetStoreError,
		"save": mapSaveStoreError,
	} {
		t.Run(name, func(t *testing.T) {
			message, status := mapError(err)
			if message != "store temporarily unavailable" {
				t.Fatalf("message = %q, want store temporarily unavailable", message)
			}
			if status != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d", status, http.StatusServiceUnavailable)
			}
		})
	}
}
