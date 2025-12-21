package layout

import (
	"testing"
)

func TestDetectCapabilities(t *testing.T) {
	caps := DetectCapabilities()
	if caps == nil {
		t.Fatal("expected non-nil capabilities")
	}

	// Just verify it doesn't panic and returns something
	_ = caps.Unicode
	_ = caps.TrueColor
	_ = caps.Color256
}

func TestGetCapabilities(t *testing.T) {
	caps := GetCapabilities()
	if caps == nil {
		t.Fatal("expected non-nil capabilities")
	}
}

func TestSetCapabilities(t *testing.T) {
	// Save original
	original := GetCapabilities()

	// Set override
	override := &TerminalCapabilities{
		Unicode:   false,
		TrueColor: false,
	}
	SetCapabilities(override)

	caps := GetCapabilities()
	if caps.Unicode != false {
		t.Error("expected Unicode to be false after override")
	}

	// Reset
	ResetCapabilities()

	// After reset, should detect again
	_ = GetCapabilities()
	_ = original // Just to use the variable
}

func TestUnicodeIcons(t *testing.T) {
	icons := UnicodeIcons()

	if icons.Check != "✓" {
		t.Errorf("expected Check to be ✓, got %s", icons.Check)
	}

	if icons.Cross != "✗" {
		t.Errorf("expected Cross to be ✗, got %s", icons.Cross)
	}
}

func TestASCIIIcons(t *testing.T) {
	icons := ASCIIIcons()

	if icons.Check != "[x]" {
		t.Errorf("expected Check to be [x], got %s", icons.Check)
	}

	if icons.Cross != "[X]" {
		t.Errorf("expected Cross to be [X], got %s", icons.Cross)
	}
}

func TestGetIcons(t *testing.T) {
	// Set Unicode capability
	SetCapabilities(&TerminalCapabilities{Unicode: true})
	icons := GetIcons()
	if icons.Check != "✓" {
		t.Error("expected Unicode icons when Unicode is enabled")
	}

	// Set ASCII capability
	SetCapabilities(&TerminalCapabilities{Unicode: false})
	icons = GetIcons()
	if icons.Check != "[x]" {
		t.Error("expected ASCII icons when Unicode is disabled")
	}

	// Reset
	ResetCapabilities()
}

func TestUnicodeBoxChars(t *testing.T) {
	box := UnicodeBoxChars()

	if box.TopLeft != "╭" {
		t.Errorf("expected TopLeft to be ╭, got %s", box.TopLeft)
	}

	if box.Horizontal != "─" {
		t.Errorf("expected Horizontal to be ─, got %s", box.Horizontal)
	}
}

func TestASCIIBoxChars(t *testing.T) {
	box := ASCIIBoxChars()

	if box.TopLeft != "+" {
		t.Errorf("expected TopLeft to be +, got %s", box.TopLeft)
	}

	if box.Horizontal != "-" {
		t.Errorf("expected Horizontal to be -, got %s", box.Horizontal)
	}
}

func TestGetBoxChars(t *testing.T) {
	// Set Unicode capability
	SetCapabilities(&TerminalCapabilities{Unicode: true})
	box := GetBoxChars()
	if box.TopLeft != "╭" {
		t.Error("expected Unicode box chars when Unicode is enabled")
	}

	// Set ASCII capability
	SetCapabilities(&TerminalCapabilities{Unicode: false})
	box = GetBoxChars()
	if box.TopLeft != "+" {
		t.Error("expected ASCII box chars when Unicode is disabled")
	}

	// Reset
	ResetCapabilities()
}
