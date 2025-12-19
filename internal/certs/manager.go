package certs

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ManagerConfig holds configuration for the certificate manager.
type ManagerConfig struct {
	// ConfigDir is the base configuration directory
	ConfigDir string

	// NodeID is this node's unique identifier
	NodeID string

	// PeerID is this node's P2P peer ID (optional)
	PeerID string

	// P2PIdentityKey is the node's P2P identity private key (for CA key encryption)
	P2PIdentityKey []byte

	// ListenAddresses are the addresses the server listens on (for SANs)
	ListenAddresses []string

	// CAValidityYears is how many years the CA is valid (default: 10)
	CAValidityYears int

	// ServerCertValidityDays is how many days server certs are valid (default: 365)
	ServerCertValidityDays int

	// ClientCertValidityDays is how many days client certs are valid (default: 90)
	ClientCertValidityDays int

	// RenewalThresholdDays is how many days before expiry to renew (default: 30)
	RenewalThresholdDays int
}

// Manager handles certificate lifecycle for bibd.
type Manager struct {
	cfg            ManagerConfig
	certsDir       string
	secretsDir     string
	trustedDir     string
	revocationPath string

	caCert     []byte
	caKey      []byte // Decrypted CA key (only kept in memory)
	serverCert []byte
	serverKey  []byte

	tlsConfig *tls.Config
	caPool    *x509.CertPool

	revocationList *RevocationList
	trustStore     *TrustStore

	mu sync.RWMutex
}

// NewManager creates a new certificate manager.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.ConfigDir == "" {
		return nil, fmt.Errorf("config directory is required")
	}
	if cfg.NodeID == "" {
		return nil, fmt.Errorf("node ID is required")
	}
	if len(cfg.P2PIdentityKey) == 0 {
		return nil, fmt.Errorf("P2P identity key is required for CA key encryption")
	}

	// Set defaults
	if cfg.CAValidityYears == 0 {
		cfg.CAValidityYears = 10
	}
	if cfg.ServerCertValidityDays == 0 {
		cfg.ServerCertValidityDays = 365
	}
	if cfg.ClientCertValidityDays == 0 {
		cfg.ClientCertValidityDays = 90
	}
	if cfg.RenewalThresholdDays == 0 {
		cfg.RenewalThresholdDays = 30
	}

	m := &Manager{
		cfg:            cfg,
		certsDir:       filepath.Join(cfg.ConfigDir, "certs"),
		secretsDir:     filepath.Join(cfg.ConfigDir, "secrets"),
		trustedDir:     filepath.Join(cfg.ConfigDir, "trusted_nodes"),
		revocationPath: filepath.Join(cfg.ConfigDir, "certs", "revocation.json"),
	}

	return m, nil
}

// Initialize initializes the certificate infrastructure.
// This should be called during daemon startup and will block until certs are ready.
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure directories exist
	for _, dir := range []string{m.certsDir, m.secretsDir, m.trustedDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Load or create CA
	if err := m.initializeCA(); err != nil {
		return fmt.Errorf("failed to initialize CA: %w", err)
	}

	// Load or create server certificate
	if err := m.initializeServerCert(); err != nil {
		return fmt.Errorf("failed to initialize server certificate: %w", err)
	}

	// Initialize TLS config
	if err := m.initializeTLSConfig(); err != nil {
		return fmt.Errorf("failed to initialize TLS config: %w", err)
	}

	// Initialize revocation list
	rl, err := NewRevocationList(m.revocationPath)
	if err != nil {
		return fmt.Errorf("failed to initialize revocation list: %w", err)
	}
	m.revocationList = rl

	// Initialize trust store
	ts, err := NewTrustStore(m.trustedDir)
	if err != nil {
		return fmt.Errorf("failed to initialize trust store: %w", err)
	}
	m.trustStore = ts

	return nil
}

// initializeCA loads or creates the CA certificate.
func (m *Manager) initializeCA() error {
	caCertPath := filepath.Join(m.certsDir, "ca.crt")
	caKeyPath := filepath.Join(m.secretsDir, "ca.key.enc")

	// Check if CA exists
	if _, err := os.Stat(caCertPath); err == nil {
		// Load existing CA
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return fmt.Errorf("failed to read CA cert: %w", err)
		}
		m.caCert = caCert

		// Load and decrypt CA key
		caKey, err := LoadEncryptedKey(caKeyPath, m.cfg.P2PIdentityKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt CA key: %w", err)
		}
		m.caKey = caKey

		// Log CA fingerprint
		fp, _ := Fingerprint(m.caCert)
		fmt.Printf("Loaded CA certificate (fingerprint: %s)\n", fp[:16]+"...")

		return nil
	}

	// Generate new CA
	cfg := m.generatorConfig()
	caCert, caKey, err := GenerateCA(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate CA: %w", err)
	}

	// Save CA cert (unencrypted)
	if err := os.WriteFile(caCertPath, caCert, 0644); err != nil {
		return fmt.Errorf("failed to save CA cert: %w", err)
	}

	// Save CA key encrypted
	if err := SaveEncryptedKey(caKeyPath, caKey, m.cfg.P2PIdentityKey); err != nil {
		return fmt.Errorf("failed to save encrypted CA key: %w", err)
	}

	m.caCert = caCert
	m.caKey = caKey

	fp, _ := Fingerprint(m.caCert)
	fmt.Printf("Generated new CA certificate (fingerprint: %s)\n", fp[:16]+"...")

	return nil
}

// initializeServerCert loads or creates the server certificate.
func (m *Manager) initializeServerCert() error {
	serverCertPath := filepath.Join(m.certsDir, "server.crt")
	serverKeyPath := filepath.Join(m.certsDir, "server.key")

	renewalThreshold := time.Duration(m.cfg.RenewalThresholdDays) * 24 * time.Hour

	// Check if server cert exists and is valid
	if _, err := os.Stat(serverCertPath); err == nil {
		serverCert, err := os.ReadFile(serverCertPath)
		if err != nil {
			return fmt.Errorf("failed to read server cert: %w", err)
		}

		needsRenewal, err := NeedsRenewal(serverCert, renewalThreshold)
		if err != nil || needsRenewal {
			// Regenerate
			if err := m.generateServerCert(); err != nil {
				return err
			}
		} else {
			m.serverCert = serverCert
			serverKey, err := os.ReadFile(serverKeyPath)
			if err != nil {
				return fmt.Errorf("failed to read server key: %w", err)
			}
			m.serverKey = serverKey
		}
	} else {
		// Generate new server cert
		if err := m.generateServerCert(); err != nil {
			return err
		}
	}

	return nil
}

// generateServerCert generates a new server certificate.
func (m *Manager) generateServerCert() error {
	cfg := m.generatorConfig()

	serverCert, serverKey, err := GenerateServerCert(m.caCert, m.caKey, cfg)
	if err != nil {
		return fmt.Errorf("failed to generate server cert: %w", err)
	}

	serverCertPath := filepath.Join(m.certsDir, "server.crt")
	serverKeyPath := filepath.Join(m.certsDir, "server.key")

	if err := os.WriteFile(serverCertPath, serverCert, 0644); err != nil {
		return fmt.Errorf("failed to save server cert: %w", err)
	}
	if err := os.WriteFile(serverKeyPath, serverKey, 0600); err != nil {
		return fmt.Errorf("failed to save server key: %w", err)
	}

	m.serverCert = serverCert
	m.serverKey = serverKey

	return nil
}

// initializeTLSConfig creates the TLS configuration for the gRPC server.
func (m *Manager) initializeTLSConfig() error {
	// Load server certificate
	cert, err := tls.X509KeyPair(m.serverCert, m.serverKey)
	if err != nil {
		return fmt.Errorf("failed to load server keypair: %w", err)
	}

	// Create CA pool for client verification
	m.caPool = x509.NewCertPool()
	if !m.caPool.AppendCertsFromPEM(m.caCert) {
		return fmt.Errorf("failed to add CA cert to pool")
	}

	m.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    m.caPool,
		ClientAuth:   tls.VerifyClientCertIfGiven, // Allow both mTLS and server-only TLS
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	return nil
}

// generatorConfig returns the generator config for certificates.
func (m *Manager) generatorConfig() GeneratorConfig {
	cfg := DefaultConfig(m.cfg.NodeID)
	cfg.PeerID = m.cfg.PeerID
	cfg.CAValidDuration = time.Duration(m.cfg.CAValidityYears) * 365 * 24 * time.Hour
	cfg.ServerValidDuration = time.Duration(m.cfg.ServerCertValidityDays) * 24 * time.Hour
	cfg.ClientValidDuration = time.Duration(m.cfg.ClientCertValidityDays) * 24 * time.Hour

	// Parse listen addresses for SANs
	for _, addr := range m.cfg.ListenAddresses {
		// Try to parse as IP
		if ip := parseIP(addr); ip != nil {
			cfg.IPAddresses = append(cfg.IPAddresses, ip)
		}
	}

	return cfg
}

// TLSConfig returns the server TLS configuration.
func (m *Manager) TLSConfig() *tls.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tlsConfig
}

// CACert returns the CA certificate PEM.
func (m *Manager) CACert() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.caCert
}

// CAFingerprint returns the CA certificate fingerprint.
func (m *Manager) CAFingerprint() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	fp, _ := Fingerprint(m.caCert)
	return fp
}

// ServerCert returns the server certificate PEM.
func (m *Manager) ServerCert() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.serverCert
}

// ServerFingerprint returns the server certificate fingerprint.
func (m *Manager) ServerFingerprint() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	fp, _ := Fingerprint(m.serverCert)
	return fp
}

// GenerateClientCert generates a new client certificate for a user.
func (m *Manager) GenerateClientCert(name, userID, sshFingerprint string) (cert, key []byte, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg := m.generatorConfig()
	cfg.ClientCommonName = name
	cfg.UserID = userID
	cfg.SSHKeyFingerprint = sshFingerprint

	return GenerateClientCert(m.caCert, m.caKey, cfg)
}

// RevocationList returns the revocation list.
func (m *Manager) RevocationList() *RevocationList {
	return m.revocationList
}

// TrustStore returns the trust store.
func (m *Manager) TrustStore() *TrustStore {
	return m.trustStore
}

// CheckRenewal checks if certificates need renewal and renews them if needed.
// This should be called periodically (e.g., daily).
func (m *Manager) CheckRenewal() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	renewalThreshold := time.Duration(m.cfg.RenewalThresholdDays) * 24 * time.Hour

	needsRenewal, err := NeedsRenewal(m.serverCert, renewalThreshold)
	if err != nil || needsRenewal {
		if err := m.generateServerCert(); err != nil {
			return fmt.Errorf("failed to renew server cert: %w", err)
		}

		// Reinitialize TLS config
		if err := m.initializeTLSConfig(); err != nil {
			return fmt.Errorf("failed to reinitialize TLS: %w", err)
		}
	}

	return nil
}

// VerifyClientCert verifies a client certificate.
func (m *Manager) VerifyClientCert(certPEM []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Verify chain
	if err := VerifyChain(certPEM, m.caCert); err != nil {
		return fmt.Errorf("certificate chain verification failed: %w", err)
	}

	// Check revocation
	cert, err := ParseCertificate(certPEM)
	if err != nil {
		return err
	}

	if m.revocationList.IsRevoked(cert.Fingerprint) {
		return fmt.Errorf("certificate has been revoked")
	}

	return nil
}

// parseIP extracts an IP address from a listen address string.
func parseIP(addr string) []byte {
	// Handle multiaddr format like /ip4/0.0.0.0/tcp/4001
	// or simple format like 0.0.0.0:4001
	// This is a simplified parser
	return nil // TODO: implement multiaddr parsing if needed
}

// Close cleans up resources.
func (m *Manager) Close() error {
	// Clear sensitive data from memory
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.caKey {
		m.caKey[i] = 0
	}
	m.caKey = nil

	return nil
}
