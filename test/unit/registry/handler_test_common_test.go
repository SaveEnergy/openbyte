package registry_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/internal/registry"
)

const (
	registryServersPath  = "/api/v1/registry/servers"
	registryServerS1Path = "/api/v1/registry/servers/s1"
	registryMissingPath  = "/api/v1/registry/servers/missing"
	registryHealthPath   = "/api/v1/registry/health"
	registryHealthyQuery = "/api/v1/registry/servers?healthy=true"
	contentTypeHeader    = "Content-Type"
	applicationJSON      = "application/json"
	textPlainType        = "text/plain"
	statusCodeWantFmt    = "status = %d, want %d"
	methodGet            = "GET"
	methodPost           = "POST"
	methodPut            = "PUT"
	methodDelete         = "DELETE"
	serverBodyS1         = `{"id":"s1","name":"Test","host":"localhost"}`
)

func setupHandler(apiKey string) (*registry.Handler, *http.ServeMux) {
	svc := registry.NewService(30*time.Second, 10*time.Second)
	logger := logging.NewLogger("test")
	h := registry.NewHandler(svc, logger, apiKey)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return h, mux
}

func mustCountField(t *testing.T, resp map[string]any) float64 {
	t.Helper()
	raw, ok := resp["count"]
	if !ok {
		t.Fatalf("response missing count field")
	}
	count, ok := raw.(float64)
	if !ok {
		t.Fatalf("count field has invalid type %T: %v", raw, raw)
	}
	return count
}
