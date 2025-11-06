package identity

import (
	"time"
)

type UserIdentity struct {
	ID        string
	PublicKey []byte
	FirstName string
	LastName  string
	Email     string
	Location  Location
	Created   time.Time
}

func NewUserIdentity(km *KeyMaterial, first, last, email string, loc Location) *UserIdentity {
	return &UserIdentity{
		ID:        km.ID,
		PublicKey: km.PublicKey,
		FirstName: first,
		LastName:  last,
		Email:     email,
		Location:  loc,
		Created:   time.Now().UTC(),
	}
}
