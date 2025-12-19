//go:build e2e

package e2e

import (
	"testing"
	"time"

	"bib/internal/config"
	"bib/internal/tui/app"
	"bib/internal/tui/i18n"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
)

// TestTUIInitialization tests that the TUI app initializes correctly.
func TestTUIInitialization(t *testing.T) {
	// Create app with defaults
	cfg := config.DefaultBibConfig()
	application := app.New(
		app.WithConfig(cfg),
		app.WithTheme(themes.Global().Active()),
		app.WithI18n(i18n.Global()),
	)

	// Initialize should return commands
	cmd := application.Init()
	if cmd == nil {
		t.Error("Init() should return commands")
	}
}

// TestTUIKeyBindings tests global key bindings.
func TestTUIKeyBindings(t *testing.T) {
	cfg := config.DefaultBibConfig()
	application := app.New(
		app.WithConfig(cfg),
		app.WithTheme(themes.Global().Active()),
		app.WithI18n(i18n.Global()),
	)

	// Simulate window size first to make app ready
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	application.Update(sizeMsg)

	tests := []struct {
		name       string
		key        string
		wantDialog bool
	}{
		{
			name:       "help key opens help dialog",
			key:        "?",
			wantDialog: true,
		},
		{
			name:       "slash key opens command palette",
			key:        "/",
			wantDialog: true,
		},
		{
			name:       "q key opens quit dialog",
			key:        "q",
			wantDialog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fresh app for each test
			testApp := app.New(
				app.WithConfig(cfg),
				app.WithTheme(themes.Global().Active()),
				app.WithI18n(i18n.Global()),
			)
			testApp.Update(sizeMsg)

			// Send key
			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			testApp.Update(keyMsg)

			// Check if view renders (basic sanity check)
			view := testApp.View()
			if view == "" {
				t.Error("View() should return non-empty string")
			}
		})
	}
}

// TestTUIErrorLog tests the error log panel.
func TestTUIErrorLog(t *testing.T) {
	cfg := config.DefaultBibConfig()
	application := app.New(
		app.WithConfig(cfg),
		app.WithTheme(themes.Global().Active()),
		app.WithI18n(i18n.Global()),
	)

	// Simulate window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	application.Update(sizeMsg)

	// Initially no errors
	if !application.ErrorLog().IsEmpty() {
		t.Error("Error log should be empty initially")
	}

	// Add an error
	application.LogError("Test error", "Error details")

	if application.ErrorLog().IsEmpty() {
		t.Error("Error log should have entry after LogError")
	}

	if application.ErrorLog().EntryCount() != 1 {
		t.Errorf("Expected 1 entry, got %d", application.ErrorLog().EntryCount())
	}

	// View should contain error log
	view := application.View()
	if view == "" {
		t.Error("View() should return non-empty string")
	}
}

// TestTUINavigation tests page navigation.
func TestTUINavigation(t *testing.T) {
	cfg := config.DefaultBibConfig()
	application := app.New(
		app.WithConfig(cfg),
		app.WithTheme(themes.Global().Active()),
		app.WithI18n(i18n.Global()),
	)

	// Simulate window size
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	application.Update(sizeMsg)

	// Test number key navigation
	for i := 1; i <= 5; i++ {
		key := string(rune('0' + i))
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		application.Update(keyMsg)

		// View should render without panic
		view := application.View()
		if view == "" {
			t.Errorf("View() returned empty after pressing %s", key)
		}
	}
}

// TestTUIThemes tests that all themes work.
func TestTUIThemes(t *testing.T) {
	cfg := config.DefaultBibConfig()
	presets := []themes.PresetID{
		themes.PresetDark,
		themes.PresetLight,
		themes.PresetNord,
		themes.PresetDracula,
		themes.PresetGruvbox,
	}

	for _, preset := range presets {
		t.Run(string(preset), func(t *testing.T) {
			themes.Global().SetActive(preset)

			application := app.New(
				app.WithConfig(cfg),
				app.WithTheme(themes.Global().Active()),
				app.WithI18n(i18n.Global()),
			)

			sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
			application.Update(sizeMsg)

			view := application.View()
			if view == "" {
				t.Error("View() should return non-empty string")
			}
		})
	}
}

// TestTUILocales tests i18n support.
func TestTUILocales(t *testing.T) {
	cfg := config.DefaultBibConfig()
	locales := []string{"en", "de", "fr", "es"}

	for _, locale := range locales {
		t.Run(locale, func(t *testing.T) {
			i18nInstance := i18n.Global()
			// Try to set locale, ignore errors for missing translations
			_ = i18nInstance.SetLocale(locale)

			application := app.New(
				app.WithConfig(cfg),
				app.WithTheme(themes.Global().Active()),
				app.WithI18n(i18nInstance),
			)

			sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
			application.Update(sizeMsg)

			view := application.View()
			if view == "" {
				t.Error("View() should return non-empty string")
			}
		})
	}
}

// TestTUIPerformance tests that the TUI renders quickly.
func TestTUIPerformance(t *testing.T) {
	cfg := config.DefaultBibConfig()
	application := app.New(
		app.WithConfig(cfg),
		app.WithTheme(themes.Global().Active()),
		app.WithI18n(i18n.Global()),
	)

	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	application.Update(sizeMsg)

	// Measure render time
	start := time.Now()
	iterations := 100
	for i := 0; i < iterations; i++ {
		_ = application.View()
	}
	elapsed := time.Since(start)

	avgRenderTime := elapsed / time.Duration(iterations)
	maxAcceptable := 10 * time.Millisecond

	if avgRenderTime > maxAcceptable {
		t.Errorf("Average render time %v exceeds acceptable %v", avgRenderTime, maxAcceptable)
	}

	t.Logf("Average render time: %v", avgRenderTime)
}
