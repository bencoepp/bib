package certs

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"
)

// generateTestCert generates a self-signed test certificate
func generateTestCert(commonName string) ([]byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return pemBytes, nil
}

func TestTOFUFingerprint(t *testing.T) {
	certPEM, err := generateTestCert("test.example.com")
	if err != nil {
		t.Fatalf("failed to generate test cert: %v", err)
	}

	fp, err := Fingerprint(certPEM)
	if err != nil {
		t.Fatalf("failed to calculate fingerprint: %v", err)
	}

	// Fingerprint should be 64 hex characters (SHA256)
	if len(fp) != 64 {
		t.Errorf("expected fingerprint length 64, got %d", len(fp))
	}

	// Should be consistent
	fp2, _ := Fingerprint(certPEM)
	if fp != fp2 {
		t.Error("fingerprint should be deterministic")
	}
}

func TestTOFUFingerprint_Invalid(t *testing.T) {
	_, err := Fingerprint([]byte("invalid pem data"))
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func TestParseCertInfo(t *testing.T) {
	certPEM, err := generateTestCert("test.example.com")
	if err != nil {
		t.Fatalf("failed to generate test cert: %v", err)
	}

	info, err := ParseCertInfo(certPEM)
	if err != nil {
		t.Fatalf("failed to parse cert info: %v", err)
	}

	if info.Fingerprint == "" {
		t.Error("fingerprint should not be empty")
	}

	if !strings.Contains(info.Subject, "test.example.com") {
		t.Errorf("subject should contain common name, got %s", info.Subject)
	}

	if info.NotBefore.IsZero() {
		t.Error("NotBefore should be set")
	}

	if info.NotAfter.IsZero() {
		t.Error("NotAfter should be set")
	}

	if info.PEM == "" {
		t.Error("PEM should be set")
	}
}

func TestFormatFingerprint(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"ab", "AB"},
		{"abcd", "AB:CD"},
		{"abcdef", "AB:CD:EF"},
		{"abcdef12", "AB:CD:EF:12"},
	}

	for _, tt := range tests {
		result := FormatFingerprint(tt.input)
		if result != tt.expected {
			t.Errorf("FormatFingerprint(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestTOFUVerifier_AutoTrust(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tofu-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewTrustStore(tmpDir)
	verifier := NewTOFUVerifier(store).WithAutoTrust(true)

	certPEM, _ := generateTestCert("node1.example.com")
	nodeID := "12D3KooWTestNode1"
	address := "node1.example.com:4000"

	result, err := verifier.Verify(nodeID, address, certPEM)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	if !result.Trusted {
		t.Error("should be trusted in auto-trust mode")
	}

	if !result.NewTrust {
		t.Error("should be new trust")
	}

	if result.Node == nil {
		t.Error("node should not be nil")
	}

	// Verify it was saved
	node, ok := store.Get(nodeID)
	if !ok {
		t.Error("node should be in store")
	}

	if node.TrustMethod != TrustMethodTOFU {
		t.Errorf("trust method should be TOFU, got %s", node.TrustMethod)
	}
}

func TestTOFUVerifier_AlreadyTrusted(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tofu-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewTrustStore(tmpDir)
	verifier := NewTOFUVerifier(store).WithAutoTrust(true)

	certPEM, _ := generateTestCert("node1.example.com")
	nodeID := "12D3KooWTestNode1"
	address := "node1.example.com:4000"

	// First verification
	result1, _ := verifier.Verify(nodeID, address, certPEM)
	if !result1.NewTrust {
		t.Error("first verification should be new trust")
	}

	// Second verification with same cert
	result2, _ := verifier.Verify(nodeID, address, certPEM)
	if !result2.Trusted {
		t.Error("should still be trusted")
	}
	if result2.NewTrust {
		t.Error("second verification should not be new trust")
	}
}

func TestTOFUVerifier_FingerprintMismatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tofu-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewTrustStore(tmpDir)
	verifier := NewTOFUVerifier(store).WithAutoTrust(true)

	certPEM1, _ := generateTestCert("node1.example.com")
	certPEM2, _ := generateTestCert("node1.example.com") // Different cert, different fingerprint

	nodeID := "12D3KooWTestNode1"
	address := "node1.example.com:4000"

	// First verification
	verifier.Verify(nodeID, address, certPEM1)

	// Second verification with different cert
	result, err := verifier.Verify(nodeID, address, certPEM2)
	if err == nil {
		t.Error("should return error for fingerprint mismatch")
	}

	if !result.FingerprintMismatch {
		t.Error("FingerprintMismatch should be true")
	}

	if result.Trusted {
		t.Error("should not be trusted with mismatched fingerprint")
	}
}

func TestTOFUVerifier_Interactive_Accept(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tofu-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewTrustStore(tmpDir)
	verifier := NewTOFUVerifier(store)

	// Simulate user input
	verifier.Input = strings.NewReader("y\n")
	verifier.Output = &bytes.Buffer{}

	certPEM, _ := generateTestCert("node1.example.com")
	nodeID := "12D3KooWTestNode1"
	address := "node1.example.com:4000"

	result, err := verifier.Verify(nodeID, address, certPEM)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	if !result.Trusted {
		t.Error("should be trusted after user accepts")
	}

	if !result.NewTrust {
		t.Error("should be new trust")
	}
}

func TestTOFUVerifier_Interactive_Reject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tofu-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewTrustStore(tmpDir)
	verifier := NewTOFUVerifier(store)

	// Simulate user rejection
	verifier.Input = strings.NewReader("n\n")
	verifier.Output = &bytes.Buffer{}

	certPEM, _ := generateTestCert("node1.example.com")
	nodeID := "12D3KooWTestNode1"
	address := "node1.example.com:4000"

	result, _ := verifier.Verify(nodeID, address, certPEM)

	if result.Trusted {
		t.Error("should not be trusted after user rejects")
	}

	// Verify it was not saved
	_, ok := store.Get(nodeID)
	if ok {
		t.Error("node should not be in store after rejection")
	}
}

func TestTOFUVerifier_PromptOutput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tofu-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, _ := NewTrustStore(tmpDir)
	verifier := NewTOFUVerifier(store)

	var output bytes.Buffer
	verifier.Input = strings.NewReader("n\n")
	verifier.Output = &output

	certPEM, _ := generateTestCert("node1.example.com")
	verifier.Verify("12D3KooWTest", "node1:4000", certPEM)

	outputStr := output.String()

	// Check for expected content in prompt
	if !strings.Contains(outputStr, "TOFU") {
		t.Error("output should mention TOFU")
	}

	if !strings.Contains(outputStr, "12D3KooWTest") {
		t.Error("output should contain node ID")
	}

	if !strings.Contains(outputStr, "node1:4000") {
		t.Error("output should contain address")
	}

	if !strings.Contains(outputStr, "Fingerprint") {
		t.Error("output should contain fingerprint")
	}

	if !strings.Contains(outputStr, "WARNING") {
		t.Error("output should contain warning")
	}
}

func TestTOFUVerifier_DisplayMismatchWarning(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "tofu-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewTrustStore(tmpDir)
	verifier := NewTOFUVerifier(store)

	var output bytes.Buffer
	verifier.Output = &output

	verifier.DisplayMismatchWarning(
		"12D3KooWTest",
		"node1:4000",
		"abc123",
		"def456",
	)

	outputStr := output.String()

	if !strings.Contains(outputStr, "SECURITY WARNING") {
		t.Error("output should contain security warning")
	}

	if !strings.Contains(outputStr, "MITM") {
		t.Error("output should mention MITM")
	}

	if !strings.Contains(outputStr, "Expected") {
		t.Error("output should show expected fingerprint")
	}

	if !strings.Contains(outputStr, "bib trust remove") {
		t.Error("output should show how to remove trust")
	}
}

func TestTOFUVerifier_DisplayNewTrust(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "tofu-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewTrustStore(tmpDir)
	verifier := NewTOFUVerifier(store)

	var output bytes.Buffer
	verifier.Output = &output

	node := &TrustedNode{
		NodeID:      "12D3KooWTest",
		Fingerprint: "abc123def456",
		TrustMethod: TrustMethodTOFU,
	}

	verifier.DisplayNewTrust(node)

	outputStr := output.String()

	if !strings.Contains(outputStr, "trusted and saved") {
		t.Error("output should confirm trust saved")
	}

	if !strings.Contains(outputStr, "bib trust pin") {
		t.Error("output should suggest pinning")
	}
}

func TestTOFUVerifier_WithCallback(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "tofu-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := NewTrustStore(tmpDir)
	verifier := NewTOFUVerifier(store)

	certPEM, _ := generateTestCert("node1.example.com")
	nodeID := "12D3KooWTestNode1"
	address := "node1.example.com:4000"

	// Test with accepting callback
	callbackCalled := false
	result, _ := verifier.VerifyWithCallback(nodeID, address, certPEM, func(id, addr string, info *CertInfo) bool {
		callbackCalled = true
		return true // Accept
	})

	if !callbackCalled {
		t.Error("callback should be called")
	}

	if !result.Trusted {
		t.Error("should be trusted when callback returns true")
	}
}

func TestVerifyResult_Fields(t *testing.T) {
	result := VerifyResult{
		Trusted:             true,
		NewTrust:            true,
		Node:                &TrustedNode{NodeID: "test"},
		Error:               "",
		FingerprintMismatch: false,
	}

	if !result.Trusted {
		t.Error("Trusted should be true")
	}
	if !result.NewTrust {
		t.Error("NewTrust should be true")
	}
	if result.Node == nil {
		t.Error("Node should not be nil")
	}
}

func TestCertInfo_Fields(t *testing.T) {
	info := CertInfo{
		Fingerprint: "abc123",
		Subject:     "CN=test",
		Issuer:      "CN=issuer",
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(24 * time.Hour),
		IsCA:        false,
		PEM:         "-----BEGIN CERTIFICATE-----",
	}

	if info.Fingerprint != "abc123" {
		t.Error("Fingerprint mismatch")
	}
	if info.Subject != "CN=test" {
		t.Error("Subject mismatch")
	}
}
