package registry

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
)

func TestClientDeregisterStatusCheck(t *testing.T) {
	tests := []struct {
		name                string
		status              int
		wantRegisteredAfter bool
	}{
		{name: "ok", status: http.StatusOK, wantRegisteredAfter: false},
		{name: "not found", status: http.StatusNotFound, wantRegisteredAfter: false},
		{name: "server error", status: http.StatusInternalServerError, wantRegisteredAfter: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			cfg := config.DefaultConfig()
			cfg.RegistryEnabled = true
			cfg.RegistryURL = srv.URL
			cfg.RegistryInterval = 10 * time.Second
			cfg.ServerID = "srv-test"

			c := NewClient(cfg, logging.NewLogger("test"))
			c.mu.Lock()
			c.registered = true
			c.mu.Unlock()

			c.deregister()

			c.mu.RLock()
			got := c.registered
			c.mu.RUnlock()
			if got != tc.wantRegisteredAfter {
				t.Fatalf("registered after deregister = %v, want %v", got, tc.wantRegisteredAfter)
			}
		})
	}
}
