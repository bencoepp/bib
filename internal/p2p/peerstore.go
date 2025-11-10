package p2p

import (
	"context"
	"sync"
	"time"
)

type PeerEntry struct {
	PeerID       string
	Multiaddrs   []string
	Source       string
	FirstSeen    time.Time
	LastSeen     time.Time
	LastRTTMs    float64
	RTTSamples   []float64
	Capabilities map[string]bool
	Region       string
	Tags         []string
	Load         float64
	ScoreCache   float64
}

type PeerStore struct {
	mu    sync.RWMutex
	peers map[string]*PeerEntry
}

func NewPeerStore() *PeerStore {
	return &PeerStore{
		peers: make(map[string]*PeerEntry),
	}
}

func (s *PeerStore) UpsertFromCandidate(c Candidate) {
	now := c.Discovered
	if now.IsZero() {
		now = time.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.peers[c.PeerID]
	if !ok {
		s.peers[c.PeerID] = &PeerEntry{
			PeerID:     c.PeerID,
			Multiaddrs: DedupeStrings(c.Multiaddrs, nil),
			Source:     c.Source,
			FirstSeen:  now,
			LastSeen:   now,
		}
		return
	}

	// Merge addresses (dedupe).
	e.Multiaddrs = DedupeStrings(c.Multiaddrs, e.Multiaddrs)
	// Update provenance to latest source (optional policy).
	e.Source = c.Source
	e.LastSeen = now
}

func (s *PeerStore) Sink() func(context.Context, Candidate) {
	return func(_ context.Context, c Candidate) {
		s.UpsertFromCandidate(c)
	}
}

// Get returns a copy of the entry for a peerID, or nil if missing.
func (s *PeerStore) Get(peerID string) *PeerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.peers[peerID]
	if !ok {
		return nil
	}
	cp := *e
	// Copy slice to avoid external mutation.
	cp.Multiaddrs = append([]string(nil), e.Multiaddrs...)
	return &cp
}

// Snapshot returns copies of all entries.
func (s *PeerStore) Snapshot() []PeerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PeerEntry, 0, len(s.peers))
	for _, e := range s.peers {
		cp := *e
		cp.Multiaddrs = append([]string(nil), e.Multiaddrs...)
		out = append(out, cp)
	}
	return out
}

// Count returns the number of peers in the store.
func (s *PeerStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.peers)
}

func (s *PeerStore) SetCapabilities(peerID string, caps []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.peers[peerID]
	if !ok {
		return
	}
	if e.Capabilities == nil {
		e.Capabilities = make(map[string]bool, len(caps))
	}
	for _, c := range caps {
		e.Capabilities[c] = true
	}
}

func (s *PeerStore) SetRegion(peerID, region string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.peers[peerID]; ok {
		e.Region = region
	}
}

func (s *PeerStore) AddTags(peerID string, tags []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.peers[peerID]
	if !ok {
		return
	}
	seen := make(map[string]struct{}, len(e.Tags))
	for _, t := range e.Tags {
		seen[t] = struct{}{}
	}
	for _, t := range tags {
		if _, exists := seen[t]; !exists {
			e.Tags = append(e.Tags, t)
			seen[t] = struct{}{}
		}
	}
}

func (s *PeerStore) SetLoad(peerID string, load float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.peers[peerID]; ok {
		e.Load = load
	}
}

func (s *PeerStore) AddRTTSample(peerID string, d time.Duration) {
	ms := float64(d.Milliseconds())
	if ms <= 0 {
		return
	}
	const maxSamples = 20

	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.peers[peerID]
	if !ok {
		return
	}
	e.LastRTTMs = ms
	e.RTTSamples = append(e.RTTSamples, ms)
	if len(e.RTTSamples) > maxSamples {
		e.RTTSamples = e.RTTSamples[len(e.RTTSamples)-maxSamples:]
	}
}

func (s *PeerStore) AvgRTT(peerID string) (avg float64, samples int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.peers[peerID]
	if !ok || len(e.RTTSamples) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range e.RTTSamples {
		sum += v
	}
	return sum / float64(len(e.RTTSamples)), len(e.RTTSamples)
}

func (s *PeerStore) UpdateScore(peerID string, score float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.peers[peerID]; ok {
		e.ScoreCache = score
	}
}
