package identity

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type PublicIdentityFile struct {
	ID        string    `json:"id"`
	PublicKey string    `json:"publicKey"` // hex
	Kind      string    `json:"kind"`
	Hostname  string    `json:"hostname,omitempty"`
	Version   string    `json:"version,omitempty"`
	Location  *Location `json:"location,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	// Consider future fields like UpdatedAt if you refresh metadata later.
}

func SavePublicIdentity(path string, kind string, km *KeyMaterial, hostname, version string, loc *Location) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data := PublicIdentityFile{
		ID:        km.ID,
		PublicKey: hex.EncodeToString(km.PublicKey),
		Kind:      kind,
		Hostname:  hostname,
		Version:   version,
		Location:  loc,
		CreatedAt: time.Now().UTC(),
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func LoadPublicIdentity(path string) (*KeyMaterial, string, *PublicIdentityFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", nil, err
	}
	var data PublicIdentityFile
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, "", nil, err
	}
	pubBytes, err := hex.DecodeString(data.PublicKey)
	if err != nil {
		return nil, "", nil, err
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return nil, "", nil, errors.New("invalid public key length")
	}
	pub := ed25519.PublicKey(pubBytes)
	if DeriveID(pub) != data.ID {
		return nil, "", nil, errors.New("id mismatch with public key")
	}
	// Private key not loaded here.
	km := &KeyMaterial{
		ID:        data.ID,
		PublicKey: pub,
	}
	return km, data.Kind, &data, nil
}

func UpdatePublicIdentityLocation(path string, loc *Location) error {
	km, kind, meta, err := LoadPublicIdentity(path)
	if err != nil {
		return err
	}
	return SavePublicIdentity(path, kind, km, meta.Hostname, meta.Version, loc)
}
