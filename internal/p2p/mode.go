package p2p

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"bib/internal/config"

	"github.com/libp2p/go-libp2p/core/host"
)

// NodeMode represents the operation mode of a node.
type NodeMode string

const (
	// NodeModeProxy forwards requests to peers, no local storage.
	NodeModeProxy NodeMode = "proxy"
	// NodeModeSelective subscribes to specific topics on-demand.
	NodeModeSelective NodeMode = "selective"
	// NodeModeFull replicates all data from connected peers.
	NodeModeFull NodeMode = "full"
)

// ParseNodeMode parses a string into a NodeMode.
func ParseNodeMode(s string) (NodeMode, error) {
	switch strings.ToLower(s) {
	case "proxy", "":
		return NodeModeProxy, nil
	case "selective":
		return NodeModeSelective, nil
	case "full":
		return NodeModeFull, nil
	default:
		return "", fmt.Errorf("invalid node mode: %s (must be proxy, selective, or full)", s)
	}
}

// String returns the string representation of the mode.
func (m NodeMode) String() string {
	return string(m)
}

// ModeHandler defines the interface for mode-specific behavior.
type ModeHandler interface {
	// Mode returns the node mode this handler implements.
	Mode() NodeMode

	// Start begins mode-specific operations.
	Start(ctx context.Context) error

	// Stop stops mode-specific operations.
	Stop() error

	// OnConfigUpdate is called when configuration changes at runtime.
	OnConfigUpdate(cfg config.P2PConfig) error
}

// ModeManager manages the current node mode and handles mode switching.
type ModeManager struct {
	host      host.Host
	discovery *Discovery
	configDir string

	mu      sync.RWMutex
	mode    NodeMode
	handler ModeHandler
	cfg     config.P2PConfig

	ctx    context.Context
	cancel context.CancelFunc
}

// NewModeManager creates a new mode manager.
func NewModeManager(h host.Host, discovery *Discovery, cfg config.P2PConfig, configDir string) (*ModeManager, error) {
	mode, err := ParseNodeMode(cfg.Mode)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	mm := &ModeManager{
		host:      h,
		discovery: discovery,
		configDir: configDir,
		mode:      mode,
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
	}

	// Create the appropriate handler
	handler, err := mm.createHandler(mode)
	if err != nil {
		cancel()
		return nil, err
	}
	mm.handler = handler

	return mm, nil
}

// Start begins the mode handler.
func (mm *ModeManager) Start(ctx context.Context) error {
	modeLog := getLogger("mode")
	modeLog.Info("starting mode handler", "mode", mm.mode)

	mm.mu.RLock()
	handler := mm.handler
	mm.mu.RUnlock()

	if handler != nil {
		if err := handler.Start(ctx); err != nil {
			modeLog.Error("failed to start mode handler", "mode", mm.mode, "error", err)
			return err
		}
		modeLog.Info("mode handler started", "mode", mm.mode)
	}
	return nil
}

// Stop stops the mode handler.
func (mm *ModeManager) Stop() error {
	modeLog := getLogger("mode")
	modeLog.Info("stopping mode handler", "mode", mm.mode)

	mm.cancel()

	mm.mu.RLock()
	handler := mm.handler
	mm.mu.RUnlock()

	if handler != nil {
		if err := handler.Stop(); err != nil {
			modeLog.Error("failed to stop mode handler", "mode", mm.mode, "error", err)
			return err
		}
		modeLog.Info("mode handler stopped", "mode", mm.mode)
	}
	return nil
}

// Mode returns the current node mode.
func (mm *ModeManager) Mode() NodeMode {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.mode
}

// SetMode switches to a new mode at runtime.
func (mm *ModeManager) SetMode(mode NodeMode) error {
	modeLog := getLogger("mode")
	modeLog.Info("switching mode", "from", mm.mode, "to", mode)

	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.mode == mode {
		modeLog.Debug("already in requested mode", "mode", mode)
		return nil // Already in this mode
	}

	// Stop current handler
	if mm.handler != nil {
		modeLog.Debug("stopping current handler", "mode", mm.mode)
		if err := mm.handler.Stop(); err != nil {
			modeLog.Error("failed to stop current handler", "error", err)
			return fmt.Errorf("failed to stop current handler: %w", err)
		}
	}

	// Create new handler
	modeLog.Debug("creating new handler", "mode", mode)
	handler, err := mm.createHandler(mode)
	if err != nil {
		modeLog.Error("failed to create handler", "mode", mode, "error", err)
		return fmt.Errorf("failed to create handler for mode %s: %w", mode, err)
	}

	// Start new handler
	if err := handler.Start(mm.ctx); err != nil {
		modeLog.Error("failed to start handler", "mode", mode, "error", err)
		return fmt.Errorf("failed to start handler for mode %s: %w", mode, err)
	}

	mm.mode = mode
	mm.handler = handler

	modeLog.Info("mode switched successfully", "mode", mode)
	return nil
}

// UpdateConfig updates the configuration and notifies the handler.
func (mm *ModeManager) UpdateConfig(cfg config.P2PConfig) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Check if mode changed
	newMode, err := ParseNodeMode(cfg.Mode)
	if err != nil {
		return err
	}

	mm.cfg = cfg

	if newMode != mm.mode {
		// Mode changed, switch handlers
		mm.mu.Unlock()
		err := mm.SetMode(newMode)
		mm.mu.Lock()
		return err
	}

	// Same mode, just update config
	if mm.handler != nil {
		return mm.handler.OnConfigUpdate(cfg)
	}

	return nil
}

// Handler returns the current mode handler.
func (mm *ModeManager) Handler() ModeHandler {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.handler
}

// createHandler creates the appropriate handler for the given mode.
func (mm *ModeManager) createHandler(mode NodeMode) (ModeHandler, error) {
	switch mode {
	case NodeModeProxy:
		return NewProxyHandler(mm.host, mm.discovery, mm.cfg, mm.configDir)
	case NodeModeSelective:
		return NewSelectiveHandler(mm.host, mm.discovery, mm.cfg, mm.configDir)
	case NodeModeFull:
		return NewFullReplicaHandler(mm.host, mm.discovery, mm.cfg, mm.configDir)
	default:
		return nil, fmt.Errorf("unknown mode: %s", mode)
	}
}
