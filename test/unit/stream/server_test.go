package stream_test

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
)

func TestServerCloseAfterStartReturnsPromptly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.BindAddress = "127.0.0.1"
	cfg.TCPTestPort = 0
	cfg.UDPTestPort = 0

	srv, err := stream.NewServer(cfg)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	done := make(chan struct{})
	go func() {
		_ = srv.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("server close timed out")
	}
}
