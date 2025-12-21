package tui

import (
	"testing"

	"bib/internal/tui/layout"
	"bib/internal/tui/themes"
)

func TestGetTheme(t *testing.T) {
	theme := GetTheme()
	if theme == nil {
		t.Fatal("expected non-nil theme")
	}
}

func TestSetTheme(t *testing.T) {
	// Save original
	originalName := themes.Global().ActiveName()

	// Set to light theme
	SetTheme(themes.PresetLight)

	theme := GetTheme()
	if theme.Name != "light" {
		t.Errorf("expected 'light', got %q", theme.Name)
	}

	// Restore
	themes.Global().SetActive(originalName)
}

func TestAutoDetectTheme(t *testing.T) {
	// This just ensures it doesn't panic
	AutoDetectTheme()

	// Theme should be set to something
	theme := GetTheme()
	if theme == nil {
		t.Fatal("expected non-nil theme after auto-detect")
	}
}

func TestNewLayoutContext(t *testing.T) {
	ctx := NewLayoutContext(80, 24)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestGetBreakpoint_FromTUI(t *testing.T) {
	tests := []struct {
		width    int
		expected layout.Breakpoint
	}{
		{30, layout.BreakpointXS},
		{50, layout.BreakpointSM},
		{80, layout.BreakpointMD},
		{120, layout.BreakpointLG},
		{180, layout.BreakpointXL},
		{240, layout.BreakpointXXL},
	}

	for _, tt := range tests {
		bp := GetBreakpoint(tt.width)
		if bp != tt.expected {
			t.Errorf("GetBreakpoint(%d) = %d, want %d", tt.width, bp, tt.expected)
		}
	}
}

func TestTypeAliases(t *testing.T) {
	// Just verify the type aliases exist and can be used
	_ = new(Badge)
	_ = new(Box)
	_ = new(Card)
	_ = new(Divider)
	_ = new(KeyValue)
	_ = new(List)
	_ = new(Modal)
	_ = new(Panel)
	_ = new(ProgressBar)
	_ = new(Spinner)
	_ = new(SplitView)
	_ = new(StatusMessage)
	_ = new(Table)
	_ = new(Tabs)
	_ = new(Toast)
	_ = new(Tree)
}
