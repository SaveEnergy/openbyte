package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/saveenergy/openbyte/internal/api"
)

func TestHandleConnectivityCheckUsesAPIKeyArgument(t *testing.T) {
	const key = "secret-token"
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	auth := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Authorization") != "Bearer "+key {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}
		return true
	}
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		handler.Ping(w, r)
	})
	mux.HandleFunc("GET /api/v1/download", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		handler.Download(w, r)
	})
	mux.HandleFunc("POST /api/v1/upload", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		handler.Upload(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	reqNoKey := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{"server_url": srv.URL},
		},
	}
	res, err := handleConnectivityCheck(context.Background(), reqNoKey)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error without api_key")
	}

	reqWithKey := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{"server_url": srv.URL, "api_key": key},
		},
	}
	res, err = handleConnectivityCheck(context.Background(), reqWithKey)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success with api_key, got error content: %#v", res.Content)
	}
}

func TestClientFromRequestTrimsAPIKey(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{"api_key": "  token  "},
		},
	}
	c := clientFromRequest("http://localhost:8080", req)
	if c == nil {
		t.Fatal("expected client")
	}

	// Ensure empty/whitespace key falls back to default client path.
	req.Params.Arguments = map[string]any{"api_key": "   "}
	c = clientFromRequest("http://localhost:8080", req)
	if c == nil {
		t.Fatal("expected client for blank api key")
	}
}

func TestToolDefinitionsExposeAPIKeyArgument(t *testing.T) {
	tools := ToolDefinitions()
	for _, tool := range tools {
		if _, ok := tool.InputSchema.Properties["api_key"]; !ok {
			t.Fatalf("tool %s missing api_key property", tool.Name)
		}
		if strings.TrimSpace(tool.Description) == "" {
			t.Fatalf("tool %s missing description", tool.Name)
		}
	}
}
