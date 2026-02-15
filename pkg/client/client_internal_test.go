package client

import "testing"

func TestNewClientHasNoFixedHTTPTimeout(t *testing.T) {
	c := New("http://localhost:8080")
	if c.httpClient == nil {
		t.Fatal("http client should be initialized")
	}
	if c.httpClient.Timeout != 0 {
		t.Fatalf("default http client timeout = %v, want 0", c.httpClient.Timeout)
	}
}
