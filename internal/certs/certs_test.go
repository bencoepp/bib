package certs

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestGenerateCA(t *testing.T) {
	cfg := DefaultConfig("test-node-id-12345678")

	caCert, caKey, err := GenerateCA(cfg)
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}

	// Verify CA certificate
	block, _ := pem.Decode(caCert)
	if block == nil {
		t.Fatal("failed to decode CA certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}

	if !cert.IsCA {
		t.Error("CA certificate should have IsCA=true")
	}

	if cert.Subject.CommonName != cfg.CACommonName {
		t.Errorf("CA common name mismatch: got %s, want %s", cert.Subject.CommonName, cfg.CACommonName)
	}

	// Verify key
	keyBlock, _ := pem.Decode(caKey)
	if keyBlock == nil {
		t.Fatal("failed to decode CA key")
	}

	if keyBlock.Type != "EC PRIVATE KEY" {
		t.Errorf("unexpected key type: %s", keyBlock.Type)
	}
}

func TestGenerateServerCert(t *testing.T) {
	cfg := DefaultConfig("test-node-id-12345678")
	cfg.PeerID = "12D3KooWTestPeerID"

	caCert, caKey, err := GenerateCA(cfg)
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}

	serverCert, serverKey, err := GenerateServerCert(caCert, caKey, cfg)
	if err != nil {
		t.Fatalf("failed to generate server cert: %v", err)
	}

	// Parse and verify
	block, _ := pem.Decode(serverCert)
	if block == nil {
		t.Fatal("failed to decode server certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse server certificate: %v", err)
	}

	// Check server auth usage
	hasServerAuth := false
	for _, usage := range cert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			hasServerAuth = true
			break
		}
	}
	if !hasServerAuth {
		t.Error("server certificate should have ExtKeyUsageServerAuth")
	}

	// Check that peer ID is in SANs
	hasPeerID := false
	for _, dns := range cert.DNSNames {
		if dns == cfg.PeerID {
			hasPeerID = true
			break
		}
	}
	if !hasPeerID {
		t.Errorf("server certificate should have peer ID in DNS names: %v", cert.DNSNames)
	}

	// Verify chain
	if err := VerifyChain(serverCert, caCert); err != nil {
		t.Errorf("server certificate should be signed by CA: %v", err)
	}

	// Verify key exists
	keyBlock, _ := pem.Decode(serverKey)
	if keyBlock == nil {
		t.Fatal("failed to decode server key")
	}
}

func TestGenerateClientCert(t *testing.T) {
	cfg := DefaultConfig("test-node-id-12345678")
	cfg.UserID = "user123"
	cfg.SSHKeyFingerprint = "SHA256:abcd1234"

	caCert, caKey, err := GenerateCA(cfg)
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}

	clientCert, clientKey, err := GenerateClientCert(caCert, caKey, cfg)
	if err != nil {
		t.Fatalf("failed to generate client cert: %v", err)
	}

	// Parse and verify
	block, _ := pem.Decode(clientCert)
	if block == nil {
		t.Fatal("failed to decode client certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse client certificate: %v", err)
	}

	// Check client auth usage
	hasClientAuth := false
	for _, usage := range cert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			hasClientAuth = true
			break
		}
	}
	if !hasClientAuth {
		t.Error("client certificate should have ExtKeyUsageClientAuth")
	}

	// Check SSH fingerprint binding
	if cert.Subject.SerialNumber != "ssh-fp:"+cfg.SSHKeyFingerprint {
		t.Errorf("SSH fingerprint not in subject: got %s, want ssh-fp:%s",
			cert.Subject.SerialNumber, cfg.SSHKeyFingerprint)
	}

	// Check user ID in OU
	hasUserID := false
	for _, ou := range cert.Subject.OrganizationalUnit {
		if ou == "user:"+cfg.UserID {
			hasUserID = true
			break
		}
	}
	if !hasUserID {
		t.Errorf("user ID not in organizational unit: %v", cert.Subject.OrganizationalUnit)
	}

	// Verify chain with client auth key usage
	if err := VerifyChainWithUsage(clientCert, caCert, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}); err != nil {
		t.Errorf("client certificate should be signed by CA: %v", err)
	}

	// Verify key exists
	keyBlock, _ := pem.Decode(clientKey)
	if keyBlock == nil {
		t.Fatal("failed to decode client key")
	}
}

func TestGenerateBundle(t *testing.T) {
	cfg := DefaultConfig("test-node-id-12345678")

	bundle, err := GenerateBundle(cfg)
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	if bundle.CACert == nil {
		t.Error("bundle should have CA cert")
	}
	if bundle.CAKey == nil {
		t.Error("bundle should have CA key")
	}
	if bundle.ServerCert == nil {
		t.Error("bundle should have server cert")
	}
	if bundle.ServerKey == nil {
		t.Error("bundle should have server key")
	}
	if bundle.ClientCert == nil {
		t.Error("bundle should have client cert")
	}
	if bundle.ClientKey == nil {
		t.Error("bundle should have client key")
	}
}

func TestSaveAndLoadBundle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-certs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig("test-node-id-12345678")
	bundle, err := GenerateBundle(cfg)
	if err != nil {
		t.Fatalf("failed to generate bundle: %v", err)
	}

	// Save
	if err := bundle.SaveToDir(tmpDir); err != nil {
		t.Fatalf("failed to save bundle: %v", err)
	}

	// Verify files exist
	files := []string{"ca.crt", "ca.key", "server.crt", "server.key", "client.crt", "client.key"}
	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Verify key files have restricted permissions (Unix only)
	if runtime.GOOS != "windows" {
		for _, f := range []string{"ca.key", "server.key", "client.key"} {
			path := filepath.Join(tmpDir, f)
			info, err := os.Stat(path)
			if err != nil {
				t.Errorf("failed to stat %s: %v", f, err)
				continue
			}
			mode := info.Mode().Perm()
			if mode != 0600 {
				t.Errorf("key file %s has permissions %o, want 0600", f, mode)
			}
		}
	}

	// Load
	loaded, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to load bundle: %v", err)
	}

	if string(loaded.CACert) != string(bundle.CACert) {
		t.Error("loaded CA cert doesn't match")
	}
	if string(loaded.ServerCert) != string(bundle.ServerCert) {
		t.Error("loaded server cert doesn't match")
	}
}

func TestFingerprint(t *testing.T) {
	cfg := DefaultConfig("test-node-id-12345678")

	caCert, _, err := GenerateCA(cfg)
	if err != nil {
		t.Fatalf("failed to generate CA: %v", err)
	}

	fp1, err := Fingerprint(caCert)
	if err != nil {
		t.Fatalf("failed to calculate fingerprint: %v", err)
	}

	if len(fp1) != 64 { // SHA256 in hex is 64 characters
		t.Errorf("unexpected fingerprint length: %d", len(fp1))
	}

	// Same cert should have same fingerprint
	fp2, _ := Fingerprint(caCert)
	if fp1 != fp2 {
		t.Error("same cert should have same fingerprint")
	}

	// Different cert should have different fingerprint
	caCert2, _, _ := GenerateCA(cfg)
	fp3, _ := Fingerprint(caCert2)
	if fp1 == fp3 {
		t.Error("different certs should have different fingerprints")
	}
}

func TestNeedsRenewal(t *testing.T) {
	cfg := DefaultConfig("test-node-id-12345678")
	cfg.ServerValidDuration = 60 * 24 * time.Hour // 60 days

	caCert, caKey, _ := GenerateCA(cfg)
	serverCert, _, _ := GenerateServerCert(caCert, caKey, cfg)

	// Certificate with 60 days validity should not need renewal at 30 day threshold
	needs, err := NeedsRenewal(serverCert, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("failed to check renewal: %v", err)
	}
	if needs {
		t.Error("certificate should not need renewal")
	}

	// Should need renewal at 90 day threshold (cert only valid for 60 days)
	needs, _ = NeedsRenewal(serverCert, 90*24*time.Hour)
	if !needs {
		t.Error("certificate should need renewal")
	}
}

func TestParseCertificate(t *testing.T) {
	cfg := DefaultConfig("test-node-id-12345678")
	cfg.PeerID = "12D3KooWTestPeerID"

	caCert, caKey, _ := GenerateCA(cfg)
	serverCert, _, _ := GenerateServerCert(caCert, caKey, cfg)

	cert, err := ParseCertificate(serverCert)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	if cert.Fingerprint == "" {
		t.Error("certificate should have fingerprint")
	}

	if cert.Subject == "" {
		t.Error("certificate should have subject")
	}

	if cert.ExpiresAt.IsZero() {
		t.Error("certificate should have expiry time")
	}

	if len(cert.DNSNames) == 0 {
		t.Error("certificate should have DNS names")
	}
}
