package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

type RotationLink struct {
	OldID     string    `json:"oldId"`
	NewID     string    `json:"newId"`
	OldPub    string    `json:"oldPub"`
	NewPub    string    `json:"newPub"`
	Timestamp time.Time `json:"ts"`
	SigOld    string    `json:"sigOld"`
	SigNew    string    `json:"sigNew"`
}

// RotateKey generates a new key pair and constructs a signed link.
func RotateKey(oldKM *KeyMaterial) (*KeyMaterial, *RotationLink, error) {
	newPub, newPrivy, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	newID := DeriveID(newPub)

	link := &RotationLink{
		OldID:     oldKM.ID,
		NewID:     newID,
		OldPub:    base64.StdEncoding.EncodeToString(oldKM.PublicKey),
		NewPub:    base64.StdEncoding.EncodeToString(newPub),
		Timestamp: time.Now().UTC(),
	}

	tmp := struct {
		OldID     string    `json:"oldId"`
		NewID     string    `json:"newId"`
		OldPub    string    `json:"oldPub"`
		NewPub    string    `json:"newPub"`
		Timestamp time.Time `json:"ts"`
	}{
		link.OldID, link.NewID, link.OldPub, link.NewPub, link.Timestamp,
	}

	canon, err := json.Marshal(tmp)
	if err != nil {
		return nil, nil, err
	}
	digest := sha256.Sum256(canon)

	sigOld := ed25519.Sign(oldKM.PrivateKey, digest[:])
	sigNew := ed25519.Sign(newPrivy, digest[:])
	link.SigOld = base64.StdEncoding.EncodeToString(sigOld)
	link.SigNew = base64.StdEncoding.EncodeToString(sigNew)

	newKM := &KeyMaterial{
		ID:         newID,
		PublicKey:  newPub,
		PrivateKey: newPrivy,
	}
	return newKM, link, nil
}

// VerifyRotationLink checks signatures & ID consistency.
func VerifyRotationLink(link *RotationLink) error {
	oldPubBytes, err := base64.StdEncoding.DecodeString(link.OldPub)
	if err != nil {
		return err
	}
	newPubBytes, err := base64.StdEncoding.DecodeString(link.NewPub)
	if err != nil {
		return err
	}
	if DeriveID(oldPubBytes) != link.OldID {
		return errors.New("oldID mismatch")
	}
	if DeriveID(newPubBytes) != link.NewID {
		return errors.New("newID mismatch")
	}

	tmp := struct {
		OldID     string    `json:"oldId"`
		NewID     string    `json:"newId"`
		OldPub    string    `json:"oldPub"`
		NewPub    string    `json:"newPub"`
		Timestamp time.Time `json:"ts"`
	}{
		link.OldID, link.NewID, link.OldPub, link.NewPub, link.Timestamp,
	}
	canon, err := json.Marshal(tmp)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(canon)

	sigOld, err := base64.StdEncoding.DecodeString(link.SigOld)
	if err != nil {
		return err
	}
	sigNew, err := base64.StdEncoding.DecodeString(link.SigNew)
	if err != nil {
		return err
	}

	if !ed25519.Verify(oldPubBytes, digest[:], sigOld) {
		return errors.New("old signature invalid")
	}
	if !ed25519.Verify(newPubBytes, digest[:], sigNew) {
		return errors.New("new signature invalid")
	}
	return nil
}
