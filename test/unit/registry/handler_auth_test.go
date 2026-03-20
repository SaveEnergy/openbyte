package registry_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerAuthRequired(t *testing.T) {
	_, mux := setupHandler("secret-key")

	// Register without auth → 401
	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(serverBodyS1))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no auth: status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Register with correct auth → 201
	req = httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(serverBodyS1))
	req.Header.Set(contentTypeHeader, applicationJSON)
	req.Header.Set("Authorization", "Bearer secret-key")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("with auth: status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestHandlerAuthWrongKey(t *testing.T) {
	_, mux := setupHandler("correct-key")

	req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(serverBodyS1))
	req.Header.Set(contentTypeHeader, applicationJSON)
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
		req := httptest.NewRequest(methodPost, registryServersPath, strings.NewReader(serverBodyS1))
		req.Header.Set(contentTypeHeader, applicationJSON)
		req.Header.Set("Authorization", auth)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("auth=%q: status = %d, want %d", auth, rec.Code, http.StatusUnauthorized)
		}
	}
}
