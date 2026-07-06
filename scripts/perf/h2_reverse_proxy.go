//go:build ignore

// h2_reverse_proxy runs a local self-signed HTTPS/HTTP2 reverse proxy for
// browser upload experiments.
//
// Usage:
//
//	go run scripts/perf/h2_reverse_proxy.go -target http://localhost:8080 -addr 127.0.0.1:8443
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func selfSignedConfig() *tls.Config {
	cert := selfSignedCertificate()
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2", "http/1.1"},
		MinVersion:   tls.VersionTLS12,
	}
}

func selfSignedCertificate() tls.Certificate {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("generate key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		log.Fatalf("generate serial: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		log.Fatalf("create certificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		log.Fatalf("load key pair: %v", err)
	}
	return cert
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
