package certs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TrustedNode represents a trusted server node for TOFU.
type TrustedNode struct {
	// NodeID is the unique identifier of the node (peer ID)
	NodeID string `json:"node_id"`

	// Fingerprint is the SHA256 fingerprint of the server certificate
	Fingerprint string `json:"fingerprint"`

	// Certificate is the full PEM-encoded certificate (for later comparison)
	Certificate string `json:"certificate"`

	// FirstSeen is when this node was first trusted
	FirstSeen time.Time `json:"first_seen"`

	// LastSeen is when we last connected to this node
	LastSeen time.Time `json:"last_seen"`

	// Alias is an optional user-friendly name
	Alias string `json:"alias,omitempty"`

	// Address is the last known address of this node
	Address string `json:"address,omitempty"`

	// TrustMethod describes how trust was established
	TrustMethod TrustMethod `json:"trust_method"`

	// PinnedAt is set if the user explicitly pinned this certificate
	PinnedAt *time.Time `json:"pinned_at,omitempty"`

	// Verified indicates if this trust was verified out-of-band
	Verified bool `json:"verified"`

	// Notes is optional user notes about this node
	Notes string `json:"notes,omitempty"`
}

// TrustMethod describes how trust was established.
type TrustMethod string

const (
	// TrustMethodTOFU means Trust-On-First-Use (automatically trusted on first connection)
	TrustMethodTOFU TrustMethod = "tofu"

	// TrustMethodManual means the user manually added this trust
	TrustMethodManual TrustMethod = "manual"

	// TrustMethodPinned means the user explicitly pinned this certificate
	TrustMethodPinned TrustMethod = "pinned"
)

// TrustStore manages trusted node certificates for TOFU.
type TrustStore struct {
	dir   string
	nodes map[string]*TrustedNode // keyed by NodeID
	mu    sync.RWMutex
}

// NewTrustStore creates a new trust store at the given directory.
func NewTrustStore(dir string) (*TrustStore, error) {
	ts := &TrustStore{
		dir:   dir,
		nodes: make(map[string]*TrustedNode),
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create trust store directory: %w", err)
	}

	// Load existing trusted nodes
	if err := ts.load(); err != nil {
		return nil, fmt.Errorf("failed to load trust store: %w", err)
	}

	return ts, nil
}

// Get returns a trusted node by ID.
func (ts *TrustStore) Get(nodeID string) (*TrustedNode, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	node, ok := ts.nodes[nodeID]
	return node, ok
}

// GetByFingerprint returns a trusted node by certificate fingerprint.
func (ts *TrustStore) GetByFingerprint(fingerprint string) (*TrustedNode, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	for _, node := range ts.nodes {
		if node.Fingerprint == fingerprint {
			return node, true
		}
	}
	return nil, false
}

// IsTrusted checks if a node is trusted with the given certificate fingerprint.
// Returns an error if the node is known but the fingerprint doesn't match (MITM warning).
func (ts *TrustStore) IsTrusted(nodeID, fingerprint string) (trusted bool, err error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	node, ok := ts.nodes[nodeID]
	if !ok {
		return false, nil // Unknown node, not trusted yet
	}

	if node.Fingerprint != fingerprint {
		return false, fmt.Errorf("certificate fingerprint mismatch for node %s: "+
			"expected %s, got %s (possible MITM attack)", nodeID, node.Fingerprint, fingerprint)
	}

	return true, nil
}

// Add adds a new trusted node or updates an existing one.
func (ts *TrustStore) Add(node *TrustedNode) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Set FirstSeen if this is a new node
	if existing, ok := ts.nodes[node.NodeID]; ok {
		node.FirstSeen = existing.FirstSeen
	} else if node.FirstSeen.IsZero() {
		node.FirstSeen = time.Now()
	}

	if node.LastSeen.IsZero() {
		node.LastSeen = time.Now()
	}

	ts.nodes[node.NodeID] = node
	return ts.save(node)
}

// UpdateLastSeen updates the last seen timestamp for a node.
func (ts *TrustStore) UpdateLastSeen(nodeID string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	node, ok := ts.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %s not found", nodeID)
	}

	node.LastSeen = time.Now()
	return ts.save(node)
}

// Remove removes a trusted node.
func (ts *TrustStore) Remove(nodeID string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if _, ok := ts.nodes[nodeID]; !ok {
		return fmt.Errorf("node %s not found", nodeID)
	}

	delete(ts.nodes, nodeID)
	return os.Remove(ts.nodePath(nodeID))
}

// List returns all trusted nodes.
func (ts *TrustStore) List() []*TrustedNode {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	nodes := make([]*TrustedNode, 0, len(ts.nodes))
	for _, node := range ts.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// Pin marks a node's certificate as explicitly pinned.
func (ts *TrustStore) Pin(nodeID string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	node, ok := ts.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %s not found", nodeID)
	}

	now := time.Now()
	node.PinnedAt = &now
	node.TrustMethod = TrustMethodPinned
	return ts.save(node)
}

// Verify marks a node's trust as verified out-of-band.
func (ts *TrustStore) Verify(nodeID string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	node, ok := ts.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %s not found", nodeID)
	}

	node.Verified = true
	return ts.save(node)
}

// SetAlias sets a user-friendly alias for a node.
func (ts *TrustStore) SetAlias(nodeID, alias string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	node, ok := ts.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %s not found", nodeID)
	}

	node.Alias = alias
	return ts.save(node)
}

// nodePath returns the file path for a node's trust data.
func (ts *TrustStore) nodePath(nodeID string) string {
	// Sanitize node ID for filesystem (replace / with _)
	safe := sanitizeFilename(nodeID)
	return filepath.Join(ts.dir, safe+".json")
}

// save persists a single node's trust data.
func (ts *TrustStore) save(node *TrustedNode) error {
	data, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal trust data: %w", err)
	}

	path := ts.nodePath(node.NodeID)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write trust data: %w", err)
	}

	return nil
}

// load loads all trusted nodes from disk.
func (ts *TrustStore) load() error {
	entries, err := os.ReadDir(ts.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No trust store yet
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(ts.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip unreadable files
		}

		var node TrustedNode
		if err := json.Unmarshal(data, &node); err != nil {
			continue // Skip invalid files
		}

		ts.nodes[node.NodeID] = &node
	}

	return nil
}

// sanitizeFilename makes a string safe for use as a filename.
func sanitizeFilename(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}
