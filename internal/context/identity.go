package context

import (
	"bib/internal/identity"
	"bib/internal/identity/transparency"
	"crypto/ed25519"
	"errors"
)

type IdentityContext struct {
	// Identity
	ID        string
	Kind      string // "daemon"
	Hostname  string
	Version   string
	PublicKey ed25519.PublicKey

	// Persisted / snapshot geolocation (maybe nil)
	Location *identity.Location

	// Key material for signing (includes private key)
	KeyMaterial *identity.KeyMaterial

	// Optional: local registry to resolve peer public keys when verifying envelopes
	Registry *identity.PublicKeyRegistry

	// Optional: transparency log manager (initialized here; you can use it later)
	LogManager *transparency.Manager
}

func (ctx *IdentityContext) SignPayload(payload any, includePub bool) (*identity.SignedEnvelope, error) {
	if ctx == nil || ctx.KeyMaterial == nil || len(ctx.KeyMaterial.PrivateKey) == 0 {
		return nil, errors.New("identity context not initialized for signing")
	}
	return identity.SignPayload(ctx.KeyMaterial, payload, includePub)
}

func (ctx *IdentityContext) VerifyPayloadEnvelope(env *identity.SignedEnvelope) error {
	if ctx == nil {
		return errors.New("nil identity context")
	}
	return identity.VerifyEnvelope(env, func(id string) (ed25519.PublicKey, error) {
		return ctx.Registry.Get(id)
	})
}

func (ctx *IdentityContext) RegisterPeerPublicKey(id string, pub ed25519.PublicKey) error {
	if identity.DeriveID(pub) != id {
		return errors.New("peer id/pubKey mismatch")
	}
	return ctx.Registry.Put(id, pub)
}
