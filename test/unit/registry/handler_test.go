package registry_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/internal/registry"
)

func setupHandler(apiKey string) (*registry.Handler, *http.ServeMux) {
	svc := registry.NewService(30*time.Second, 10*time.Second)
	logger := logging.NewLogger("test")
	h := registry.NewHandler(svc, logger, apiKey)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return h, mux
}

func mustCountField(t *testing.T, resp map[string]interface{}) float64 {
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

func TestHandlerRegisterAndList(t *testing.T) {
	_, mux := setupHandler("")

	// Register
	body := `{"id":"s1","name":"Test Server","host":"localhost","tcp_port":8081,"udp_port":8082,"health":"healthy"}`
	req := httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// List
	req = httptest.NewRequest("GET", "/api/v1/registry/servers", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if got := mustCountField(t, resp); got != 1 {
		t.Errorf("count = %.0f, want 1", got)
	}
}

func TestHandlerGetServer(t *testing.T) {
	_, mux := setupHandler("")

	// Register first
	body := `{"id":"s1","name":"Test","host":"localhost"}`
	req := httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Get
	req = httptest.NewRequest("GET", "/api/v1/registry/servers/s1", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandlerGetServerNotFound(t *testing.T) {
	_, mux := setupHandler("")

	req := httptest.NewRequest("GET", "/api/v1/registry/servers/missing", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlerAuthRequired(t *testing.T) {
	_, mux := setupHandler("secret-key")

	// Register without auth → 401
	body := `{"id":"s1","name":"Test","host":"localhost"}`
	req := httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no auth: status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Register with correct auth → 201
	req = httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-key")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("with auth: status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestHandlerAuthWrongKey(t *testing.T) {
	_, mux := setupHandler("correct-key")

	body := `{"id":"s1","name":"Test","host":"localhost"}`
	req := httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-key")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key: status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandlerAuthMalformedBearerHeader(t *testing.T) {
	_, mux := setupHandler("secret-key")

	for _, auth := range []string{"Bearer", "Bearer ", "Bear secret-key"} {
		body := `{"id":"s1","name":"Test","host":"localhost"}`
		req := httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", auth)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("auth=%q: status = %d, want %d", auth, rec.Code, http.StatusUnauthorized)
		}
	}
}

func TestHandlerUpdateServer(t *testing.T) {
	_, mux := setupHandler("")

	// Register
	body := `{"id":"s1","name":"Before","host":"localhost"}`
	req := httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Update
	body = `{"name":"After","host":"remotehost"}`
	req = httptest.NewRequest("PUT", "/api/v1/registry/servers/s1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestHandlerUpdateNotFound(t *testing.T) {
	_, mux := setupHandler("")

	body := `{"name":"X"}`
	req := httptest.NewRequest("PUT", "/api/v1/registry/servers/missing", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlerDeregister(t *testing.T) {
	_, mux := setupHandler("")

	// Register
	body := `{"id":"s1","name":"Test","host":"localhost"}`
	req := httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Deregister
	req = httptest.NewRequest("DELETE", "/api/v1/registry/servers/s1", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("deregister status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify gone
	req = httptest.NewRequest("GET", "/api/v1/registry/servers/s1", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("after deregister: status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlerDeregisterNotFound(t *testing.T) {
	_, mux := setupHandler("")

	req := httptest.NewRequest("DELETE", "/api/v1/registry/servers/missing", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlerHealth(t *testing.T) {
	_, mux := setupHandler("")

	req := httptest.NewRequest("GET", "/api/v1/registry/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if resp["status"] != "healthy" {
		t.Errorf("status = %v, want healthy", resp["status"])
	}
}

func TestHandlerRegisterMissingID(t *testing.T) {
	_, mux := setupHandler("")

	body := `{"name":"No ID","host":"localhost"}`
	req := httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing id: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlerRegisterWrongContentType(t *testing.T) {
	_, mux := setupHandler("")

	req := httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wrong content-type: status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestHandlerRegisterRejectsConcatenatedJSON(t *testing.T) {
	_, mux := setupHandler("")

	body := `{"id":"s1","name":"Test","host":"localhost"}{"id":"s2"}`
	req := httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("concatenated json: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlerListHealthyFilter(t *testing.T) {
	_, mux := setupHandler("")

	// Register healthy + unhealthy
	for _, body := range []string{
		`{"id":"h1","name":"Healthy","host":"a","health":"healthy"}`,
		`{"id":"h2","name":"Unhealthy","host":"b","health":"degraded"}`,
	} {
		req := httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
	}

	// List with healthy=true
	req := httptest.NewRequest("GET", "/api/v1/registry/servers?healthy=true", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode healthy filter response: %v", err)
	}
	if got := mustCountField(t, resp); got != 1 {
		t.Errorf("healthy filter count = %.0f, want 1", got)
	}
}

func TestHandlerUpdateRejectsConcatenatedJSON(t *testing.T) {
	_, mux := setupHandler("")

	// Register baseline server.
	body := `{"id":"s1","name":"Before","host":"localhost"}`
	req := httptest.NewRequest("POST", "/api/v1/registry/servers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	updateBody := `{"name":"After","host":"remotehost"}{"name":"Extra"}`
	req = httptest.NewRequest("PUT", "/api/v1/registry/servers/s1", strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("concatenated json update: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
