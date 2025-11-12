package watcher

import (
	"bib/internal/capcheck"
	"bib/internal/capcheck/checks"
	"bib/internal/config"
	"bib/internal/contexts"
	"context"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

type CapabilityWatcher struct {
	mu        sync.RWMutex
	current   *contexts.CapabilityContext
	stopCh    chan struct{}
	updatesCh chan *contexts.CapabilityContext
	interval  time.Duration
	runner    *capcheck.Runner
	cfg       *config.BibDaemonConfig
}

func StartCapabilityChecks(cfg *config.BibDaemonConfig) *CapabilityWatcher {
	interval := 5 * time.Minute
	if cfg != nil && cfg.General.CapabilityRefreshInterval > 0 {
		interval = time.Duration(cfg.General.CapabilityRefreshInterval) * time.Minute
	}

	runner := capcheck.NewRunner(
		checks.AllCheckers(),
		capcheck.WithGlobalTimeout(15*time.Second),
		capcheck.WithPerCheckTimeout(3*time.Second),
	)

	w := &CapabilityWatcher{
		current:   &contexts.CapabilityContext{GenericCapabilities: map[string]capcheck.CheckResult{}},
		stopCh:    make(chan struct{}),
		updatesCh: make(chan *contexts.CapabilityContext, 1),
		interval:  interval,
		runner:    runner,
		cfg:       cfg,
	}

	// initial sync run
	w.runOnce(context.Background(), true)
	go w.loop()
	return w
}

func (w *CapabilityWatcher) loop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.runOnce(context.Background(), false)
		case <-w.stopCh:
			return
		}
	}
}

func (w *CapabilityWatcher) runOnce(ctx context.Context, first bool) {
	if w.cfg != nil && !w.cfg.General.CheckCapabilities {
		if first {
			log.Info("Capability checks disabled; returning empty context.")
		}
		return
	}
	results := w.runner.Run(ctx)

	w.mu.Lock()
	prev := w.current
	newCtx := BuildCapabilityContext(prev, results)
	newCtx.LastUpdated = time.Now()
	w.current = newCtx
	w.mu.Unlock()

	w.logDiff(prev, newCtx, first)
	select {
	case w.updatesCh <- newCtx:
	default:
	}
}

func (w *CapabilityWatcher) logDiff(old, cur *contexts.CapabilityContext, first bool) {
	if first || old == nil {
		log.Info("Initial capability scan complete.")
		return
	}
	// Core deltas
	type delta struct {
		id   string
		prev bool
		cur  bool
		errP string
		errC string
	}
	deltas := []delta{
		{"container_runtime", old.ContainerRuntime.Responsive, cur.ContainerRuntime.Responsive, old.ContainerRuntime.LastResult.Error, cur.ContainerRuntime.LastResult.Error},
		{"internet_access", old.Internet.Supported, cur.Internet.Supported, old.Internet.LastResult.Error, cur.Internet.LastResult.Error},
		{"resources", old.Resources.Supported, cur.Resources.Supported, old.Resources.LastResult.Error, cur.Resources.LastResult.Error},
	}
	for _, d := range deltas {
		if d.prev != d.cur || d.errP != d.errC {
			if d.cur {
				log.Infof("✅ capability %s now supported", d.id)
			} else {
				log.Errorf("⛔ capability %s unsupported: %s", d.id, d.errC)
			}
		}
	}
	// Generic capabilities: flag flips
	for id, res := range cur.GenericCapabilities {
		oldRes, ok := old.GenericCapabilities[id]
		if !ok {
			log.Infof("capability %s added supported=%v", id, res.Supported)
			continue
		}
		if oldRes.Supported != res.Supported || oldRes.Error != res.Error {
			if res.Supported {
				log.Infof("✅ capability %s now supported", id)
			} else {
				log.Errorf("⛔ capability %s unsupported: %s", id, res.Error)
			}
		}
	}
}

func (w *CapabilityWatcher) Current() *contexts.CapabilityContext {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.current
}

func (w *CapabilityWatcher) Updates() <-chan *contexts.CapabilityContext {
	return w.updatesCh
}

func (w *CapabilityWatcher) Stop() {
	close(w.stopCh)
}
