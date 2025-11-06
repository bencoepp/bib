package identity

import (
	"crypto/ed25519"
	"errors"
	"sync"
)

// PublicKeyRegistry maps ID -> public key; used when envelopes omit PubKey.
type PublicKeyRegistry struct {
	mu   sync.RWMutex
	data map[string]ed25519.PublicKey
}

func NewPublicKeyRegistry() *PublicKeyRegistry {
	return &PublicKeyRegistry{
		data: make(map[string]ed25519.PublicKey),
	}
}

func (r *PublicKeyRegistry) Put(id string, pub ed25519.PublicKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.data[id]; ok {
		if string(existing) != string(pub) {
			return errors.New("conflict: id already mapped to different public key")
		}
		return nil
	}
	r.data[id] = pub
	return nil
}

func (r *PublicKeyRegistry) Get(id string) (ed25519.PublicKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pub, ok := r.data[id]
	if !ok {
		return nil, errors.New("public key not found")
	}
	return pub, nil
}
