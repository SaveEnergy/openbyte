package registry_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerRegisterMissingID(t *testing.T) {
	_, mux := setupHandler("")

	body := `{"name":"No ID","host":"localhost"}`
	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing id: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlerRegisterWrongContentType(t *testing.T) {
	_, mux := setupHandler("")

	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader("not json"))
	req.Header.Set(contentTypeHeader, textPlainType)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wrong content-type: status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestHandlerRegisterRejectsConcatenatedJSON(t *testing.T) {
	_, mux := setupHandler("")

	body := `{"id":"s1","name":"Test","host":"localhost"}{"id":"s2"}`
	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("concatenated json: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlerRegisterRejectsUnknownFields(t *testing.T) {
	_, mux := setupHandler("")
	body := `{"id":"s1","name":"Test","host":"localhost","unknown":1}`
	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field register: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlerUpdateRejectsConcatenatedJSON(t *testing.T) {
	_, mux := setupHandler("")

	// Register baseline server.
	body := `{"id":"s1","name":"Before","host":"localhost"}`
	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	updateBody := `{"name":"After","host":"remotehost"}{"name":"Extra"}`
	req = httptest.NewRequest(methodPut, registryServerS1Path, strings.NewReader(updateBody))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("concatenated json update: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlerUpdateRejectsUnknownFields(t *testing.T) {
	_, mux := setupHandler("")
	body := `{"id":"s1","name":"Before","host":"localhost"}`
	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	updateBody := `{"name":"After","unknown":true}`
	req = httptest.NewRequest(methodPut, registryServerS1Path, strings.NewReader(updateBody))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field update: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlerRegisterBodyTooLarge(t *testing.T) {
	_, mux := setupHandler("")
	oversized := strings.Repeat("x", 70*1024)
	body := `{"id":"s1","name":"` + oversized + `","host":"localhost"}`

	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("register body too large: status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestHandlerUpdateBodyTooLarge(t *testing.T) {
	_, mux := setupHandler("")
	// Register first
	body := `{"id":"s1","name":"Before","host":"localhost"}`
	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	oversized := strings.Repeat("x", 70*1024)
	update := `{"name":"` + oversized + `","host":"localhost"}`
	req = httptest.NewRequest(methodPut, registryServerS1Path, strings.NewReader(update))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("update body too large: status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}
