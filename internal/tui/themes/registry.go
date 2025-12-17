// Package themes provides a theme registry and preset management for the TUI.
package themes

import (
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// PresetName identifies a built-in theme preset
type PresetName string

const (
	PresetDark    PresetName = "dark"
	PresetLight   PresetName = "light"
	PresetDracula PresetName = "dracula"
	PresetNord    PresetName = "nord"
	PresetGruvbox PresetName = "gruvbox"
)

// Registry manages theme presets and the active theme
type Registry struct {
	mu         sync.RWMutex
	presets    map[PresetName]*Theme
	active     *Theme
	activeName PresetName
	custom     map[string]*Theme
}

var (
	globalRegistry *Registry
	once           sync.Once
)

// Global returns the global theme registry
func Global() *Registry {
	once.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}

// NewRegistry creates a new theme registry with built-in presets
func NewRegistry() *Registry {
	r := &Registry{
		presets: make(map[PresetName]*Theme),
		custom:  make(map[string]*Theme),
	}

	// Register built-in presets
	r.presets[PresetDark] = DarkTheme()
	r.presets[PresetLight] = LightTheme()
	r.presets[PresetDracula] = DraculaTheme()
	r.presets[PresetNord] = NordTheme()
	r.presets[PresetGruvbox] = GruvboxTheme()

	// Default to dark theme
	r.active = r.presets[PresetDark]
	r.activeName = PresetDark

	return r
}

// Active returns the currently active theme
func (r *Registry) Active() *Theme {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active
}

// ActiveName returns the name of the currently active preset
func (r *Registry) ActiveName() PresetName {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.activeName
}

// SetActive sets the active theme by preset name
func (r *Registry) SetActive(name PresetName) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if theme, ok := r.presets[name]; ok {
		r.active = theme
		r.activeName = name
		return true
	}
	return false
}

// SetCustomActive sets a custom theme as active
func (r *Registry) SetCustomActive(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if theme, ok := r.custom[name]; ok {
		r.active = theme
		r.activeName = PresetName(name)
		return true
	}
	return false
}

// RegisterCustom registers a custom theme
func (r *Registry) RegisterCustom(name string, theme *Theme) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.custom[name] = theme
}

// Get returns a preset theme by name
func (r *Registry) Get(name PresetName) *Theme {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.presets[name]
}

// GetCustom returns a custom theme by name
func (r *Registry) GetCustom(name string) *Theme {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.custom[name]
}

// ListPresets returns all available preset names
func (r *Registry) ListPresets() []PresetName {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]PresetName, 0, len(r.presets))
	for name := range r.presets {
		names = append(names, name)
	}
	return names
}

// ListCustom returns all registered custom theme names
func (r *Registry) ListCustom() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.custom))
	for name := range r.custom {
		names = append(names, name)
	}
	return names
}

// DetectColorScheme attempts to detect if the terminal prefers light or dark mode
func DetectColorScheme() PresetName {
	// lipgloss can detect this via HasDarkBackground
	if lipgloss.HasDarkBackground() {
		return PresetDark
	}
	return PresetLight
}

// AutoDetect sets the active theme based on terminal detection
func (r *Registry) AutoDetect() {
	r.SetActive(DetectColorScheme())
}
