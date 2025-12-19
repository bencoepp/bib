// Package certs provides TLS certificate generation and management for bibd.
// This package handles CA generation, server/client certificate issuance,
// certificate rotation, and Trust-On-First-Use (TOFU) for CLI connections.
package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// CertBundle holds a set of related certificates (CA, server, client).
type CertBundle struct {
	CACert     []byte
	CAKey      []byte
	ServerCert []byte
	ServerKey  []byte
	ClientCert []byte
	ClientKey  []byte
}

// Certificate represents a parsed certificate with metadata.
type Certificate struct {
	Raw         []byte
	Cert        *x509.Certificate
	Fingerprint string // SHA256 fingerprint in hex
	ExpiresAt   time.Time
	IsCA        bool
	Subject     string
	Issuer      string
	SerialHex   string
	DNSNames    []string
	IPAddresses []net.IP
}

// GeneratorConfig holds configuration for certificate generation.
type GeneratorConfig struct {
	// CACommonName for the CA certificate
	CACommonName string

	// ServerCommonName for the server certificate
	ServerCommonName string

	// ClientCommonName for the client certificate
	ClientCommonName string

	// CAValidDuration is how long the CA certificate is valid (default: 10 years)
	CAValidDuration time.Duration

	// ServerValidDuration is how long server certificates are valid (default: 1 year)
	ServerValidDuration time.Duration

	// ClientValidDuration is how long client certificates are valid (default: 90 days)
	ClientValidDuration time.Duration

	// DNSNames for the server certificate SANs
	DNSNames []string

	// IPAddresses for the server certificate SANs
	IPAddresses []net.IP

	// Organization for all certificates
	Organization string

	// PeerID is the node's P2P peer ID (included in server cert SAN)
	PeerID string

	// SSHKeyFingerprint is the user's SSH key fingerprint (for client certs)
	SSHKeyFingerprint string

	// UserID is the user ID for client certificates
	UserID string
}

// DefaultConfig returns sensible defaults for certificate generation.
func DefaultConfig(nodeID string) GeneratorConfig {
	shortID := nodeID
	if len(nodeID) > 8 {
		shortID = nodeID[:8]
	}

	return GeneratorConfig{
		CACommonName:        fmt.Sprintf("bibd-ca-%s", shortID),
		ServerCommonName:    fmt.Sprintf("bibd-server-%s", shortID),
		ClientCommonName:    fmt.Sprintf("bibd-client-%s", shortID),
		CAValidDuration:     10 * 365 * 24 * time.Hour, // 10 years
		ServerValidDuration: 365 * 24 * time.Hour,      // 1 year
		ClientValidDuration: 90 * 24 * time.Hour,       // 90 days
		DNSNames:            []string{"localhost"},
		IPAddresses:         []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		Organization:        "bibd",
	}
}

// GenerateCA generates a new Certificate Authority.
func GenerateCA(cfg GeneratorConfig) (cert, key []byte, err error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	validDuration := cfg.CAValidDuration
	if validDuration == 0 {
		validDuration = 10 * 365 * 24 * time.Hour // 10 years default
	}

	template := &x509.Certificate{
		SerialNumber: newSerialNumber(),
		Subject: pkix.Name{
			CommonName:   cfg.CACommonName,
			Organization: []string{cfg.Organization},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(validDuration),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
		MaxPathLen:            1,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal CA key: %w", err)
	}
	key = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return cert, key, nil
}

// GenerateServerCert generates a server certificate signed by the CA.
func GenerateServerCert(caCert, caKey []byte, cfg GeneratorConfig) (cert, key []byte, err error) {
	ca, caPriv, err := parseCABundle(caCert, caKey)
	if err != nil {
		return nil, nil, err
	}

	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate server key: %w", err)
	}

	validDuration := cfg.ServerValidDuration
	if validDuration == 0 {
		validDuration = 365 * 24 * time.Hour // 1 year default
	}

	dnsNames := cfg.DNSNames
	// Add peer ID as a SAN if provided
	if cfg.PeerID != "" {
		dnsNames = append(dnsNames, cfg.PeerID)
	}

	template := &x509.Certificate{
		SerialNumber: newSerialNumber(),
		Subject: pkix.Name{
			CommonName:   cfg.ServerCommonName,
			Organization: []string{cfg.Organization},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(validDuration),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    dnsNames,
		IPAddresses: cfg.IPAddresses,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca, &serverKey.PublicKey, caPriv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create server certificate: %w", err)
	}

	cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal server key: %w", err)
	}
	key = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return cert, key, nil
}

// GenerateClientCert generates a client certificate signed by the CA.
// The SSH key fingerprint is included in the certificate subject for binding.
func GenerateClientCert(caCert, caKey []byte, cfg GeneratorConfig) (cert, key []byte, err error) {
	ca, caPriv, err := parseCABundle(caCert, caKey)
	if err != nil {
		return nil, nil, err
	}

	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate client key: %w", err)
	}

	validDuration := cfg.ClientValidDuration
	if validDuration == 0 {
		validDuration = 90 * 24 * time.Hour // 90 days default
	}

	subject := pkix.Name{
		CommonName:   cfg.ClientCommonName,
		Organization: []string{cfg.Organization},
	}

	// Include SSH key fingerprint in the subject's SerialNumber field
	// This creates a binding between the TLS client cert and SSH identity
	if cfg.SSHKeyFingerprint != "" {
		subject.SerialNumber = fmt.Sprintf("ssh-fp:%s", cfg.SSHKeyFingerprint)
	}

	// Include user ID in organizational unit
	if cfg.UserID != "" {
		subject.OrganizationalUnit = []string{fmt.Sprintf("user:%s", cfg.UserID)}
	}

	template := &x509.Certificate{
		SerialNumber: newSerialNumber(),
		Subject:      subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(validDuration),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca, &clientKey.PublicKey, caPriv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create client certificate: %w", err)
	}

	cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal client key: %w", err)
	}
	key = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return cert, key, nil
}

// GenerateBundle generates a complete certificate bundle (CA + server + client).
func GenerateBundle(cfg GeneratorConfig) (*CertBundle, error) {
	bundle := &CertBundle{}

	// Generate CA
	var err error
	bundle.CACert, bundle.CAKey, err = GenerateCA(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA: %w", err)
	}

	// Generate server certificate
	bundle.ServerCert, bundle.ServerKey, err = GenerateServerCert(bundle.CACert, bundle.CAKey, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate server cert: %w", err)
	}

	// Generate client certificate
	bundle.ClientCert, bundle.ClientKey, err = GenerateClientCert(bundle.CACert, bundle.CAKey, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client cert: %w", err)
	}

	return bundle, nil
}

// ParseCertificate parses a PEM-encoded certificate and returns detailed info.
func ParseCertificate(certPEM []byte) (*Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	fingerprint := sha256.Sum256(cert.Raw)

	return &Certificate{
		Raw:         certPEM,
		Cert:        cert,
		Fingerprint: hex.EncodeToString(fingerprint[:]),
		ExpiresAt:   cert.NotAfter,
		IsCA:        cert.IsCA,
		Subject:     cert.Subject.String(),
		Issuer:      cert.Issuer.String(),
		SerialHex:   fmt.Sprintf("%x", cert.SerialNumber),
		DNSNames:    cert.DNSNames,
		IPAddresses: cert.IPAddresses,
	}, nil
}

// Fingerprint calculates the SHA256 fingerprint of a PEM-encoded certificate.
func Fingerprint(certPEM []byte) (string, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	hash := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(hash[:]), nil
}

// NeedsRenewal checks if a certificate needs renewal based on the threshold.
func NeedsRenewal(certPEM []byte, threshold time.Duration) (bool, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return true, nil // Can't parse, assume needs renewal
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true, nil // Can't parse, assume needs renewal
	}

	// Check if certificate expires within threshold
	return time.Until(cert.NotAfter) < threshold, nil
}

// VerifyChain verifies that a certificate is signed by the given CA.
func VerifyChain(certPEM, caCertPEM []byte) error {
	return VerifyChainWithUsage(certPEM, caCertPEM, nil)
}

// VerifyChainWithUsage verifies that a certificate is signed by the given CA
// and optionally checks for specific extended key usages.
func VerifyChainWithUsage(certPEM, caCertPEM []byte, keyUsages []x509.ExtKeyUsage) error {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil {
		return fmt.Errorf("failed to decode CA PEM")
	}

	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	opts := x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: time.Now(),
	}
	if len(keyUsages) > 0 {
		opts.KeyUsages = keyUsages
	}

	_, err = cert.Verify(opts)
	return err
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
		if data == nil {
			continue // Skip empty files
		}
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
			if os.IsNotExist(err) {
				continue // Optional files
			}
			return nil, fmt.Errorf("failed to read %s: %w", name, err)
		}
		*dest = data
	}

	return bundle, nil
}

// Exists checks if essential certificates exist in the directory.
func Exists(dir string) bool {
	required := []string{"ca.crt", "server.crt", "server.key"}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// CAExists checks if CA certificate exists in the directory.
func CAExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "ca.crt"))
	return err == nil
}

// newSerialNumber generates a random serial number.
func newSerialNumber() *big.Int {
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, _ := rand.Int(rand.Reader, max)
	return serial
}

// parseCABundle parses CA certificate and key from PEM.
func parseCABundle(caCertPEM, caKeyPEM []byte) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		return nil, nil, fmt.Errorf("failed to decode CA certificate PEM")
	}

	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	keyBlock, _ := pem.Decode(caKeyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA key PEM")
	}

	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA key: %w", err)
	}

	return caCert, caKey, nil
}
