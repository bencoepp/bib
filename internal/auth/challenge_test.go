package auth

import (
	"testing"
	"time"
)

func TestChallengeStore_Create(t *testing.T) {
	store := NewChallengeStore(30 * time.Second)
	defer store.Stop()

	publicKey := make([]byte, 32) // Fake Ed25519 key
	for i := range publicKey {
		publicKey[i] = byte(i)
	}

	challenge, err := store.Create(publicKey, "ed25519")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if challenge.ID == "" {
		t.Error("Challenge ID should not be empty")
	}
	if len(challenge.Nonce) != 32 {
		t.Errorf("Nonce should be 32 bytes, got %d", len(challenge.Nonce))
	}
	if challenge.KeyType != "ed25519" {
		t.Errorf("KeyType should be ed25519, got %s", challenge.KeyType)
	}
	if challenge.SignatureAlgorithm != "ssh-ed25519" {
		t.Errorf("SignatureAlgorithm should be ssh-ed25519, got %s", challenge.SignatureAlgorithm)
	}
	if challenge.IsExpired() {
		t.Error("New challenge should not be expired")
	}
}

func TestChallengeStore_Get(t *testing.T) {
	store := NewChallengeStore(30 * time.Second)
	defer store.Stop()

	publicKey := make([]byte, 32)
	challenge, _ := store.Create(publicKey, "ed25519")

	// Get should succeed
	retrieved, ok := store.Get(challenge.ID)
	if !ok {
		t.Fatal("Get should succeed for valid challenge")
	}
	if retrieved.ID != challenge.ID {
		t.Error("Retrieved challenge should have same ID")
	}

	// Get again should fail (one-time use)
	_, ok = store.Get(challenge.ID)
	if ok {
		t.Error("Get should fail after challenge is consumed")
	}
}

func TestChallengeStore_GetExpired(t *testing.T) {
	store := NewChallengeStore(10 * time.Millisecond) // Very short TTL
	defer store.Stop()

	publicKey := make([]byte, 32)
	challenge, _ := store.Create(publicKey, "ed25519")

	// Wait for expiry
	time.Sleep(20 * time.Millisecond)

	_, ok := store.Get(challenge.ID)
	if ok {
		t.Error("Get should fail for expired challenge")
	}
}

func TestChallengeStore_Peek(t *testing.T) {
	store := NewChallengeStore(30 * time.Second)
	defer store.Stop()

	publicKey := make([]byte, 32)
	challenge, _ := store.Create(publicKey, "ed25519")

	// Peek should succeed multiple times
	for i := 0; i < 3; i++ {
		retrieved, ok := store.Peek(challenge.ID)
		if !ok {
			t.Fatalf("Peek #%d should succeed", i+1)
		}
		if retrieved.ID != challenge.ID {
			t.Error("Peeked challenge should have same ID")
		}
	}

	// Challenge should still be consumable
	_, ok := store.Get(challenge.ID)
	if !ok {
		t.Error("Get should succeed after Peek")
	}
}

func TestChallengeStore_Delete(t *testing.T) {
	store := NewChallengeStore(30 * time.Second)
	defer store.Stop()

	publicKey := make([]byte, 32)
	challenge, _ := store.Create(publicKey, "ed25519")

	store.Delete(challenge.ID)

	_, ok := store.Get(challenge.ID)
	if ok {
		t.Error("Get should fail after Delete")
	}
}

func TestChallengeStore_Count(t *testing.T) {
	store := NewChallengeStore(30 * time.Second)
	defer store.Stop()

	publicKey := make([]byte, 32)

	if store.Count() != 0 {
		t.Error("Empty store should have count 0")
	}

	store.Create(publicKey, "ed25519")
	store.Create(publicKey, "ed25519")
	store.Create(publicKey, "ed25519")

	if store.Count() != 3 {
		t.Errorf("Count should be 3, got %d", store.Count())
	}
}

func TestChallengeStore_RSAKeyType(t *testing.T) {
	store := NewChallengeStore(30 * time.Second)
	defer store.Stop()

	publicKey := make([]byte, 256) // Fake RSA key
	challenge, err := store.Create(publicKey, "rsa")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if challenge.KeyType != "rsa" {
		t.Errorf("KeyType should be rsa, got %s", challenge.KeyType)
	}
	if challenge.SignatureAlgorithm != "rsa-sha2-256" {
		t.Errorf("SignatureAlgorithm should be rsa-sha2-256, got %s", challenge.SignatureAlgorithm)
	}
}
