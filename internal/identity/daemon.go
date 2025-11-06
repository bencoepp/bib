package identity

import (
	"time"
)

type DaemonIdentity struct {
	ID        string
	PublicKey []byte
	Version   string
	Hostname  string
	Location  Location
	Created   time.Time
}
