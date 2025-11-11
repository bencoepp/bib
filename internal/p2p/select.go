package p2p

import "sort"

// CandidateView is the presentation struct used to build the RPC response.
type CandidateView struct {
	PeerID         string
	MultiAddresses []string
	Score          float64
	LastRTTMs      float64
	Load           float64
	Region         string
	Tags           []string
	Source         string
}

// TopCandidates returns up to 'limit' ranked peers using given Preferences.
// If prefs.IsEmpty(), score falls back to a basic latency/load heuristic (ScorePeer with defaults).
func (s *PeerStore) TopCandidates(limit int, prefs Preferences, includeSelf bool, selfID string) []CandidateView {
	if limit <= 0 {
		limit = 10
	}
	snap := s.Snapshot()
	type ranked struct {
		view CandidateView
	}
	rankedList := make([]ranked, 0, len(snap))
	for _, e := range snap {
		if !includeSelf && selfID != "" && e.PeerID == selfID {
			continue
		}
		entry := &PeerEntry{
			PeerID:     e.PeerID,
			Multiaddrs: e.Multiaddrs,
			LastRTTMs:  e.LastRTTMs,
			RTTSamples: e.RTTSamples,
			Load:       e.Load,
			Region:     e.Region,
			Tags:       e.Tags,
		}
		var score float64
		if prefs.IsEmpty() {
			score = ScorePeer(entry, Preferences{})
		} else {
			score = ScorePeer(entry, prefs)
		}
		rankedList = append(rankedList, ranked{
			view: CandidateView{
				PeerID:         e.PeerID,
				MultiAddresses: e.Multiaddrs,
				Score:          score,
				LastRTTMs:      e.LastRTTMs,
				Load:           e.Load,
				Region:         e.Region,
				Tags:           e.Tags,
				Source:         e.Source,
			},
		})
	}
	sort.Slice(rankedList, func(i, j int) bool {
		return rankedList[i].view.Score > rankedList[j].view.Score
	})
	if len(rankedList) > limit {
		rankedList = rankedList[:limit]
	}
	out := make([]CandidateView, 0, len(rankedList))
	for _, r := range rankedList {
		out = append(out, r.view)
	}
	return out
}
