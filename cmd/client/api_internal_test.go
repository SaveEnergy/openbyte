package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/saveenergy/openbyte/pkg/types"
)

type noopFormatter struct{}

func (noopFormatter) FormatProgress(float64, float64, float64) {}
func (noopFormatter) FormatMetrics(*types.Metrics)             {}
func (noopFormatter) FormatComplete(*StreamResults)            {}
func (noopFormatter) FormatError(error)                        {}

func TestCompleteStreamHonorsCanceledContext(t *testing.T) {
	called := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case called <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		ServerURL: server.URL,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := completeStream(ctx, cfg, "stream-id", EngineMetrics{})
	if err == nil {
		t.Fatal("expected canceled-context error")
	}

	select {
	case <-called:
		t.Fatal("completion request should not reach server when context is canceled")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestCompleteStreamReturnsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &Config{ServerURL: server.URL}
	err := completeStream(context.Background(), cfg, "stream-id", EngineMetrics{})
	if err == nil {
		t.Fatal("expected error for non-2xx completion status")
	}
}

func TestStreamMetricsNormalCloseWithoutComplete(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade failed: %v", err)
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"), time.Now().Add(time.Second))
		_ = conn.Close()
	}))
	defer server.Close()

	cfg := &Config{ServerURL: server.URL, Timeout: 2}
	err := streamMetrics(context.Background(), server.URL, noopFormatter{}, cfg)
	if err == nil {
		t.Fatal("expected error when websocket closes before complete message")
	}
	if !errors.Is(err, context.Canceled) && err.Error() == "" {
		t.Fatal("expected non-empty error")
	}
}
