//go:build ignore

// h2_reverse_proxy runs a local self-signed HTTPS/HTTP2 reverse proxy for
// browser upload experiments.
//
// Usage:
//
//	go run scripts/perf/h2_reverse_proxy.go -target http://localhost:8080 -addr 127.0.0.1:8443
package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/saveenergy/openbyte/internal/tlsutil"
)

func selfSignedConfig() *tls.Config {
	cert, err := tlsutil.SelfSignedLocalhost()
	if err != nil {
		log.Fatalf("generate certificate: %v", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2", "http/1.1"},
		MinVersion:   tls.VersionTLS12,
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8443", "listen address")
	targetRaw := flag.String("target", "http://localhost:8080", "upstream target")
	flag.Parse()

	target, err := url.Parse(*targetRaw)
	if err != nil {
		log.Fatalf("parse target: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1
	server := &http.Server{
		Addr:              *addr,
		Handler:           proxy,
		TLSConfig:         selfSignedConfig(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("listening on https://%s -> %s", *addr, target)
	log.Fatal(server.ListenAndServeTLS("", ""))
}
