// Package certs provides TLS certificate generation for PostgreSQL mTLS.
// This package wraps the common certs package with PostgreSQL-specific defaults.
package certs

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	commoncerts "bib/internal/certs"
)

// CertBundle holds the generated certificates.
// Re-exported from common certs package for backwards compatibility.
type CertBundle = commoncerts.CertBundle

// GeneratorConfig holds configuration for certificate generation.
type GeneratorConfig struct {
	// CommonName for the CA
	CACommonName string

	// ServerCommonName for the server certificate
	ServerCommonName string

	// ClientCommonName for the client certificate
	ClientCommonName string

	// ValidDuration is how long the certificates are valid
	ValidDuration time.Duration

	// DNSNames for the server certificate
	DNSNames []string

	// IPAddresses for the server certificate
	IPAddresses []net.IP

	// Organization for all certificates
	Organization string
}

// DefaultGeneratorConfig returns sensible defaults for PostgreSQL.
func DefaultGeneratorConfig(nodeID string) GeneratorConfig {
	shortID := nodeID
	if len(nodeID) > 8 {
		shortID = nodeID[:8]
	}
	return GeneratorConfig{
		CACommonName:     fmt.Sprintf("bibd-postgres-ca-%s", shortID),
		ServerCommonName: "bibd-postgres",
		ClientCommonName: fmt.Sprintf("bibd-postgres-client-%s", shortID),
		ValidDuration:    365 * 24 * time.Hour, // 1 year
		DNSNames:         []string{"localhost", "bibd-postgres"},
		IPAddresses:      []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		Organization:     "bibd-postgres",
	}
}

// toCommonConfig converts PostgreSQL config to common certs config.
func (cfg GeneratorConfig) toCommonConfig() commoncerts.GeneratorConfig {
	return commoncerts.GeneratorConfig{
		CACommonName:        cfg.CACommonName,
		ServerCommonName:    cfg.ServerCommonName,
		ClientCommonName:    cfg.ClientCommonName,
		CAValidDuration:     cfg.ValidDuration,
		ServerValidDuration: cfg.ValidDuration,
		ClientValidDuration: cfg.ValidDuration,
		DNSNames:            cfg.DNSNames,
		IPAddresses:         cfg.IPAddresses,
		Organization:        cfg.Organization,
	}
}

// Generate generates a complete certificate bundle.
func Generate(cfg GeneratorConfig) (*CertBundle, error) {
	return commoncerts.GenerateBundle(cfg.toCommonConfig())
}

// SaveToDir saves the certificate bundle to a directory.
// This is now handled by the CertBundle type alias.

// LoadFromDir loads a certificate bundle from a directory.
func LoadFromDir(dir string) (*CertBundle, error) {
	return commoncerts.LoadFromDir(dir)
}

// Exists checks if certificates already exist in the directory.
func Exists(dir string) bool {
	return commoncerts.Exists(dir)
}

// NeedsRotation checks if certificates need rotation.
func NeedsRotation(dir string, threshold time.Duration) (bool, error) {
	certPath := filepath.Join(dir, "server.crt")
	data, err := os.ReadFile(certPath)
	if err != nil {
		return true, nil // If can't read, assume needs rotation
	}

	return commoncerts.NeedsRenewal(data, threshold)
}

// RotateCertificates generates new certificates and backs up old ones.
func RotateCertificates(dir string, cfg GeneratorConfig) (*CertBundle, error) {
	// Backup existing certificates
	backupDir := filepath.Join(dir, fmt.Sprintf("backup-%d", time.Now().Unix()))
	if Exists(dir) {
		if err := os.MkdirAll(backupDir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create backup directory: %w", err)
		}

		oldBundle, err := LoadFromDir(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to load existing certificates: %w", err)
		}

		if err := oldBundle.SaveToDir(backupDir); err != nil {
			return nil, fmt.Errorf("failed to backup certificates: %w", err)
		}
	}

	// Generate new certificates
	bundle, err := Generate(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new certificates: %w", err)
	}

	if err := bundle.SaveToDir(dir); err != nil {
		return nil, fmt.Errorf("failed to save new certificates: %w", err)
	}

	return bundle, nil
}
