package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/charmbracelet/log"
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

func (s *PeerStore) UpsertFromCandidate(c Candidate) (created bool, addrDelta int) {
	now := c.Discovered
	if now.IsZero() {
		now = time.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.peers[c.PeerID]; ok {
		// Existing peer: compute how many addresses are new before deduping.
		existing := make(map[string]struct{}, len(e.Multiaddrs))
		for _, a := range e.Multiaddrs {
			existing[a] = struct{}{}
		}
		for _, a := range c.MultiAddress {
			if _, ok := existing[a]; !ok {
				addrDelta++
			}
		}
		// Merge addresses (dedupe) and update metadata.
		e.Multiaddrs = DedupeStrings(c.MultiAddress, e.Multiaddrs)
		e.Source = c.Source
		e.LastSeen = now
		return false, addrDelta
	}

	// New peer.
	s.peers[c.PeerID] = &PeerEntry{
		PeerID:     c.PeerID,
		Multiaddrs: DedupeStrings(c.MultiAddress, nil),
		Source:     c.Source,
		FirstSeen:  now,
		LastSeen:   now,
	}
	return true, len(c.MultiAddress)
}

func (s *PeerStore) Sink() func(context.Context, Candidate) {
	return func(_ context.Context, c Candidate) {
		created, addrDelta := s.UpsertFromCandidate(c)
		switch {
		case created:
			log.Info("peer added", "peerID", c.PeerID, "source", c.Source, "addrs", len(c.MultiAddress))
		case addrDelta > 0:
			log.Info("peer updated", "peerID", c.PeerID, "source", c.Source, "new_addrs", addrDelta)
		default:
			log.Debug("peer seen", "peerID", c.PeerID, "source", c.Source)
		}
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
