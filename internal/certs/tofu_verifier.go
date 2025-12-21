package certs

import (
	"bufio"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// TOFUVerifier handles Trust-On-First-Use verification with user prompts
type TOFUVerifier struct {
	// Store is the trust store for persisting trusted nodes
	Store *TrustStore

	// AutoTrust enables automatic trusting without prompts
	AutoTrust bool

	// Input is the source for user input (default: os.Stdin)
	Input io.Reader

	// Output is the destination for prompts (default: os.Stdout)
	Output io.Writer
}

// NewTOFUVerifier creates a new TOFU verifier
func NewTOFUVerifier(store *TrustStore) *TOFUVerifier {
	return &TOFUVerifier{
		Store:  store,
		Input:  os.Stdin,
		Output: os.Stdout,
	}
}

// WithAutoTrust enables automatic trusting
func (v *TOFUVerifier) WithAutoTrust(auto bool) *TOFUVerifier {
	v.AutoTrust = auto
	return v
}

// VerifyResult contains the result of a TOFU verification
type VerifyResult struct {
	// Trusted indicates if the node is trusted
	Trusted bool

	// NewTrust indicates if this is a newly trusted node
	NewTrust bool

	// Node contains the trusted node info (if trusted)
	Node *TrustedNode

	// Error contains any error message
	Error string

	// FingerprintMismatch indicates a potential MITM attack
	FingerprintMismatch bool
}

// CertInfo contains information about a certificate for display
type CertInfo struct {
	// Fingerprint is the SHA256 fingerprint
	Fingerprint string

	// Subject is the certificate subject
	Subject string

	// Issuer is the certificate issuer
	Issuer string

	// NotBefore is the certificate validity start
	NotBefore time.Time

	// NotAfter is the certificate validity end
	NotAfter time.Time

	// IsCA indicates if this is a CA certificate
	IsCA bool

	// PEM is the PEM-encoded certificate
	PEM string
}

// FingerprintFromCert calculates the SHA256 fingerprint of a certificate
func FingerprintFromCert(cert *x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(hash[:])
}

// ParseCertInfo extracts information from a PEM-encoded certificate
func ParseCertInfo(pemData []byte) (*CertInfo, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	hash := sha256.Sum256(cert.Raw)

	return &CertInfo{
		Fingerprint: hex.EncodeToString(hash[:]),
		Subject:     cert.Subject.String(),
		Issuer:      cert.Issuer.String(),
		NotBefore:   cert.NotBefore,
		NotAfter:    cert.NotAfter,
		IsCA:        cert.IsCA,
		PEM:         string(pemData),
	}, nil
}

// FormatFingerprint formats a fingerprint for display with colons
func FormatFingerprint(fingerprint string) string {
	if len(fingerprint) == 0 {
		return ""
	}

	// Insert colons every 2 characters
	var result strings.Builder
	for i := 0; i < len(fingerprint); i += 2 {
		if i > 0 {
			result.WriteString(":")
		}
		end := i + 2
		if end > len(fingerprint) {
			end = len(fingerprint)
		}
		result.WriteString(strings.ToUpper(fingerprint[i:end]))
	}
	return result.String()
}

// Verify checks if a node with the given certificate should be trusted
func (v *TOFUVerifier) Verify(nodeID, address string, certPEM []byte) (*VerifyResult, error) {
	result := &VerifyResult{}

	// Parse certificate
	certInfo, err := ParseCertInfo(certPEM)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	fingerprint := certInfo.Fingerprint

	// Check if already trusted
	trusted, err := v.Store.IsTrusted(nodeID, fingerprint)
	if err != nil {
		// Fingerprint mismatch - potential MITM
		result.FingerprintMismatch = true
		result.Error = err.Error()
		return result, err
	}

	if trusted {
		// Already trusted, update last seen
		v.Store.UpdateLastSeen(nodeID)
		node, _ := v.Store.Get(nodeID)
		result.Trusted = true
		result.Node = node
		return result, nil
	}

	// New node - need to establish trust
	if v.AutoTrust {
		// Auto-trust mode
		node := &TrustedNode{
			NodeID:      nodeID,
			Fingerprint: fingerprint,
			Certificate: string(certPEM),
			Address:     address,
			TrustMethod: TrustMethodTOFU,
		}

		if err := v.Store.Add(node); err != nil {
			result.Error = err.Error()
			return result, err
		}

		result.Trusted = true
		result.NewTrust = true
		result.Node = node
		return result, nil
	}

	// Interactive mode - prompt user
	if v.promptTrust(nodeID, address, certInfo) {
		node := &TrustedNode{
			NodeID:      nodeID,
			Fingerprint: fingerprint,
			Certificate: string(certPEM),
			Address:     address,
			TrustMethod: TrustMethodTOFU,
		}

		if err := v.Store.Add(node); err != nil {
			result.Error = err.Error()
			return result, err
		}

		result.Trusted = true
		result.NewTrust = true
		result.Node = node
		return result, nil
	}

	// User rejected
	result.Trusted = false
	result.Error = "trust rejected by user"
	return result, nil
}

// promptTrust displays the TOFU prompt and returns true if user accepts
func (v *TOFUVerifier) promptTrust(nodeID, address string, certInfo *CertInfo) bool {
	out := v.Output
	if out == nil {
		out = os.Stdout
	}

	in := v.Input
	if in == nil {
		in = os.Stdin
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "═══════════════════════════════════════════════════════════════")
	fmt.Fprintln(out, "  🔐 New Node Certificate - Trust On First Use (TOFU)")
	fmt.Fprintln(out, "═══════════════════════════════════════════════════════════════")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  Node ID:     %s\n", nodeID)
	fmt.Fprintf(out, "  Address:     %s\n", address)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  Certificate:\n")
	fmt.Fprintf(out, "    Subject:     %s\n", certInfo.Subject)
	fmt.Fprintf(out, "    Issuer:      %s\n", certInfo.Issuer)
	fmt.Fprintf(out, "    Valid From:  %s\n", certInfo.NotBefore.Format(time.RFC3339))
	fmt.Fprintf(out, "    Valid Until: %s\n", certInfo.NotAfter.Format(time.RFC3339))
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  Fingerprint (SHA256):\n")
	fmt.Fprintf(out, "    %s\n", FormatFingerprint(certInfo.Fingerprint))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "───────────────────────────────────────────────────────────────")
	fmt.Fprintln(out, "  ⚠️  WARNING: This is the first time connecting to this node.")
	fmt.Fprintln(out, "  Verify the fingerprint matches what the server operator provided.")
	fmt.Fprintln(out, "───────────────────────────────────────────────────────────────")
	fmt.Fprintln(out)
	fmt.Fprint(out, "  Do you want to trust this certificate? [y/N]: ")

	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// DisplayMismatchWarning displays a warning about fingerprint mismatch
func (v *TOFUVerifier) DisplayMismatchWarning(nodeID, address, expectedFP, actualFP string) {
	out := v.Output
	if out == nil {
		out = os.Stdout
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "═══════════════════════════════════════════════════════════════")
	fmt.Fprintln(out, "  🚨 SECURITY WARNING: Certificate Fingerprint Mismatch!")
	fmt.Fprintln(out, "═══════════════════════════════════════════════════════════════")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  Node ID: %s\n", nodeID)
	fmt.Fprintf(out, "  Address: %s\n", address)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  Expected Fingerprint:\n")
	fmt.Fprintf(out, "    %s\n", FormatFingerprint(expectedFP))
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  Received Fingerprint:\n")
	fmt.Fprintf(out, "    %s\n", FormatFingerprint(actualFP))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "───────────────────────────────────────────────────────────────")
	fmt.Fprintln(out, "  This could indicate a Man-In-The-Middle (MITM) attack!")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  Possible causes:")
	fmt.Fprintln(out, "    • The server's certificate was rotated")
	fmt.Fprintln(out, "    • You're connecting to a different server")
	fmt.Fprintln(out, "    • Your connection is being intercepted")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  If you expect this change, remove the old trust with:")
	fmt.Fprintf(out, "    bib trust remove %s\n", nodeID)
	fmt.Fprintln(out, "───────────────────────────────────────────────────────────────")
	fmt.Fprintln(out)
}

// DisplayNewTrust displays information about a newly trusted node
func (v *TOFUVerifier) DisplayNewTrust(node *TrustedNode) {
	out := v.Output
	if out == nil {
		out = os.Stdout
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "✓ Certificate trusted and saved")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  Node ID:     %s\n", node.NodeID)
	fmt.Fprintf(out, "  Fingerprint: %s\n", FormatFingerprint(node.Fingerprint))
	fmt.Fprintf(out, "  Method:      %s\n", node.TrustMethod)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  To pin this certificate (recommended):")
	fmt.Fprintf(out, "    bib trust pin %s\n", node.NodeID)
	fmt.Fprintln(out)
}

// TOFUCallback is a callback function for TOFU verification
type TOFUCallback func(nodeID, address string, certInfo *CertInfo) bool

// VerifyWithCallback verifies using a custom callback for trust decisions
func (v *TOFUVerifier) VerifyWithCallback(nodeID, address string, certPEM []byte, callback TOFUCallback) (*VerifyResult, error) {
	result := &VerifyResult{}

	// Parse certificate
	certInfo, err := ParseCertInfo(certPEM)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	fingerprint := certInfo.Fingerprint

	// Check if already trusted
	trusted, err := v.Store.IsTrusted(nodeID, fingerprint)
	if err != nil {
		result.FingerprintMismatch = true
		result.Error = err.Error()
		return result, err
	}

	if trusted {
		v.Store.UpdateLastSeen(nodeID)
		node, _ := v.Store.Get(nodeID)
		result.Trusted = true
		result.Node = node
		return result, nil
	}

	// Use callback for trust decision
	if callback(nodeID, address, certInfo) {
		node := &TrustedNode{
			NodeID:      nodeID,
			Fingerprint: fingerprint,
			Certificate: string(certPEM),
			Address:     address,
			TrustMethod: TrustMethodTOFU,
		}

		if err := v.Store.Add(node); err != nil {
			result.Error = err.Error()
			return result, err
		}

		result.Trusted = true
		result.NewTrust = true
		result.Node = node
		return result, nil
	}

	result.Trusted = false
	result.Error = "trust rejected"
	return result, nil
}
