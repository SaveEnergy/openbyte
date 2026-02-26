package client

import "testing"

func TestNewClientHasDefaultHTTPTimeout(t *testing.T) {
	c := New("http://localhost:8080")
	if c.httpClient == nil {
		t.Fatal("http client should be initialized")
	}
	if c.httpClient.Timeout != defaultHTTPTimeout {
		t.Fatalf("default http client timeout = %v, want %v", c.httpClient.Timeout, defaultHTTPTimeout)
	}
}
