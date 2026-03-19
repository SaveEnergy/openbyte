package jsonbody

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeSingleObjectOK(t *testing.T) {
	var dst struct {
		A int `json:"a"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1}`))
	w := httptest.NewRecorder()
	if err := DecodeSingleObject(w, req, &dst, 1024); err != nil {
		t.Fatal(err)
	}
	if dst.A != 1 {
		t.Fatalf("A = %d", dst.A)
	}
}

func TestDecodeSingleObjectUnknownField(t *testing.T) {
	var dst struct {
		A int `json:"a"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1,"b":2}`))
	w := httptest.NewRecorder()
	err := DecodeSingleObject(w, req, &dst, 1024)
	if err == nil {
		t.Fatal("want error")
	}
}

func TestDecodeSingleObjectTrailingJSON(t *testing.T) {
	var dst struct {
		A int `json:"a"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1}{"b":2}`))
	w := httptest.NewRecorder()
	err := DecodeSingleObject(w, req, &dst, 1024)
	if !errors.Is(err, ErrTrailingJSON) {
		t.Fatalf("err = %v, want ErrTrailingJSON", err)
	}
}

func TestDecodeSingleObjectMaxBytes(t *testing.T) {
	var dst struct {
		A int `json:"a"`
	}
	// 11 bytes of JSON; limit 10 triggers MaxBytesError while decoding.
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":12345}`))
	w := httptest.NewRecorder()
	err := DecodeSingleObject(w, req, &dst, 10)
	if err == nil {
		t.Fatal("want error")
	}
	var maxErr *http.MaxBytesError
	if !errors.As(err, &maxErr) {
		t.Fatalf("want MaxBytesError, got %v", err)
	}
}
