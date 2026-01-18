package quic

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
)

// GetTLSConfig returns a TLS config for QUIC/HTTP3
func GetTLSConfig(cfg *config.Config) (*tls.Config, error) {
	// If cert files are specified, use them
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS cert/key: %w", err)
		}
		logging.Info("Loaded TLS certificate",
			logging.Field{Key: "cert", Value: cfg.TLSCertFile},
			logging.Field{Key: "key", Value: cfg.TLSKeyFile})
		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"speedtest", "h3", "h3-29"}, // QUIC speedtest + HTTP/3 ALPN
			MinVersion:   tls.VersionTLS13,
		}, nil
	}

	// Auto-generate self-signed cert
	if cfg.TLSAutoGen {
		return generateSelfSignedCert(cfg)
	}

	return nil, fmt.Errorf("no TLS certificate configured and auto-generation disabled")
}

func generateSelfSignedCert(cfg *config.Config) (*tls.Config, error) {
	// Determine cert directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/tmp"
	}
	certDir := filepath.Join(homeDir, ".openbyte", "certs")
	certFile := filepath.Join(certDir, "server.crt")
	keyFile := filepath.Join(certDir, "server.key")

	// Check if certs already exist
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err == nil {
				logging.Info("Using existing self-signed certificate",
					logging.Field{Key: "path", Value: certDir})
				return &tls.Config{
					Certificates: []tls.Certificate{cert},
					NextProtos:   []string{"speedtest", "h3", "h3-29"},
					MinVersion:   tls.VersionTLS13,
				}, nil
			}
		}
	}

	// Create cert directory
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create cert directory: %w", err)
	}

	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Generate serial number
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"OpenByte Speed Test"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0")},
		DNSNames:              []string{"localhost", cfg.PublicHost},
	}

	// Add public host IP if it's an IP address
	if ip := net.ParseIP(cfg.PublicHost); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	}

	// Create certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Save certificate
	certOut, err := os.Create(certFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create cert file: %w", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	// Save private key
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create key file: %w", err)
	}
	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	keyOut.Close()

	logging.Info("Generated self-signed certificate",
		logging.Field{Key: "path", Value: certDir},
		logging.Field{Key: "valid_for", Value: "1 year"})

	// Load and return
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load generated cert: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"speedtest", "h3", "h3-29"},
		MinVersion:   tls.VersionTLS13,
	}, nil
}
