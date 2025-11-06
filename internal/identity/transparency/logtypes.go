package transparency

import (
	"encoding/json"
	"time"
)

type Registration struct {
	ID        string    `json:"id"`
	PublicKey string    `json:"publicKey"` // hex
	Type      string    `json:"type"`      // "daemon" | "user" | "rotation"
	Hostname  string    `json:"hostname,omitempty"`
	Version   string    `json:"version,omitempty"`
	Timestamp time.Time `json:"ts"`
}

func Canonical(reg *Registration) ([]byte, error) {
	// Deterministic JSON; rely on stable marshal of map with fixed keys.
	m := map[string]any{
		"id":        reg.ID,
		"publicKey": reg.PublicKey,
		"type":      reg.Type,
		"hostname":  reg.Hostname,
		"version":   reg.Version,
		"ts":        reg.Timestamp.UTC().Format(time.RFC3339Nano),
	}
	return json.Marshal(m)
}

type SignedLogRoot struct {
	TreeSize  uint64    `json:"treeSize"`
	RootHash  string    `json:"rootHash"` // hex
	Timestamp time.Time `json:"ts"`
	Signature string    `json:"sig"`      // base64
	LogKeyID  string    `json:"logKeyId"` // hex(hash(logPub))
}
