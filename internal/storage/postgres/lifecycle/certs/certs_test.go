package certs

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	cfg := DefaultGeneratorConfig("test-node-id-12345678")

	bundle, err := Generate(cfg)
	if err != nil {
		t.Fatalf("failed to generate certificates: %v", err)
	}

	// Verify CA certificate
	block, _ := pem.Decode(bundle.CACert)
	if block == nil {
		t.Fatal("failed to decode CA certificate")
	}

	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}

	if !caCert.IsCA {
		t.Error("CA certificate should have IsCA=true")
	}

	if caCert.Subject.CommonName != cfg.CACommonName {
		t.Errorf("CA common name mismatch: got %s, want %s", caCert.Subject.CommonName, cfg.CACommonName)
	}

	// Verify server certificate
	block, _ = pem.Decode(bundle.ServerCert)
	if block == nil {
		t.Fatal("failed to decode server certificate")
	}

	serverCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse server certificate: %v", err)
	}

	if serverCert.Subject.CommonName != cfg.ServerCommonName {
		t.Errorf("server common name mismatch: got %s, want %s", serverCert.Subject.CommonName, cfg.ServerCommonName)
	}

	// Verify server certificate has ServerAuth extended key usage
	hasServerAuth := false
	for _, usage := range serverCert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			hasServerAuth = true
			break
		}
	}
	if !hasServerAuth {
		t.Error("server certificate should have ExtKeyUsageServerAuth")
	}

	// Verify client certificate
	block, _ = pem.Decode(bundle.ClientCert)
	if block == nil {
		t.Fatal("failed to decode client certificate")
	}

	clientCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse client certificate: %v", err)
	}

	// Verify client certificate has ClientAuth extended key usage
	hasClientAuth := false
	for _, usage := range clientCert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			hasClientAuth = true
			break
		}
	}
	if !hasClientAuth {
		t.Error("client certificate should have ExtKeyUsageClientAuth")
	}

	// Verify server certificate is signed by CA
	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	if _, err := serverCert.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
		t.Errorf("server certificate not signed by CA: %v", err)
	}

	// Verify client certificate is signed by CA
	if _, err := clientCert.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Errorf("client certificate not signed by CA: %v", err)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-certs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultGeneratorConfig("test-node-id-12345678")
	bundle, err := Generate(cfg)
	if err != nil {
		t.Fatalf("failed to generate certificates: %v", err)
	}

	// Save
	if err := bundle.SaveToDir(tmpDir); err != nil {
		t.Fatalf("failed to save certificates: %v", err)
	}

	// Verify files exist
	files := []string{"ca.crt", "ca.key", "server.crt", "server.key", "client.crt", "client.key"}
	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Verify key files have restricted permissions
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

	// Load
	loaded, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to load certificates: %v", err)
	}

	// Verify loaded matches original
	if string(loaded.CACert) != string(bundle.CACert) {
		t.Error("loaded CA cert doesn't match original")
	}
	if string(loaded.ServerCert) != string(bundle.ServerCert) {
		t.Error("loaded server cert doesn't match original")
	}
	if string(loaded.ClientCert) != string(bundle.ClientCert) {
		t.Error("loaded client cert doesn't match original")
	}
}

func TestExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-certs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Empty directory
	if Exists(tmpDir) {
		t.Error("Exists should return false for empty directory")
	}

	// Generate and save
	cfg := DefaultGeneratorConfig("test-node-id-12345678")
	bundle, _ := Generate(cfg)
	bundle.SaveToDir(tmpDir)

	if !Exists(tmpDir) {
		t.Error("Exists should return true after saving certificates")
	}
}

func TestNeedsRotation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-certs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate with 1 year validity
	cfg := DefaultGeneratorConfig("test-node-id-12345678")
	cfg.ValidDuration = 365 * 24 * time.Hour
	bundle, _ := Generate(cfg)
	bundle.SaveToDir(tmpDir)

	// Should not need rotation with 30 day threshold
	needsRotation, err := NeedsRotation(tmpDir, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("NeedsRotation failed: %v", err)
	}
	if needsRotation {
		t.Error("certificates should not need rotation")
	}

	// Generate with 7 day validity
	cfg.ValidDuration = 7 * 24 * time.Hour
	bundle, _ = Generate(cfg)
	bundle.SaveToDir(tmpDir)

	// Should need rotation with 30 day threshold
	needsRotation, err = NeedsRotation(tmpDir, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("NeedsRotation failed: %v", err)
	}
	if !needsRotation {
		t.Error("certificates should need rotation")
	}
}
