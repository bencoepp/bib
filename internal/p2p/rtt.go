package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	lppeer "github.com/libp2p/go-libp2p/core/peer"
	lpping "github.com/libp2p/go-libp2p/p2p/protocol/ping"
)

type TCPPingFunc func(ctx context.Context, peerID string) (time.Duration, error)

// RTTSamplerOptions control the sampling behavior.
type RTTSamplerOptions struct {
	// How often to scan the peer set for sampling.
	Interval time.Duration

	// Minimum interval per peer between samples to avoid hammering.
	PerPeerMinInterval time.Duration

	// Number of concurrent probes (libp2p + optional TCP).
	Concurrency int

	// Number of ping messages per peer for libp2p; average is used.
	PingsPerPeer int

	// Timeouts.
	ConnectTimeout time.Duration
	PingTimeout    time.Duration

	// Optional TCP ping function. If nil, only libp2p ping is used.
	TCPPing TCPPingFunc

	// Optional scoring preferences. If non-zero, sampler recomputes and stores score after updates.
	Preferences Preferences
}

func (o *RTTSamplerOptions) normalize() {
	if o.Interval <= 0 {
		o.Interval = 1 * time.Second
	}
	if o.PerPeerMinInterval <= 0 {
		o.PerPeerMinInterval = 2 * time.Second
	}
	if o.Concurrency <= 0 {
		o.Concurrency = 4
	}
	if o.PingsPerPeer <= 0 {
		o.PingsPerPeer = 3
	}
	if o.ConnectTimeout <= 0 {
		o.ConnectTimeout = 5 * time.Second
	}
	if o.PingTimeout <= 0 {
		o.PingTimeout = 3 * time.Second
	}
}

// RTTSampler manages background RTT sampling.
type RTTSampler struct {
	host  host.Host
	store *PeerStore
	opts  RTTSamplerOptions

	cancel  context.CancelFunc
	wg      sync.WaitGroup
	lastRun sync.Map // peerID -> time.Time of last sample
}

// StartRTTSampler starts background RTT sampling for peers in store.
// Call Stop() to terminate.
func StartRTTSampler(parent context.Context, h host.Host, store *PeerStore, opts RTTSamplerOptions) *RTTSampler {
	opts.normalize()

	ctx, cancel := context.WithCancel(parent)
	s := &RTTSampler{
		host:   h,
		store:  store,
		opts:   opts,
		cancel: cancel,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(opts.Interval)
		defer ticker.Stop()

		// Initial run shortly after start.
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				s.sampleOnce(ctx)
			case <-ticker.C:
				s.sampleOnce(ctx)
			}
		}
	}()

	return s
}

// Stop terminates the sampler.
func (s *RTTSampler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *RTTSampler) sampleOnce(ctx context.Context) {
	snap := s.store.Snapshot()
	sem := make(chan struct{}, s.opts.Concurrency)
	var wg sync.WaitGroup

	for _, e := range snap {
		peerID := e.PeerID

		// Per-peer throttling.
		if lastAny, ok := s.lastRun.Load(peerID); ok {
			if t, _ := lastAny.(time.Time); !t.IsZero() && time.Since(t) < s.opts.PerPeerMinInterval {
				continue
			}
		}
		s.lastRun.Store(peerID, time.Now())

		sem <- struct{}{}
		wg.Add(1)
		go func(e PeerEntry) {
			defer func() {
				<-sem
				wg.Done()
			}()
			s.samplePeer(ctx, &e)
		}(e)
	}

	wg.Wait()
}

func (s *RTTSampler) samplePeer(ctx context.Context, e *PeerEntry) {
	var best time.Duration
	var any bool

	// 1) libp2p ping
	if d, err := s.pingP2P(ctx, e.PeerID); err == nil {
		best = d
		any = true
	} else {
		// Not fatal, just log at debug level later; keep quiet for now to avoid spam.
	}

	// 2) Optional TCP ping
	if s.opts.TCPPing != nil {
		if d, err := s.opts.TCPPing(ctx, e.PeerID); err == nil {
			if !any || d < best {
				best = d
				any = true
			}
		}
	}

	if any {
		s.store.AddRTTSample(e.PeerID, best)
		// Optional score refresh.
		if !s.opts.Preferences.IsEmpty() {
			latest := s.store.Get(e.PeerID)
			score := ScorePeer(latest, s.opts.Preferences)
			s.store.UpdateScore(e.PeerID, score)
		}
	}
}

func (s *RTTSampler) pingP2P(parent context.Context, peerID string) (time.Duration, error) {
	pid, err := lppeer.Decode(peerID)
	if err != nil {
		return 0, err
	}

	// Ensure we have a connection (best-effort).
	{
		ctx, cancel := context.WithTimeout(parent, s.opts.ConnectTimeout)
		defer cancel()
		_ = s.host.Connect(ctx, lppeer.AddrInfo{ID: pid})
	}

	// Use libp2p ping protocol.
	pinger := lpping.NewPingService(s.host)

	var count int
	var sum time.Duration

	for i := 0; i < s.opts.PingsPerPeer; i++ {
		ctx, cancel := context.WithTimeout(parent, s.opts.PingTimeout)
		ch := pinger.Ping(ctx, pid)
		select {
		case res, ok := <-ch:
			cancel()
			if !ok || res.Error != nil {
				continue
			}
			count++
			sum += res.RTT
		case <-ctx.Done():
			cancel()
			// timeout
		}
	}

	if count == 0 {
		return 0, context.DeadlineExceeded
	}
	avg := sum / time.Duration(count)
	return avg, nil
}
