// Package certs provides TLS certificate generation for PostgreSQL mTLS.
package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// CertBundle holds the generated certificates.
type CertBundle struct {
	CACert     []byte
	CAKey      []byte
	ServerCert []byte
	ServerKey  []byte
	ClientCert []byte
	ClientKey  []byte
}

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

// DefaultGeneratorConfig returns sensible defaults.
func DefaultGeneratorConfig(nodeID string) GeneratorConfig {
	return GeneratorConfig{
		CACommonName:     fmt.Sprintf("bibd-ca-%s", nodeID[:8]),
		ServerCommonName: "bibd-postgres",
		ClientCommonName: fmt.Sprintf("bibd-client-%s", nodeID[:8]),
		ValidDuration:    365 * 24 * time.Hour, // 1 year
		DNSNames:         []string{"localhost", "bibd-postgres"},
		IPAddresses:      []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		Organization:     "bibd",
	}
}

// Generate generates a complete certificate bundle.
func Generate(cfg GeneratorConfig) (*CertBundle, error) {
	bundle := &CertBundle{}

	// Generate CA
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: newSerialNumber(),
		Subject: pkix.Name{
			CommonName:   cfg.CACommonName,
			Organization: []string{cfg.Organization},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(cfg.ValidDuration),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
		MaxPathLen:            1,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	bundle.CACert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})

	caKeyDER, err := x509.MarshalECPrivateKey(caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal CA key: %w", err)
	}
	bundle.CAKey = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: caKeyDER})

	// Parse CA cert for signing
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Generate server certificate
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate server key: %w", err)
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: newSerialNumber(),
		Subject: pkix.Name{
			CommonName:   cfg.ServerCommonName,
			Organization: []string{cfg.Organization},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(cfg.ValidDuration),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    cfg.DNSNames,
		IPAddresses: cfg.IPAddresses,
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create server certificate: %w", err)
	}

	bundle.ServerCert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER})

	serverKeyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal server key: %w", err)
	}
	bundle.ServerKey = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: serverKeyDER})

	// Generate client certificate
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client key: %w", err)
	}

	clientTemplate := &x509.Certificate{
		SerialNumber: newSerialNumber(),
		Subject: pkix.Name{
			CommonName:   cfg.ClientCommonName,
			Organization: []string{cfg.Organization},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(cfg.ValidDuration),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create client certificate: %w", err)
	}

	bundle.ClientCert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})

	clientKeyDER, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal client key: %w", err)
	}
	bundle.ClientKey = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: clientKeyDER})

	return bundle, nil
}

// newSerialNumber generates a random serial number.
func newSerialNumber() *big.Int {
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, _ := rand.Int(rand.Reader, max)
	return serial
}

// SaveToDir saves the certificate bundle to a directory.
func (b *CertBundle) SaveToDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	files := map[string][]byte{
		"ca.crt":     b.CACert,
		"ca.key":     b.CAKey,
		"server.crt": b.ServerCert,
		"server.key": b.ServerKey,
		"client.crt": b.ClientCert,
		"client.key": b.ClientKey,
	}

	for name, data := range files {
		path := filepath.Join(dir, name)
		perm := os.FileMode(0644)
		if filepath.Ext(name) == ".key" {
			perm = 0600 // Restrict key files
		}
		if err := os.WriteFile(path, data, perm); err != nil {
			return fmt.Errorf("failed to write %s: %w", name, err)
		}
	}

	return nil
}

// LoadFromDir loads a certificate bundle from a directory.
func LoadFromDir(dir string) (*CertBundle, error) {
	bundle := &CertBundle{}

	files := map[string]*[]byte{
		"ca.crt":     &bundle.CACert,
		"ca.key":     &bundle.CAKey,
		"server.crt": &bundle.ServerCert,
		"server.key": &bundle.ServerKey,
		"client.crt": &bundle.ClientCert,
		"client.key": &bundle.ClientKey,
	}

	for name, dest := range files {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", name, err)
		}
		*dest = data
	}

	return bundle, nil
}

// Exists checks if certificates already exist in the directory.
func Exists(dir string) bool {
	required := []string{"ca.crt", "server.crt", "server.key"}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// NeedsRotation checks if certificates need rotation.
func NeedsRotation(dir string, threshold time.Duration) (bool, error) {
	certPath := filepath.Join(dir, "server.crt")
	data, err := os.ReadFile(certPath)
	if err != nil {
		return true, nil // If can't read, assume needs rotation
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return true, nil
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true, nil
	}

	// Check if certificate expires within threshold
	if time.Until(cert.NotAfter) < threshold {
		return true, nil
	}

	return false, nil
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
