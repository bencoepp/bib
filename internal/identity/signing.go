package identity

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"reflect"
	"time"
)

type SignedEnvelope struct {
	ID          string    `json:"id"`
	PubKey      string    `json:"pubKey,omitempty"` // base64 encoded Ed25519 public key (optional)
	Alg         string    `json:"alg"`              // "Ed25519"
	PayloadType string    `json:"payloadType"`
	Payload     any       `json:"payload"`
	Signature   string    `json:"sig"`
	Timestamp   time.Time `json:"ts"`
}

// SignPayload produces a SignedEnvelope.
// includePub=true is helpful for first contact; subsequent messages may omit PubKey to reduce size.
func SignPayload(km *KeyMaterial, payload any, includePub bool) (*SignedEnvelope, error) {
	canon, err := CanonicalJSON(payload)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(canon)
	sig := ed25519.Sign(km.PrivateKey, digest[:])

	env := &SignedEnvelope{
		ID:          km.ID,
		Alg:         "Ed25519",
		PayloadType: reflect.TypeOf(payload).String(),
		Payload:     payload,
		Signature:   base64.StdEncoding.EncodeToString(sig),
		Timestamp:   time.Now().UTC(),
	}
	if includePub {
		env.PubKey = base64.StdEncoding.EncodeToString(km.PublicKey)
	}
	return env, nil
}

// VerifyEnvelope validates signature & ID/public key consistency.
func VerifyEnvelope(env *SignedEnvelope, pubResolver func(id string) (ed25519.PublicKey, error)) error {
	if env.Alg != "Ed25519" {
		return errors.New("unsupported algorithm")
	}

	var pub ed25519.PublicKey
	var err error
	if env.PubKey != "" {
		pubBytes, err := base64.StdEncoding.DecodeString(env.PubKey)
		if err != nil {
			return err
		}
		pub = pubBytes
		if DeriveID(pub) != env.ID {
			return errors.New("id/pubKey mismatch")
		}
	} else {
		pub, err = pubResolver(env.ID)
		if err != nil {
			return err
		}
	}

	canon, err := CanonicalJSON(env.Payload)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(canon)
	sigBytes, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, digest[:], sigBytes) {
		return errors.New("invalid signature")
	}
	return nil
}
