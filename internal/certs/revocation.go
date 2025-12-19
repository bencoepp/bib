package certs

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RevokedCertificate represents a revoked certificate entry.
type RevokedCertificate struct {
	// Fingerprint is the SHA256 fingerprint of the revoked certificate
	Fingerprint string `json:"fingerprint"`

	// SerialHex is the certificate serial number in hex
	SerialHex string `json:"serial_hex"`

	// Subject is the certificate subject
	Subject string `json:"subject"`

	// RevokedAt is when the certificate was revoked
	RevokedAt time.Time `json:"revoked_at"`

	// Reason is the revocation reason
	Reason RevocationReason `json:"reason"`

	// RevokedBy is the user/node that revoked this certificate
	RevokedBy string `json:"revoked_by"`

	// ExpiresAt is when the original certificate would have expired
	// (can be used to prune old entries)
	ExpiresAt time.Time `json:"expires_at"`

	// Notes is optional notes about the revocation
	Notes string `json:"notes,omitempty"`
}

// RevocationReason describes why a certificate was revoked.
type RevocationReason string

const (
	ReasonUnspecified        RevocationReason = "unspecified"
	ReasonKeyCompromise      RevocationReason = "key_compromise"
	ReasonCACompromise       RevocationReason = "ca_compromise"
	ReasonAffiliationChanged RevocationReason = "affiliation_changed"
	ReasonSuperseded         RevocationReason = "superseded"
	ReasonCessationOfOps     RevocationReason = "cessation_of_operation"
	ReasonCertificateHold    RevocationReason = "certificate_hold"
	ReasonRemoveFromCRL      RevocationReason = "remove_from_crl"
	ReasonPrivilegeWithdrawn RevocationReason = "privilege_withdrawn"
	ReasonAACompromise       RevocationReason = "aa_compromise"
)

// RevocationList manages revoked certificates.
type RevocationList struct {
	path    string
	entries map[string]*RevokedCertificate // keyed by fingerprint
	version uint64                         // Incremented on each change (for sync)
	mu      sync.RWMutex
}

// RevocationListData is the serializable form of the revocation list.
type RevocationListData struct {
	Version   uint64                `json:"version"`
	UpdatedAt time.Time             `json:"updated_at"`
	Entries   []*RevokedCertificate `json:"entries"`
}

// NewRevocationList creates or loads a revocation list.
func NewRevocationList(path string) (*RevocationList, error) {
	rl := &RevocationList{
		path:    path,
		entries: make(map[string]*RevokedCertificate),
	}

	if err := rl.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load revocation list: %w", err)
	}

	return rl, nil
}

// IsRevoked checks if a certificate is revoked by its fingerprint.
func (rl *RevocationList) IsRevoked(fingerprint string) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	_, ok := rl.entries[fingerprint]
	return ok
}

// IsCertRevoked checks if a parsed certificate is revoked.
func (rl *RevocationList) IsCertRevoked(cert *x509.Certificate) bool {
	fp := CertFingerprint(cert)
	return rl.IsRevoked(fp)
}

// Revoke adds a certificate to the revocation list.
func (rl *RevocationList) Revoke(entry *RevokedCertificate) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if entry.RevokedAt.IsZero() {
		entry.RevokedAt = time.Now()
	}

	rl.entries[entry.Fingerprint] = entry
	rl.version++

	return rl.save()
}

// RevokeCert revokes a certificate given its PEM data.
func (rl *RevocationList) RevokeCert(certPEM []byte, reason RevocationReason, revokedBy, notes string) error {
	cert, err := ParseCertificate(certPEM)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	entry := &RevokedCertificate{
		Fingerprint: cert.Fingerprint,
		SerialHex:   cert.SerialHex,
		Subject:     cert.Subject,
		RevokedAt:   time.Now(),
		Reason:      reason,
		RevokedBy:   revokedBy,
		ExpiresAt:   cert.ExpiresAt,
		Notes:       notes,
	}

	return rl.Revoke(entry)
}

// Remove removes a certificate from the revocation list.
func (rl *RevocationList) Remove(fingerprint string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if _, ok := rl.entries[fingerprint]; !ok {
		return fmt.Errorf("certificate not in revocation list")
	}

	delete(rl.entries, fingerprint)
	rl.version++

	return rl.save()
}

// List returns all revoked certificates.
func (rl *RevocationList) List() []*RevokedCertificate {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	result := make([]*RevokedCertificate, 0, len(rl.entries))
	for _, entry := range rl.entries {
		result = append(result, entry)
	}
	return result
}

// Version returns the current version of the revocation list.
func (rl *RevocationList) Version() uint64 {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return rl.version
}

// Prune removes expired entries older than the given duration.
func (rl *RevocationList) Prune(olderThan time.Duration) (int, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	pruned := 0

	for fp, entry := range rl.entries {
		if entry.ExpiresAt.Before(cutoff) {
			delete(rl.entries, fp)
			pruned++
		}
	}

	if pruned > 0 {
		rl.version++
		if err := rl.save(); err != nil {
			return pruned, err
		}
	}

	return pruned, nil
}

// Export returns the revocation list as serializable data for P2P sync.
func (rl *RevocationList) Export() *RevocationListData {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return rl.exportLocked()
}

// exportLocked returns the revocation list data without acquiring locks.
// Caller must hold at least a read lock.
func (rl *RevocationList) exportLocked() *RevocationListData {
	entries := make([]*RevokedCertificate, 0, len(rl.entries))
	for _, entry := range rl.entries {
		entries = append(entries, entry)
	}

	return &RevocationListData{
		Version:   rl.version,
		UpdatedAt: time.Now(),
		Entries:   entries,
	}
}

// Merge merges remote revocation list data into this list.
// Used for P2P synchronization - adds new entries but doesn't remove existing ones.
func (rl *RevocationList) Merge(data *RevocationListData) (added int, err error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for _, entry := range data.Entries {
		if _, exists := rl.entries[entry.Fingerprint]; !exists {
			rl.entries[entry.Fingerprint] = entry
			added++
		}
	}

	if added > 0 {
		rl.version++
		if err := rl.save(); err != nil {
			return added, err
		}
	}

	return added, nil
}

// save persists the revocation list to disk.
func (rl *RevocationList) save() error {
	dir := filepath.Dir(rl.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data := rl.exportLocked()
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal revocation list: %w", err)
	}

	if err := os.WriteFile(rl.path, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write revocation list: %w", err)
	}

	return nil
}

// load loads the revocation list from disk.
func (rl *RevocationList) load() error {
	data, err := os.ReadFile(rl.path)
	if err != nil {
		return err
	}

	// Handle empty file
	if len(data) == 0 {
		return nil
	}

	var listData RevocationListData
	if err := json.Unmarshal(data, &listData); err != nil {
		return fmt.Errorf("failed to unmarshal revocation list: %w", err)
	}

	rl.version = listData.Version
	for _, entry := range listData.Entries {
		rl.entries[entry.Fingerprint] = entry
	}

	return nil
}

// CertFingerprint calculates the SHA256 fingerprint of a parsed certificate.
func CertFingerprint(cert *x509.Certificate) string {
	info, _ := ParseCertificate(cert.Raw)
	if info != nil {
		return info.Fingerprint
	}
	return ""
}
