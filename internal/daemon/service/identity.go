package service

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"bib/internal/contexts"
	"bib/internal/identity"
	"bib/internal/identity/transparency"
	bibv1 "bib/internal/pb/bibd/v1"
)

type StoredProof struct {
	Inclusion  *bibv1.InclusionProof
	SignedRoot *bibv1.SignedLogRoot
}

type IdentityStore struct {
	mu         sync.RWMutex
	identities map[string]*bibv1.IdentityPayload
	rotations  map[string]string // old_id -> new_id
	proofs     map[string]*StoredProof
}

func NewIdentityStore() *IdentityStore {
	return &IdentityStore{
		identities: make(map[string]*bibv1.IdentityPayload),
		rotations:  make(map[string]string),
		proofs:     make(map[string]*StoredProof),
	}
}

func (s *IdentityStore) PutIdentity(p *bibv1.IdentityPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.identities[p.Id] = p
}

func (s *IdentityStore) GetIdentity(id string) (*bibv1.IdentityPayload, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.identities[id]
	return p, ok
}

func (s *IdentityStore) ListIdentities(kindFilter string, start, limit int) ([]*bibv1.IdentityPayload, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := make([]*bibv1.IdentityPayload, 0, len(s.identities))
	for _, v := range s.identities {
		all = append(all, v)
	}
	out := make([]*bibv1.IdentityPayload, 0, limit)
	consumed := start
	for i := start; i < len(all) && len(out) < limit; i++ {
		if kindFilter == "" || all[i].Kind == kindFilter {
			out = append(out, all[i])
		}
		consumed = i + 1
	}
	return out, consumed
}

func (s *IdentityStore) AddRotation(oldID, newID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rotations[oldID] = newID
}

func (s *IdentityStore) PutProof(id string, inc *bibv1.InclusionProof, root *bibv1.SignedLogRoot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proofs[id] = &StoredProof{Inclusion: inc, SignedRoot: root}
}

func (s *IdentityStore) GetProof(id string) *StoredProof {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.proofs[id]
}

type IdentityService struct {
	bibv1.UnimplementedIdentityServiceServer

	IDCtx *contexts.IdentityContext
	Store *IdentityStore
}

// GetSelfIdentity returns the daemon's own identity and latest proof if known.
// Proof will be non-nil once you've published this identity (PublishIdentity)
// and the service has stored the resulting inclusion proof.
func (s *IdentityService) GetSelfIdentity(ctx context.Context, _ *bibv1.GetSelfIdentityRequest) (*bibv1.GetSelfIdentityResponse, error) {
	// Prefer the published/stored version
	published, ok := s.Store.GetIdentity(s.IDCtx.ID)

	var identityPayload *bibv1.IdentityPayload
	if ok {
		identityPayload = published
	} else {
		// Synthesize from IdentityContext (not yet published)
		identityPayload = &bibv1.IdentityPayload{
			Id:        s.IDCtx.ID,
			Kind:      s.IDCtx.Kind,
			PublicKey: s.IDCtx.PublicKey,
			Hostname:  s.IDCtx.Hostname,
			Version:   s.IDCtx.Version,
			CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
		if s.IDCtx.Location != nil {
			identityPayload.Location = &bibv1.Location{
				Country:     s.IDCtx.Location.Country,
				CountryCode: s.IDCtx.Location.CountryCode,
				Region:      s.IDCtx.Location.Region,
				RegionName:  s.IDCtx.Location.RegionName,
				City:        s.IDCtx.Location.City,
				Zip:         s.IDCtx.Location.Zip,
				Latitude:    s.IDCtx.Location.Latitude,
				Longitude:   s.IDCtx.Location.Longitude,
				Timezone:    s.IDCtx.Location.Timezone,
				Isp:         s.IDCtx.Location.Isp,
				Org:         s.IDCtx.Location.Org,
				Asn:         s.IDCtx.Location.As,
				Ip:          s.IDCtx.Location.Ip.String(),
			}
		}
	}

	// Try to fetch the latest proof from the store.
	latestProof, signedRoot, err := s.getLatestProofForID(identityPayload.Id)
	var regProof *bibv1.RegistrationProof
	if err == nil && latestProof != nil && signedRoot != nil {
		regProof = &bibv1.RegistrationProof{
			Identity: identityPayload,
			Inclusion: &bibv1.InclusionProof{
				LeafIndex:  latestProof.LeafIndex,
				LeafHash:   latestProof.LeafHash,
				Siblings:   latestProof.Siblings,
				SignedRoot: signedRoot,
			},
		}
	}

	return &bibv1.GetSelfIdentityResponse{
		Identity:    identityPayload,
		LatestProof: regProof,
	}, nil
}

func (s *IdentityService) PublishIdentity(ctx context.Context, req *bibv1.PublishIdentityRequest) (*bibv1.PublishIdentityResponse, error) {
	if req == nil || req.Envelope == nil {
		return nil, errors.New("missing envelope")
	}
	env := req.Envelope
	if env.PayloadType != "IdentityPayload" {
		return nil, fmt.Errorf("unexpected payload_type %q", env.PayloadType)
	}
	identityPayload, err := decodeIdentityPayload(env.PayloadBytes)
	if err != nil {
		return nil, fmt.Errorf("decode identity payload: %w", err)
	}
	if identityPayload.Id == "" {
		return nil, errors.New("identity payload missing id")
	}
	if len(identityPayload.PublicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("identity public key must be %d bytes", ed25519.PublicKeySize)
	}
	derivedID := identity.DeriveID(identityPayload.PublicKey)
	if derivedID != identityPayload.Id {
		return nil, fmt.Errorf("derived id mismatch: %s != %s", derivedID, identityPayload.Id)
	}
	if env.Id != identityPayload.Id {
		return nil, fmt.Errorf("envelope id != payload id")
	}
	pubKey, err := s.resolveEnvelopePublicKey(env)
	if err != nil {
		return nil, fmt.Errorf("resolve public key: %w", err)
	}
	if !ed25519.Verify(pubKey, env.PayloadBytes, env.Signature) {
		return nil, errors.New("signature verification failed")
	}
	// Register key and store identity.
	if err := s.IDCtx.Registry.Put(identityPayload.Id, identityPayload.PublicKey); err != nil {
		return nil, fmt.Errorf("registry put: %w", err)
	}
	s.Store.PutIdentity(identityPayload)

	// Append to transparency log (replace stub with real manager calls).
	proof, signedRoot, err := s.appendIdentityToLog(identityPayload, env)
	if err != nil {
		return nil, fmt.Errorf("append to log: %w", err)
	}

	// Cache the proof for later retrieval by GetSelfIdentity/RetrieveIdentity.
	inc := &bibv1.InclusionProof{
		LeafIndex:  proof.LeafIndex,
		LeafHash:   proof.LeafHash,
		Siblings:   proof.Siblings,
		SignedRoot: signedRoot,
	}
	s.Store.PutProof(identityPayload.Id, inc, signedRoot)

	return &bibv1.PublishIdentityResponse{
		Proof: &bibv1.RegistrationProof{
			Identity:  identityPayload,
			Inclusion: inc,
		},
	}, nil
}

func (s *IdentityService) RetrieveIdentity(ctx context.Context, req *bibv1.RetrieveIdentityRequest) (*bibv1.RetrieveIdentityResponse, error) {
	if req == nil || req.Id == "" {
		return nil, errors.New("missing id")
	}
	payload, ok := s.Store.GetIdentity(req.Id)
	if !ok {
		return nil, fmt.Errorf("identity %s not found", req.Id)
	}

	// Try to load the latest proof from the store (or manager fallback).
	proof, signedRoot, err := s.getLatestProofForID(req.Id)
	if err != nil || proof == nil || signedRoot == nil {
		return &bibv1.RetrieveIdentityResponse{Identity: payload}, nil
	}
	return &bibv1.RetrieveIdentityResponse{
		Identity: payload,
		LatestProof: &bibv1.RegistrationProof{
			Identity: payload,
			Inclusion: &bibv1.InclusionProof{
				LeafIndex:  proof.LeafIndex,
				LeafHash:   proof.LeafHash,
				Siblings:   proof.Siblings,
				SignedRoot: signedRoot,
			},
		},
	}, nil
}

func (s *IdentityService) RetrieveIdentities(ctx context.Context, req *bibv1.RetrieveIdentitiesRequest) (*bibv1.RetrieveIdentitiesResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 25
	} else if limit > 500 {
		limit = 500
	}
	start := 0
	if req.PageToken != "" {
		off, err := strconv.Atoi(req.PageToken)
		if err != nil {
			return nil, fmt.Errorf("invalid page_token: %w", err)
		}
		start = off
	}
	items, consumed := s.Store.ListIdentities(req.Kind, start, limit)

	out := make([]*bibv1.IdentitySummary, 0, len(items))
	for _, p := range items {
		out = append(out, &bibv1.IdentitySummary{
			Id:       p.Id,
			Kind:     p.Kind,
			Hostname: p.Hostname,
			Version:  p.Version,
			Email:    p.Email,
		})
	}
	var next string
	if consumed >= start+limit {
		next = strconv.Itoa(consumed)
	}
	return &bibv1.RetrieveIdentitiesResponse{
		Identities:    out,
		NextPageToken: next,
	}, nil
}

func (s *IdentityService) SubmitRotationLink(ctx context.Context, req *bibv1.SubmitRotationLinkRequest) (*bibv1.SubmitRotationLinkResponse, error) {
	if req == nil || req.Envelope == nil {
		return nil, errors.New("missing envelope")
	}
	env := req.Envelope
	if env.PayloadType != "RotationLinkPayload" {
		return nil, fmt.Errorf("expected payload_type RotationLinkPayload, got %s", env.PayloadType)
	}
	rp, err := decodeRotationPayload(env.PayloadBytes)
	if err != nil {
		return nil, fmt.Errorf("decode rotation payload: %w", err)
	}
	if rp.OldId == "" || rp.NewId == "" {
		return nil, errors.New("rotation payload missing ids")
	}
	if len(rp.OldPublicKey) != ed25519.PublicKeySize || len(rp.NewPublicKey) != ed25519.PublicKeySize {
		return nil, errors.New("invalid public key sizes")
	}
	if identity.DeriveID(rp.OldPublicKey) != rp.OldId {
		return nil, errors.New("old_id mismatch derived")
	}
	if identity.DeriveID(rp.NewPublicKey) != rp.NewId {
		return nil, errors.New("new_id mismatch derived")
	}
	envSigner, err := s.resolveEnvelopePublicKey(env)
	if err != nil {
		return nil, fmt.Errorf("resolve envelope signer: %w", err)
	}
	if !ed25519.Verify(envSigner, env.PayloadBytes, env.Signature) {
		return nil, errors.New("envelope signature invalid")
	}
	canon := env.PayloadBytes
	if !ed25519.Verify(rp.OldPublicKey, canon, rp.SigOld) {
		return nil, errors.New("sig_old invalid")
	}
	if !ed25519.Verify(rp.NewPublicKey, canon, rp.SigNew) {
		return nil, errors.New("sig_new invalid")
	}

	// Update registry and rotation map
	if err := s.IDCtx.Registry.Put(rp.NewId, rp.NewPublicKey); err != nil {
		return nil, fmt.Errorf("registry put new key: %w", err)
	}
	s.Store.AddRotation(rp.OldId, rp.NewId)

	// Append to log and cache proof keyed by newId (or both old and new if you prefer).
	proof, signedRoot, err := s.appendRotationToLog(rp, env)
	if err != nil {
		return nil, fmt.Errorf("append rotation to log: %w", err)
	}
	inc := &bibv1.InclusionProof{
		LeafIndex:  proof.LeafIndex,
		LeafHash:   proof.LeafHash,
		Siblings:   proof.Siblings,
		SignedRoot: signedRoot,
	}
	s.Store.PutProof(rp.NewId, inc, signedRoot)

	return &bibv1.SubmitRotationLinkResponse{
		Proof: &bibv1.RegistrationProof{
			Identity: &bibv1.IdentityPayload{
				Id:        rp.NewId,
				Kind:      "daemon",
				PublicKey: rp.NewPublicKey,
			},
			Inclusion: inc,
		},
	}, nil
}

func (s *IdentityService) GetLogRoot(ctx context.Context, _ *bibv1.GetLogRootRequest) (*bibv1.GetLogRootResponse, error) {
	root, err := getSignedRootViaManager(s.IDCtx.LogManager)
	if err != nil {
		return nil, fmt.Errorf("fetch signed root: %w", err)
	}
	return &bibv1.GetLogRootResponse{SignedRoot: root}, nil
}

// ----------------- Helpers / Integration Points --------------------

func decodeIdentityPayload(b []byte) (*bibv1.IdentityPayload, error) {
	var ip bibv1.IdentityPayload
	if err := json.Unmarshal(b, &ip); err != nil {
		return nil, err
	}
	return &ip, nil
}

func decodeRotationPayload(b []byte) (*bibv1.RotationLinkPayload, error) {
	var rp bibv1.RotationLinkPayload
	if err := json.Unmarshal(b, &rp); err != nil {
		return nil, err
	}
	return &rp, nil
}

// resolveEnvelopePublicKey chooses key in envelope or registry.
func (s *IdentityService) resolveEnvelopePublicKey(env *bibv1.SignedEnvelope) (ed25519.PublicKey, error) {
	if len(env.PublicKey) == ed25519.PublicKeySize {
		// Validate id matches
		if identity.DeriveID(env.PublicKey) != env.Id {
			return nil, errors.New("envelope id/public_key mismatch")
		}
		return env.PublicKey, nil
	}
	// Lookup from registry
	pub, err := s.IDCtx.Registry.Get(env.Id)
	if err != nil {
		return nil, fmt.Errorf("registry get: %w", err)
	}
	return pub, nil
}

// Internal representation to decouple manager output from proto.
type inclusionProofInternal struct {
	LeafIndex uint64
	LeafHash  []byte
	Siblings  [][]byte
}

// appendIdentityToLog integrates with transparency.Manager.
// Replace the stub with your actual manager calls.
func (s *IdentityService) appendIdentityToLog(identityPayload *bibv1.IdentityPayload, env *bibv1.SignedEnvelope) (*inclusionProofInternal, *bibv1.SignedLogRoot, error) {
	leaf := env.PayloadBytes
	hash, idx, siblings, signedRoot, err := appendLeafViaManager(s.IDCtx.LogManager, leaf)
	if err != nil {
		return nil, nil, err
	}
	return &inclusionProofInternal{
		LeafIndex: idx,
		LeafHash:  hash,
		Siblings:  siblings,
	}, signedRoot, nil
}

func (s *IdentityService) appendRotationToLog(rp *bibv1.RotationLinkPayload, env *bibv1.SignedEnvelope) (*inclusionProofInternal, *bibv1.SignedLogRoot, error) {
	leaf := env.PayloadBytes
	hash, idx, siblings, signedRoot, err := appendLeafViaManager(s.IDCtx.LogManager, leaf)
	if err != nil {
		return nil, nil, err
	}
	return &inclusionProofInternal{
		LeafIndex: idx,
		LeafHash:  hash,
		Siblings:  siblings,
	}, signedRoot, nil
}

// getLatestProofForID now checks the in-memory store first.
// Optional: add a fallback to the transparency manager if you can look up by leaf hash or index.
func (s *IdentityService) getLatestProofForID(id string) (*inclusionProofInternal, *bibv1.SignedLogRoot, error) {
	if sp := s.Store.GetProof(id); sp != nil && sp.Inclusion != nil && sp.SignedRoot != nil {
		return &inclusionProofInternal{
			LeafIndex: sp.Inclusion.LeafIndex,
			LeafHash:  sp.Inclusion.LeafHash,
			Siblings:  sp.Inclusion.Siblings,
		}, sp.SignedRoot, nil
	}
	// Fallback: if your manager supports querying by id/leaf hash, do it here.
	// return s.lookupProofViaManager(id)
	return nil, nil, errors.New("no proof found for id")
}

// ---------------- Stub adapters for transparency.Manager ----------------
// Replace these with actual calls to transparency.Manager.
// They demonstrate how you would transform its outputs to proto responses.

func appendLeafViaManager(mgr *transparency.Manager, leaf []byte) (leafHash []byte, index uint64, siblings [][]byte, signedRoot *bibv1.SignedLogRoot, err error) {
	// PSEUDOCODE: Replace with real manager calls.
	return []byte("fake_leaf_hash_32_bytes_______________"), 0, nil, &bibv1.SignedLogRoot{
		TreeSize:     0,
		RootHash:     []byte("fake_root_hash_32_bytes_______________"),
		Timestamp:    "1970-01-01T00:00:00Z",
		Signature:    []byte("fake_signature"),
		LogPublicKey: []byte("fake_log_pub_key"),
		LogKeyId:     base64.StdEncoding.EncodeToString([]byte("fake_log_pub_key")),
	}, nil
}

func getSignedRootViaManager(mgr *transparency.Manager) (*bibv1.SignedLogRoot, error) {
	// PSEUDOCODE: Replace with mgr.CurrentRoot()
	return &bibv1.SignedLogRoot{
		TreeSize:     0,
		RootHash:     []byte("fake_root_hash_32_bytes_______________"),
		Timestamp:    "1970-01-01T00:00:00Z",
		Signature:    []byte("fake_signature"),
		LogPublicKey: []byte("fake_log_pub_key"),
		LogKeyId:     base64.StdEncoding.EncodeToString([]byte("fake_log_pub_key")),
	}, nil
}
