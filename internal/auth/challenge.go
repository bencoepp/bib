// Package auth provides user authentication and session management for bibd.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ChallengeStore manages authentication challenges with automatic expiry.
// Challenges are stored in-memory and are not persisted across restarts.
type ChallengeStore struct {
	mu         sync.RWMutex
	challenges map[string]*Challenge
	ttl        time.Duration
	stopCh     chan struct{}
}

// Challenge represents an authentication challenge.
type Challenge struct {
	// ID is the unique challenge identifier.
	ID string

	// PublicKey is the public key bytes that must sign the challenge.
	PublicKey []byte

	// KeyType is the type of public key (ed25519, rsa).
	KeyType string

	// Nonce is the random bytes to be signed.
	Nonce []byte

	// CreatedAt is when the challenge was created.
	CreatedAt time.Time

	// ExpiresAt is when the challenge expires.
	ExpiresAt time.Time

	// SignatureAlgorithm is the algorithm to use for signing.
	SignatureAlgorithm string
}

// IsExpired returns true if the challenge has expired.
func (c *Challenge) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// NewChallengeStore creates a new challenge store with the given TTL.
// The TTL determines how long challenges remain valid.
func NewChallengeStore(ttl time.Duration) *ChallengeStore {
	if ttl <= 0 {
		ttl = 30 * time.Second // Default 30 second TTL
	}

	cs := &ChallengeStore{
		challenges: make(map[string]*Challenge),
		ttl:        ttl,
		stopCh:     make(chan struct{}),
	}

	// Start background cleanup goroutine
	go cs.cleanupLoop()

	return cs
}

// Create creates a new challenge for the given public key.
func (cs *ChallengeStore) Create(publicKey []byte, keyType string) (*Challenge, error) {
	// Generate challenge ID
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("failed to generate challenge ID: %w", err)
	}
	id := hex.EncodeToString(idBytes)

	// Generate nonce (32 bytes of random data to sign)
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate challenge nonce: %w", err)
	}

	// Determine signature algorithm based on key type
	sigAlgo := "ssh-ed25519"
	if keyType == "rsa" {
		sigAlgo = "rsa-sha2-256"
	}

	now := time.Now()
	challenge := &Challenge{
		ID:                 id,
		PublicKey:          publicKey,
		KeyType:            keyType,
		Nonce:              nonce,
		CreatedAt:          now,
		ExpiresAt:          now.Add(cs.ttl),
		SignatureAlgorithm: sigAlgo,
	}

	cs.mu.Lock()
	cs.challenges[id] = challenge
	cs.mu.Unlock()

	return challenge, nil
}

// Get retrieves a challenge by ID and removes it from the store.
// This implements one-time-use semantics - a challenge can only be verified once.
func (cs *ChallengeStore) Get(id string) (*Challenge, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	challenge, ok := cs.challenges[id]
	if !ok {
		return nil, false
	}

	// Remove the challenge (one-time use)
	delete(cs.challenges, id)

	// Check if expired
	if challenge.IsExpired() {
		return nil, false
	}

	return challenge, true
}

// Peek retrieves a challenge by ID without removing it.
// Used for checking if a challenge exists.
func (cs *ChallengeStore) Peek(id string) (*Challenge, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	challenge, ok := cs.challenges[id]
	if !ok || challenge.IsExpired() {
		return nil, false
	}

	return challenge, true
}

// Delete removes a challenge by ID.
func (cs *ChallengeStore) Delete(id string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.challenges, id)
}

// Count returns the number of active challenges.
func (cs *ChallengeStore) Count() int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return len(cs.challenges)
}

// Stop stops the background cleanup goroutine.
func (cs *ChallengeStore) Stop() {
	close(cs.stopCh)
}

// cleanupLoop periodically removes expired challenges.
func (cs *ChallengeStore) cleanupLoop() {
	ticker := time.NewTicker(cs.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-cs.stopCh:
			return
		case <-ticker.C:
			cs.cleanup()
		}
	}
}

// cleanup removes all expired challenges.
func (cs *ChallengeStore) cleanup() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	now := time.Now()
	for id, challenge := range cs.challenges {
		if now.After(challenge.ExpiresAt) {
			delete(cs.challenges, id)
		}
	}
}
