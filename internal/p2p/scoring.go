package p2p

import "strings"

type Preferences struct {
	// PreferRegion boosts peers matching this region (case-insensitive).
	PreferRegion string

	// PreferTags boosts peers containing any of these tags.
	PreferTags []string

	// Weights
	WeightLatency    float64 // default 1.0
	WeightLoad       float64 // default 0.5 (penalize higher load)
	WeightRegion     float64 // default 0.3
	WeightTags       float64 // default 0.2
	MinSamplesForRTT int     // default 3; before this RTT impact is reduced
}

// ScorePeer returns a larger-is-better score. Simple heuristic:
// - Lower latency increases score.
// - Lower load increases score.
// - Region match adds a fixed bonus.
// - Tag matches add incremental bonuses.
// - If insufficient RTT samples, latency impact is dampened.
func ScorePeer(e *PeerEntry, p Preferences) float64 {
	if e == nil {
		return 0
	}
	p = NormalizePreferences(p)

	// Latency component (invert ms; guard zero).
	lat := e.LastRTTMs
	latencyScore := 0.0
	if lat > 0 {
		latencyScore = 1000.0 / lat // e.g., 10ms -> 100, 100ms -> 10
	}

	// Dampen if few samples.
	if len(e.RTTSamples) < p.MinSamplesForRTT {
		latencyScore *= 0.5
	}

	// Load (penalty: higher load reduces score).
	loadPenalty := e.Load                    // assume e.Load ~ [0..N]; tune later
	loadScore := 100.0 / (1.0 + loadPenalty) // 0 load -> 100, load 9 -> 10

	// Region preference.
	regionScore := 0.0
	if p.PreferRegion != "" && strings.EqualFold(e.Region, p.PreferRegion) {
		regionScore = 25.0
	}

	// Tag preference.
	tagScore := 0.0
	if len(p.PreferTags) > 0 && len(e.Tags) > 0 {
		tagSet := make(map[string]struct{}, len(e.Tags))
		for _, t := range e.Tags {
			tagSet[strings.ToLower(t)] = struct{}{}
		}
		for _, pt := range p.PreferTags {
			if _, ok := tagSet[strings.ToLower(pt)]; ok {
				tagScore += 10.0
			}
		}
	}

	// Weighted aggregate.
	score := latencyScore*p.WeightLatency +
		loadScore*p.WeightLoad +
		regionScore*p.WeightRegion +
		tagScore*p.WeightTags

	return score
}

func (p Preferences) IsEmpty() bool {
	return p.PreferRegion == "" &&
		len(p.PreferTags) == 0 &&
		p.WeightLatency == 0 &&
		p.WeightLoad == 0 &&
		p.WeightRegion == 0 &&
		p.WeightTags == 0 &&
		p.MinSamplesForRTT == 0
}
