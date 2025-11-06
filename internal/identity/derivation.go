package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

func DeriveID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:])
}

type KeyMaterial struct {
	ID         string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

func GenerateKeyMaterial() (*KeyMaterial, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &KeyMaterial{
		ID:         DeriveID(public),
		PublicKey:  public,
		PrivateKey: private,
	}, nil
}

func ValidateID(id string, pub ed25519.PublicKey) error {
	if DeriveID(pub) != id {
		return errors.New("id does not match public key")
	}
	return nil
}
