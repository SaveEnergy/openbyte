package server

import (
	"errors"
	"flag"
	"net/http"
	"testing"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestParseServerArgs(t *testing.T) {
	version, err := parseServerArgs([]string{"--version"})
	if err != nil || !version {
		t.Fatalf("version = %t, err = %v; want true, nil", version, err)
	}

	if _, err := parseServerArgs([]string{"--help"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help error = %v, want flag.ErrHelp", err)
	}

	if _, err := parseServerArgs([]string{"--port=9090"}); err == nil {
		t.Fatal("server configuration flags should be rejected")
	}
	if _, err := parseServerArgs([]string{"unexpected"}); err == nil {
		t.Fatal("positional server arguments should be rejected")
	}
}

func TestSpeedtestHTTP2ConfigUsesThroughputWindows(t *testing.T) {
	cfg := config.DefaultConfig()

	h2 := speedtestHTTP2Config(cfg)
	if h2.MaxReadFrameSize != 1024*1024 {
		t.Fatalf("MaxReadFrameSize = %d, want 1048576", h2.MaxReadFrameSize)
	}
	const receiveWindow = 4*1024*1024 - 1
	if h2.MaxReceiveBufferPerConnection != receiveWindow {
		t.Fatalf("MaxReceiveBufferPerConnection = %d, want %d", h2.MaxReceiveBufferPerConnection, receiveWindow)
	}
	if h2.MaxReceiveBufferPerStream != receiveWindow {
		t.Fatalf("MaxReceiveBufferPerStream = %d, want %d", h2.MaxReceiveBufferPerStream, receiveWindow)
	}
}

func TestSpeedtestHTTP2ConfigUsesTransferLimitWithProtocolFloor(t *testing.T) {
	tests := []struct {
		name  string
		cfg   *config.Config
		limit int
		want  int
	}{
		{name: "nil config", want: 100},
		{name: "below protocol floor", cfg: config.DefaultConfig(), limit: 50, want: 100},
		{name: "default", cfg: config.DefaultConfig(), limit: 200, want: 200},
		{name: "above default", cfg: config.DefaultConfig(), limit: 320, want: 320},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.cfg != nil {
				test.cfg.MaxConcurrentTransfers = test.limit
			}
			if got := speedtestHTTP2Config(test.cfg).MaxConcurrentStreams; got != test.want {
				t.Fatalf("MaxConcurrentStreams = %d, want %d", got, test.want)
			}
		})
	}
}

func TestDefaultMaxConcurrentPerIPMatchesBrowserRamp(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.MaxConcurrentPerIP != 64 {
		t.Fatalf("MaxConcurrentPerIP = %d, want 64", cfg.MaxConcurrentPerIP)
	}
}

func TestConfigureHTTPProtocolsDefaultsToHTTP2(t *testing.T) {
	cfg := config.DefaultConfig()
	srv := &http.Server{}
	configureHTTPProtocols(cfg, srv)
	if srv.Protocols != nil {
		t.Fatal("default protocols should stay nil so Go enables HTTP/1 and HTTP/2 defaults")
	}
}

func TestConfigureHTTPProtocolsCanDisableHTTP2(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.HTTP2Enabled = false
	srv := &http.Server{}
	configureHTTPProtocols(cfg, srv)
	if srv.Protocols == nil {
		t.Fatal("expected explicit protocols when HTTP/2 is disabled")
	}
	if !srv.Protocols.HTTP1() {
		t.Fatal("HTTP/1 should remain enabled")
	}
	if srv.Protocols.HTTP2() {
		t.Fatal("HTTP/2 should be disabled")
	}
}
