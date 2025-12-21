package layout

import (
	"testing"
	"time"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewShell(t *testing.T) {
	shell := NewShell()
	if shell == nil {
		t.Fatal("expected non-nil shell")
	}

	if shell.theme == nil {
		t.Error("expected theme to be set")
	}

	if shell.breakpoint != BreakpointMD {
		t.Errorf("expected default breakpoint MD, got %d", shell.breakpoint)
	}
}

func TestShell_WithOptions(t *testing.T) {
	theme := themes.Global().Active()

	shell := NewShell(
		WithTheme(theme),
		WithInfoBar(false),
		WithLogPanel(true),
	)

	if !shell.showLogPanel {
		t.Error("expected log panel to be shown")
	}

	if shell.showInfoBar {
		t.Error("expected info bar to be hidden")
	}
}

func TestShell_AddView(t *testing.T) {
	shell := NewShell()

	view := &testView{id: "test"}
	shell.AddView(view)

	if len(shell.views) != 1 {
		t.Errorf("expected 1 view, got %d", len(shell.views))
	}

	if shell.views[0].ID() != "test" {
		t.Errorf("expected view ID 'test', got %s", shell.views[0].ID())
	}
}

func TestShell_SetActiveView(t *testing.T) {
	shell := NewShell()

	view1 := &testView{id: "view1"}
	view2 := &testView{id: "view2"}

	shell.AddView(view1)
	shell.AddView(view2)

	if !shell.SetActiveView("view2") {
		t.Error("expected SetActiveView to return true")
	}

	if shell.activeView != 1 {
		t.Errorf("expected active view 1, got %d", shell.activeView)
	}

	if !shell.SetActiveView("view1") {
		t.Error("expected SetActiveView to return true")
	}

	if shell.activeView != 0 {
		t.Errorf("expected active view 0, got %d", shell.activeView)
	}

	if shell.SetActiveView("nonexistent") {
		t.Error("expected SetActiveView to return false for nonexistent view")
	}
}

func TestShell_RemoveView(t *testing.T) {
	shell := NewShell()

	view1 := &testView{id: "view1"}
	view2 := &testView{id: "view2"}

	shell.AddView(view1)
	shell.AddView(view2)

	shell.RemoveView("view1")

	if len(shell.views) != 1 {
		t.Errorf("expected 1 view, got %d", len(shell.views))
	}

	if shell.views[0].ID() != "view2" {
		t.Errorf("expected remaining view ID 'view2', got %s", shell.views[0].ID())
	}
}

func TestShell_SetSidebarItems(t *testing.T) {
	shell := NewShell()

	items := []SidebarItem{
		{ID: "item1", Title: "Item 1"},
		{ID: "item2", Title: "Item 2"},
	}

	shell.SetSidebarItems(items)
	// Just verify it doesn't panic
}

func TestShell_AddLogEntry(t *testing.T) {
	shell := NewShell()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   LogLevelInfo,
		Message: "Test message",
	}

	shell.AddLogEntry(entry)
	// Just verify it doesn't panic
}

func TestLayoutMode(t *testing.T) {
	tests := []struct {
		bp       Breakpoint
		expected LayoutMode
	}{
		{BreakpointXS, LayoutMinimal},
		{BreakpointSM, LayoutCompact},
		{BreakpointMD, LayoutStandard},
		{BreakpointLG, LayoutExtended},
		{BreakpointXL, LayoutWide},
		{BreakpointXXL, LayoutUltrawide},
	}

	for _, tt := range tests {
		got := GetLayoutMode(tt.bp)
		if got != tt.expected {
			t.Errorf("GetLayoutMode(%d) = %d, want %d", tt.bp, got, tt.expected)
		}
	}
}

func TestLayoutModeName(t *testing.T) {
	tests := []struct {
		mode     LayoutMode
		expected string
	}{
		{LayoutMinimal, "minimal"},
		{LayoutCompact, "compact"},
		{LayoutStandard, "standard"},
		{LayoutExtended, "extended"},
		{LayoutWide, "wide"},
		{LayoutUltrawide, "ultrawide"},
	}

	for _, tt := range tests {
		got := LayoutModeName(tt.mode)
		if got != tt.expected {
			t.Errorf("LayoutModeName(%d) = %s, want %s", tt.mode, got, tt.expected)
		}
	}
}

func TestBreakpointName(t *testing.T) {
	tests := []struct {
		bp       Breakpoint
		expected string
	}{
		{BreakpointXS, "xs"},
		{BreakpointSM, "sm"},
		{BreakpointMD, "md"},
		{BreakpointLG, "lg"},
		{BreakpointXL, "xl"},
		{BreakpointXXL, "xxl"},
	}

	for _, tt := range tests {
		got := BreakpointName(tt.bp)
		if got != tt.expected {
			t.Errorf("BreakpointName(%d) = %s, want %s", tt.bp, got, tt.expected)
		}
	}
}

func TestDefaultConstraints(t *testing.T) {
	c := DefaultConstraints()

	if c.SidebarMinWidth != 3 {
		t.Errorf("expected SidebarMinWidth 3, got %d", c.SidebarMinWidth)
	}

	if c.SidebarMaxWidth != 40 {
		t.Errorf("expected SidebarMaxWidth 40, got %d", c.SidebarMaxWidth)
	}

	if c.LogPanelMinHeight != 4 {
		t.Errorf("expected LogPanelMinHeight 4, got %d", c.LogPanelMinHeight)
	}
}

func TestConstraintsForBreakpoint(t *testing.T) {
	tests := []struct {
		bp              Breakpoint
		expectedSidebar int
	}{
		{BreakpointXS, 0},
		{BreakpointSM, 3},
		{BreakpointMD, 20},
		{BreakpointLG, 22},
		{BreakpointXL, 24},
		{BreakpointXXL, 26},
	}

	for _, tt := range tests {
		c := ConstraintsForBreakpoint(tt.bp)
		if c.SidebarDefaultWidth != tt.expectedSidebar {
			t.Errorf("ConstraintsForBreakpoint(%d).SidebarDefaultWidth = %d, want %d",
				tt.bp, c.SidebarDefaultWidth, tt.expectedSidebar)
		}
	}
}

// testView is a minimal ContentView implementation for testing
type testView struct {
	id     string
	width  int
	height int
}

func (v *testView) ID() string                              { return v.id }
func (v *testView) Title() string                           { return v.id }
func (v *testView) ShortTitle() string                      { return v.id }
func (v *testView) Icon() string                            { return "" }
func (v *testView) SetSize(w, h int)                        { v.width, v.height = w, h }
func (v *testView) Init() tea.Cmd                           { return nil }
func (v *testView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }
func (v *testView) View() string                            { return "" }
