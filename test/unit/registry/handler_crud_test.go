package registry_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerRegisterAndList(t *testing.T) {
	_, mux := setupHandler("")

	// Register
	body := `{"id":"s1","name":"Test Server","host":"localhost","tcp_port":8081,"udp_port":8082,"health":"healthy"}`
	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// List
	req = httptest.NewRequest(methodGet, registryServersPath, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
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
	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(serverBodyS1))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Get
	req = httptest.NewRequest(methodGet, registryServerS1Path, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandlerGetServerNotFound(t *testing.T) {
	_, mux := setupHandler("")

	req := httptest.NewRequest(methodGet, registryMissingPath, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusNotFound)
	}
}

func TestHandlerUpdateServer(t *testing.T) {
	_, mux := setupHandler("")

	// Register
	body := `{"id":"s1","name":"Before","host":"localhost"}`
	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Update
	body = `{"name":"After","host":"remotehost"}`
	req = httptest.NewRequest(methodPut, registryServerS1Path, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestHandlerUpdateNotFound(t *testing.T) {
	_, mux := setupHandler("")

	body := `{"name":"X"}`
	req := httptest.NewRequest(methodPut, registryMissingPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusNotFound)
	}
}

func TestHandlerDeregister(t *testing.T) {
	_, mux := setupHandler("")

	// Register
	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(serverBodyS1))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Deregister
	req = httptest.NewRequest(methodDelete, registryServerS1Path, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("deregister status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify gone
	req = httptest.NewRequest(methodGet, registryServerS1Path, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("after deregister: status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlerDeregisterNotFound(t *testing.T) {
	_, mux := setupHandler("")

	req := httptest.NewRequest(methodDelete, registryMissingPath, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusNotFound)
	}
}

func TestHandlerHealth(t *testing.T) {
	_, mux := setupHandler("")

	req := httptest.NewRequest(methodGet, registryHealthPath, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if resp["status"] != "healthy" {
		t.Errorf("status = %v, want healthy", resp["status"])
	}
}

func TestHandlerListHealthyFilter(t *testing.T) {
	_, mux := setupHandler("")

	// Register healthy + unhealthy
	for _, body := range []string{
		`{"id":"h1","name":"Healthy","host":"a","health":"healthy"}`,
		`{"id":"h2","name":"Unhealthy","host":"b","health":"degraded"}`,
	} {
		req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(body))
		req.Header.Set(contentTypeHeader, applicationJSON)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
	}

	// List with healthy=true
	req := httptest.NewRequest(methodGet, registryHealthyQuery, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode healthy filter response: %v", err)
	}
	if got := mustCountField(t, resp); got != 1 {
		t.Errorf("healthy filter count = %.0f, want 1", got)
	}
}
func TestHandlerUpdateServerPreservesRequiredFields(t *testing.T) {
	_, mux := setupHandler("")

	// Register with required baseline fields.
	registerBody := `{"id":"s1","name":"Before","host":"localhost","tcp_port":8081,"udp_port":8082,"api_endpoint":"http://localhost:8080","health":"healthy"}`
	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(registerBody))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d", rec.Code, http.StatusCreated)
	}

	// Partial update should not zero omitted fields.
	updateBody := `{"name":"After"}`
	req = httptest.NewRequest(methodPut, registryServerS1Path, strings.NewReader(updateBody))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d", rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(methodGet, registryServerS1Path, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode server: %v", err)
	}
	if got["name"] != "After" {
		t.Fatalf("name = %v, want After", got["name"])
	}
	if got["host"] != "localhost" {
		t.Fatalf("host = %v, want localhost", got["host"])
	}
	if got["tcp_port"] != float64(8081) {
		t.Fatalf("tcp_port = %v, want 8081", got["tcp_port"])
	}
	if got["udp_port"] != float64(8082) {
		t.Fatalf("udp_port = %v, want 8082", got["udp_port"])
	}
}
