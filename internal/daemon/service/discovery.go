package service

import (
	"bib/internal/p2p"
	bibv1 "bib/internal/pb/bibd/v1"
	"context"

	"github.com/libp2p/go-libp2p/core/host"
)

type DiscoveryService struct {
	bibv1.UnimplementedDiscoveryServer

	PeerStore *p2p.PeerStore
	Host      host.Host
}

func NewServer(peerStore *p2p.PeerStore, h host.Host) *DiscoveryService {
	return &DiscoveryService{PeerStore: peerStore, Host: h}
}

func (s *DiscoveryService) FindCandidates(ctx context.Context, req *bibv1.FindCandidatesRequest) (*bibv1.FindCandidatesResponse, error) {
	if s.PeerStore == nil {
		return &bibv1.FindCandidatesResponse{}, nil
	}
	prefs := p2p.Preferences{
		PreferRegion: req.PreferRegion,
		PreferTags:   req.PreferTags,
	}
	selfID := ""
	if s.Host != nil {
		selfID = s.Host.ID().String()
	}

	views := s.PeerStore.TopCandidates(int(req.Limit), prefs, req.IncludeSelf, selfID)
	resp := &bibv1.FindCandidatesResponse{
		Candidates: make([]*bibv1.PeerCandidate, 0, len(views)),
	}
	for _, v := range views {
		resp.Candidates = append(resp.Candidates, &bibv1.PeerCandidate{
			PeerId:     v.PeerID,
			Multiaddrs: v.MultiAddresses,
			Score:      v.Score,
			LastRttMs:  v.LastRTTMs,
			Load:       v.Load,
			Region:     v.Region,
			Tags:       v.Tags,
			Source:     v.Source,
		})
	}
	return resp, nil
}
