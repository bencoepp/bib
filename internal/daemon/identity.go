package daemon

import (
	"bib/internal/config"
	"bib/internal/identity"
	"bib/internal/identity/secure"
	"bib/internal/identity/transparency"
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
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

func RegisterDaemonIdentity(config *config.BibDaemonConfig, version string) (*IdentityContext, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	home := filepath.Join(homeDir, ".bibd")
	if err := os.MkdirAll(home, 0700); err != nil {
		return nil, err
	}

	pubIdentityPath := filepath.Join(home, "identity_public.json")
	privyEncPath := filepath.Join(home, "private_key.enc")
	logPath := filepath.Join(home, "transparency.log")
	rootsPath := filepath.Join(home, "roots.log")
	logKeyEncPath := filepath.Join(home, "log_signing_key.enc")

	passphrase := []byte("example-passphrase")
	secondFactor := []byte("123456")
	if config.General.UsePassphrase {
		passphrase, err = secure.ReadPassphrase("Enter identity passphrase: ")
		if err != nil {
			return nil, err
		}
	}

	if config.General.UseSecondFactor {
		secondFactor, err = secure.ReadSecondFactor()
		if err != nil {
			return nil, err
		}
	}

	var km *identity.KeyMaterial
	kind := "daemon"
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Attempt to retrieve location early if we will create new identity.
	// Use outward-facing IP (nil). Non-fatal.
	var loc *identity.Location
	if l, err := identity.RetrieveLocationInfo(nil); err != nil {
		log.Warnf("location retrieval failed (will persist without location): %v", err)
	} else {
		loc = l
	}

	if _, err := os.Stat(pubIdentityPath); os.IsNotExist(err) {
		// Create new key material
		km, err = identity.GenerateKeyMaterial()
		if err != nil {
			return nil, err
		}
		// Encrypt and store private key
		encBlob, err := secure.EncryptPrivateKey(passphrase, secondFactor, km.PrivateKey)
		if err != nil {
			return nil, err
		}
		encBytes, _ := secure.MarshalBlob(encBlob)
		if err := os.WriteFile(privyEncPath, encBytes, 0600); err != nil {
			return nil, err
		}
		// Persist public identity including (optional) location
		if err := identity.SavePublicIdentity(pubIdentityPath, kind, km, hostname, version, loc); err != nil {
			return nil, err
		}
		log.Infof("Created new identity: %s", km.ID)
	} else {
		// Load existing identity
		loadedKM, loadedKind, pubMeta, err := identity.LoadPublicIdentity(pubIdentityPath)
		if err != nil {
			return nil, err
		}
		kind = loadedKind
		hostname = pubMeta.Hostname
		version = pubMeta.Version
		loc = pubMeta.Location // may be nil if older file without location

		encData, err := os.ReadFile(privyEncPath)
		if err != nil {
			return nil, err
		}
		blob, err := secure.UnmarshalBlob(encData)
		if err != nil {
			return nil, err
		}
		pt, err := secure.DecryptPrivateKey(passphrase, secondFactor, blob)
		if err != nil {
			return nil, err
		}
		loadedKM.PrivateKey = pt
		km = loadedKM
		log.Infof("Loaded identity: %s", km.ID)

		// If location was not previously persisted, attempt one retrieval now and update file.
		if loc == nil {
			if l, err := identity.RetrieveLocationInfo(nil); err == nil {
				loc = l
				if err := identity.UpdatePublicIdentityLocation(pubIdentityPath, loc); err != nil {
					log.Warnf("failed to update identity file with location: %v", err)
				} else {
					log.Info("Persisted newly retrieved location info.")
				}
			} else {
				log.Warnf("location retrieval (update) failed: %v", err)
			}
		}
	}

	// Log signing key (separate from identity key)
	var logPrivy ed25519.PrivateKey
	var logPub ed25519.PublicKey
	if _, err := os.Stat(logKeyEncPath); os.IsNotExist(err) {
		logPub, logPrivy, err = ed25519.GenerateKey(nil)
		if err != nil {
			return nil, err
		}
		enc, err := secure.EncryptPrivateKey(passphrase, secondFactor, logPrivy)
		if err != nil {
			return nil, err
		}
		blob, _ := secure.MarshalBlob(enc)
		if err := os.WriteFile(logKeyEncPath, blob, 0600); err != nil {
			return nil, err
		}
		log.Info("Generated log signing key.")
	} else {
		raw, _ := os.ReadFile(logKeyEncPath)
		blob, err := secure.UnmarshalBlob(raw)
		if err != nil {
			return nil, err
		}
		pt, err := secure.DecryptPrivateKey(passphrase, secondFactor, blob)
		if err != nil {
			return nil, err
		}
		logPrivy = pt
		logPub = logPrivy.Public().(ed25519.PublicKey)
	}
	if logPub == nil && logPrivy != nil {
		logPub = logPrivy.Public().(ed25519.PublicKey)
	}

	mgr := transparency.NewManager(logPath, rootsPath, logPrivy, logPub)
	if err := mgr.Load(); err != nil {
		return nil, err
	}

	ctx := &IdentityContext{
		ID:          km.ID,
		Kind:        kind,
		Hostname:    hostname,
		Version:     version,
		PublicKey:   km.PublicKey,
		Location:    loc,
		KeyMaterial: km,
		Registry:    identity.NewPublicKeyRegistry(),
		LogManager:  mgr,
	}
	// Register self public key for convenience
	_ = ctx.Registry.Put(ctx.ID, ctx.PublicKey)

	return ctx, nil
}
