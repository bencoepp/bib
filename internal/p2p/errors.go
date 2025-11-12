package p2p

import (
	"fmt"
)

var (
	ErrIdentityNoPrivateKey = fmt.Errorf("p2p: the identity provided did not have a public key")
	ErrInvalidListenAddress = fmt.Errorf("p2p: invalid listen address")
)
