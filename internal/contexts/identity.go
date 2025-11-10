package contexts

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
	ID          string
	Kind        string // "daemon" or "user"
	User        identity.UserIdentity
	Daemon      identity.DaemonIdentity
	PublicKey   ed25519.PublicKey
	KeyMaterial *identity.KeyMaterial
	Registry    *identity.PublicKeyRegistry
	LogManager  *transparency.Manager // only set for daemon or user (both use transparency)
}

var (
	ErrUserIdentityNotFound    = errors.New("no existing user identity found")
	ErrLogSigningKeyNotFound   = errors.New("no existing log signing key found")
	ErrPassphraseRequired      = errors.New("passphrase required by config but empty")
	ErrSecondFactorRequired    = errors.New("second factor required but acquisition failed")
	ErrUnexpectedIdentityKind  = errors.New("loaded identity kind mismatch")
	ErrNilConfigProvided       = errors.New("nil config provided")
	ErrPrivateKeyFileNotFound  = errors.New("private key encryption file missing")
	ErrPublicIdentityFileError = errors.New("public identity file missing or unreadable")
)

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

/* =========================
   Internal Helper Functions
   ========================= */

func getHomeDir(kind string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch kind {
	case "daemon":
		return filepath.Join(homeDir, ".bibd"), nil
	case "user":
		return filepath.Join(homeDir, ".bib"), nil
	default:
		return "", errors.New("unsupported identity kind")
	}
}

// Daemon secrets: may prompt based on config flags.
func acquireDaemonSecrets(cfg *config.BibDaemonConfig) ([]byte, []byte, error) {
	passphrase := []byte("example-passphrase")
	secondFactor := []byte("123456")

	var err error
	if cfg.General.UsePassphrase {
		passphrase, err = secure.ReadPassphrase("Enter identity passphrase: ")
		if err != nil {
			return nil, nil, err
		}
	}
	if cfg.General.UseSecondFactor {
		secondFactor, err = secure.ReadSecondFactor()
		if err != nil {
			return nil, nil, err
		}
	}
	return passphrase, secondFactor, nil
}

// User secrets: do not prompt for passphrase; second factor may prompt if enabled.
func acquireUserSecrets(cfg *config.BibConfig, providedPassphrase string) ([]byte, []byte, error) {
	if cfg.General.UsePassphrase && providedPassphrase == "" {
		return nil, nil, errors.New("passphrase must be provided for user identity")
	}
	passphrase := []byte("example-passphrase")
	if cfg.General.UsePassphrase {
		passphrase = []byte(providedPassphrase)
	}

	secondFactor := []byte("123456")
	if cfg.General.UseSecondFactor {
		sf, err := secure.ReadSecondFactor()
		if err != nil {
			return nil, nil, err
		}
		secondFactor = sf
	}
	return passphrase, secondFactor, nil
}

// Core routine to load existing identity and private key, or create and persist a new one.
func loadOrCreateKeyMaterial(
	pubIdentityPath, privEncPath, kind string,
	passphrase, secondFactor []byte,
	user *identity.UserIdentity,
	daemon *identity.DaemonIdentity,
) (*identity.KeyMaterial, *identity.PublicIdentityFile, error) {
	var km *identity.KeyMaterial

	if _, err := os.Stat(pubIdentityPath); os.IsNotExist(err) {
		// Create new key material
		newKM, err := identity.GenerateKeyMaterial()
		if err != nil {
			return nil, nil, err
		}
		encBlob, err := secure.EncryptPrivateKey(passphrase, secondFactor, newKM.PrivateKey)
		if err != nil {
			return nil, nil, err
		}
		encBytes, _ := secure.MarshalBlob(encBlob)
		if err := os.WriteFile(privEncPath, encBytes, 0600); err != nil {
			return nil, nil, err
		}
		// Persist a new public identity record
		if err := identity.SavePublicIdentity(pubIdentityPath, kind, newKM, user, daemon); err != nil {
			return nil, nil, err
		}
		log.Infof("Created new %s identity: %s", kind, newKM.ID)
		km = newKM
		return km, nil, nil
	}

	// Load existing identity
	loadedKM, loadedKind, meta, err := identity.LoadPublicIdentity(pubIdentityPath)
	if err != nil {
		return nil, nil, err
	}
	if loadedKind != kind {
		log.Warnf("loaded identity kind (%s) differs from expected (%s)", loadedKind, kind)
	}

	encData, err := os.ReadFile(privEncPath)
	if err != nil {
		return nil, nil, err
	}
	blob, err := secure.UnmarshalBlob(encData)
	if err != nil {
		return nil, nil, err
	}
	pt, err := secure.DecryptPrivateKey(passphrase, secondFactor, blob)
	if err != nil {
		return nil, nil, err
	}
	loadedKM.PrivateKey = pt
	km = loadedKM

	if user != nil {
		*user = meta.User
	}
	if daemon != nil {
		*daemon = meta.Daemon
	}

	log.Infof("Loaded %s identity: %s", kind, km.ID)
	return km, meta, nil
}

// If missing, retrieve location and persist it using UpdatePublicIdentityLocation.
func ensureAndPersistLocation(pubIdentityPath, kind string, user *identity.UserIdentity, daemon *identity.DaemonIdentity) {
	switch kind {
	case "user":
		if user == nil {
			return
		}
		if user.Location.Country != "" {
			return
		}
		if l, err := identity.RetrieveLocationInfo(nil); err == nil && l != nil {
			user.Location = *l
			if err := identity.UpdatePublicIdentityLocation(pubIdentityPath, &user.Location); err != nil {
				log.Warnf("failed to update user location in identity file: %v", err)
			} else {
				log.Info("Persisted newly retrieved user location.")
			}
		} else if err != nil {
			log.Warnf("user location retrieval failed: %v", err)
		}
	case "daemon":
		if daemon == nil {
			return
		}
		if daemon.Location.Country != "" {
			return
		}
		if l, err := identity.RetrieveLocationInfo(nil); err == nil && l != nil {
			daemon.Location = *l
			if err := identity.UpdatePublicIdentityLocation(pubIdentityPath, &daemon.Location); err != nil {
				log.Warnf("failed to update daemon location in identity file: %v", err)
			} else {
				log.Info("Persisted newly retrieved daemon location.")
			}
		} else if err != nil {
			log.Warnf("daemon location retrieval failed: %v", err)
		}
	}
}

func loadOrCreateLogSigningKey(path string, passphrase, secondFactor []byte) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	var priv ed25519.PrivateKey
	var pub ed25519.PublicKey

	if _, err := os.Stat(path); os.IsNotExist(err) {
		pubGen, privGen, err := ed25519.GenerateKey(nil)
		if err != nil {
			return nil, nil, err
		}
		priv = privGen
		pub = pubGen
		enc, err := secure.EncryptPrivateKey(passphrase, secondFactor, priv)
		if err != nil {
			return nil, nil, err
		}
		blob, _ := secure.MarshalBlob(enc)
		if err := os.WriteFile(path, blob, 0600); err != nil {
			return nil, nil, err
		}
		log.Info("Generated log signing key.")
	} else {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		blob, err := secure.UnmarshalBlob(raw)
		if err != nil {
			return nil, nil, err
		}
		pt, err := secure.DecryptPrivateKey(passphrase, secondFactor, blob)
		if err != nil {
			return nil, nil, err
		}
		priv = pt
		pub = priv.Public().(ed25519.PublicKey)
	}
	if pub == nil && priv != nil {
		pub = priv.Public().(ed25519.PublicKey)
	}
	return priv, pub, nil
}

/*
=========================

	New helper: load existing log signing key only
	=========================
*/
func loadExistingLogSigningKey(path string, passphrase, secondFactor []byte) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil, ErrLogSigningKeyNotFound
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	blob, err := secure.UnmarshalBlob(raw)
	if err != nil {
		return nil, nil, err
	}
	pt, err := secure.DecryptPrivateKey(passphrase, secondFactor, blob)
	if err != nil {
		return nil, nil, err
	}
	priv := ed25519.PrivateKey(pt)
	pub := priv.Public().(ed25519.PublicKey)
	return priv, pub, nil
}

func RegisterDaemonIdentity(cfg *config.BibDaemonConfig, version string) (*IdentityContext, error) {
	home, err := getHomeDir("daemon")
	if cfg.General.IdentityPath != "" {
		home = cfg.General.IdentityPath
	}
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return nil, err
	}

	pubIdentityPath := filepath.Join(home, "identity_public.json")
	privyEncPath := filepath.Join(home, "private_key.enc")
	logPath := filepath.Join(home, "transparency.log")
	rootsPath := filepath.Join(home, "roots.log")
	logKeyEncPath := filepath.Join(home, "log_signing_key.enc")

	passphrase, secondFactor, err := acquireDaemonSecrets(cfg)
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	daemon := &identity.DaemonIdentity{
		Version:  version,
		Hostname: hostname,
	}

	if l, err := identity.RetrieveLocationInfo(nil); err == nil && l != nil {
		daemon.Location = *l
	} else if err != nil {
		log.Warnf("location retrieval failed (will persist without location): %v", err)
	}

	km, meta, err := loadOrCreateKeyMaterial(pubIdentityPath, privyEncPath, "daemon", passphrase, secondFactor, &identity.UserIdentity{}, daemon)
	if err != nil {
		return nil, err
	}

	if meta != nil && meta.Daemon.Hostname != "" {
		*daemon = meta.Daemon
	}

	ensureAndPersistLocation(pubIdentityPath, "daemon", nil, daemon)

	logPriv, logPub, err := loadOrCreateLogSigningKey(logKeyEncPath, passphrase, secondFactor)
	if err != nil {
		return nil, err
	}
	mgr := transparency.NewManager(logPath, rootsPath, logPriv, logPub)
	if err := mgr.Load(); err != nil {
		return nil, err
	}

	ctx := &IdentityContext{
		ID:          km.ID,
		Kind:        "daemon",
		Daemon:      *daemon,
		PublicKey:   km.PublicKey,
		KeyMaterial: km,
		Registry:    identity.NewPublicKeyRegistry(),
		LogManager:  mgr,
	}
	_ = ctx.Registry.Put(ctx.ID, ctx.PublicKey)

	return ctx, nil
}

func RegisterUserIdentity(cfg *config.BibConfig, version, firstName, lastName, email, passphraseStr string) (*IdentityContext, error) {
	home, err := getHomeDir("user")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return nil, err
	}

	pubIdentityPath := filepath.Join(home, "identity_public.json")
	privyEncPath := filepath.Join(home, "private_key.enc")
	logPath := filepath.Join(home, "transparency.log")
	rootsPath := filepath.Join(home, "roots.log")
	logKeyEncPath := filepath.Join(home, "log_signing_key.enc")

	passphrase, secondFactor, err := acquireUserSecrets(cfg, passphraseStr)
	if err != nil {
		return nil, err
	}

	user := &identity.UserIdentity{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
		Version:   version,
	}

	if l, err := identity.RetrieveLocationInfo(nil); err == nil && l != nil {
		user.Location = *l
	} else if err != nil {
		log.Warnf("user location retrieval failed (will persist without location): %v", err)
	}

	km, meta, err := loadOrCreateKeyMaterial(pubIdentityPath, privyEncPath, "user", passphrase, secondFactor, user, &identity.DaemonIdentity{})
	if err != nil {
		return nil, err
	}

	if meta != nil {
		user.FirstName = meta.User.FirstName
		user.LastName = meta.User.LastName
		user.Email = meta.User.Email
		user.Location = meta.User.Location
		if meta.User.Version != "" {
			user.Version = meta.User.Version
		}
	}

	ensureAndPersistLocation(pubIdentityPath, "user", user, nil)

	logPriv, logPub, err := loadOrCreateLogSigningKey(logKeyEncPath, passphrase, secondFactor)
	if err != nil {
		return nil, err
	}
	mgr := transparency.NewManager(logPath, rootsPath, logPriv, logPub)
	if err := mgr.Load(); err != nil {
		return nil, err
	}

	ctx := &IdentityContext{
		ID:          km.ID,
		Kind:        "user",
		User:        *user,
		PublicKey:   km.PublicKey,
		KeyMaterial: km,
		Registry:    identity.NewPublicKeyRegistry(),
		LogManager:  mgr,
	}
	_ = ctx.Registry.Put(ctx.ID, ctx.PublicKey)

	return ctx, nil
}

/* =========================
   New: Load existing user identity ONLY (no creation)
   ========================= */

// LoadExistingUserIdentity attempts to load an already existing user identity.
// It will NOT create anything; if the identity or its private key/log signing key
// do not exist, it returns ErrUserIdentityNotFound (or a specific error).
// Only the passphrase is provided by the caller; if second factor is enabled
// per config it will prompt interactively (same as other routines).
func LoadExistingUserIdentity(cfg *config.BibConfig, passphraseStr string) (*IdentityContext, error) {
	if cfg == nil {
		return nil, ErrNilConfigProvided
	}
	home, err := getHomeDir("user")
	if err != nil {
		return nil, err
	}

	pubIdentityPath := filepath.Join(home, "identity_public.json")
	privEncPath := filepath.Join(home, "private_key.enc")
	logPath := filepath.Join(home, "transparency.log")
	rootsPath := filepath.Join(home, "roots.log")
	logKeyEncPath := filepath.Join(home, "log_signing_key.enc")

	// Quick existence checks
	if _, err := os.Stat(pubIdentityPath); os.IsNotExist(err) {
		return nil, ErrUserIdentityNotFound
	}
	if _, err := os.Stat(privEncPath); os.IsNotExist(err) {
		return nil, ErrPrivateKeyFileNotFound
	}
	if cfg.General.UsePassphrase && passphraseStr == "" {
		return nil, ErrPassphraseRequired
	}

	// Gather secrets (passphrase + optional second factor)
	passphrase := []byte(passphraseStr)
	secondFactor := []byte("123456")
	if cfg.General.UseSecondFactor {
		sf, err := secure.ReadSecondFactor()
		if err != nil {
			return nil, ErrSecondFactorRequired
		}
		secondFactor = sf
	}

	// Load public identity meta
	km, kind, meta, err := identity.LoadPublicIdentity(pubIdentityPath)
	if err != nil {
		return nil, err
	}
	if kind != "user" {
		return nil, ErrUnexpectedIdentityKind
	}

	// Decrypt private key
	encData, err := os.ReadFile(privEncPath)
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
	km.PrivateKey = pt

	// Load existing log signing key (must exist)
	logPriv, logPub, err := loadExistingLogSigningKey(logKeyEncPath, passphrase, secondFactor)
	if err != nil {
		return nil, err
	}
	mgr := transparency.NewManager(logPath, rootsPath, logPriv, logPub)
	if err := mgr.Load(); err != nil {
		return nil, err
	}

	// Build identity context
	ctx := &IdentityContext{
		ID:          km.ID,
		Kind:        "user",
		User:        meta.User, // as saved
		PublicKey:   km.PublicKey,
		KeyMaterial: km,
		Registry:    identity.NewPublicKeyRegistry(),
		LogManager:  mgr,
	}
	_ = ctx.Registry.Put(ctx.ID, ctx.PublicKey)

	return ctx, nil
}
