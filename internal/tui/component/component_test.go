package component

import (
	"testing"
	"time"

	"bib/internal/tui/themes"
)

func TestSpinnerStyle_Constants(t *testing.T) {
	if SpinnerDots != 0 {
		t.Errorf("expected SpinnerDots = 0, got %d", SpinnerDots)
	}
	if SpinnerLine != 1 {
		t.Errorf("expected SpinnerLine = 1, got %d", SpinnerLine)
	}
	if SpinnerCircle != 2 {
		t.Errorf("expected SpinnerCircle = 2, got %d", SpinnerCircle)
	}
	if SpinnerBounce != 3 {
		t.Errorf("expected SpinnerBounce = 3, got %d", SpinnerBounce)
	}
	if SpinnerPulse != 4 {
		t.Errorf("expected SpinnerPulse = 4, got %d", SpinnerPulse)
	}
	if SpinnerGrow != 5 {
		t.Errorf("expected SpinnerGrow = 5, got %d", SpinnerGrow)
	}
}

func TestNewSpinner(t *testing.T) {
	s := NewSpinner()
	if s == nil {
		t.Fatal("expected non-nil Spinner")
	}

	if s.style != SpinnerDots {
		t.Errorf("expected default style SpinnerDots, got %d", s.style)
	}
	if s.interval != 80*time.Millisecond {
		t.Errorf("expected default interval 80ms, got %v", s.interval)
	}
	if len(s.frames) == 0 {
		t.Error("expected frames to be initialized")
	}
}

func TestSpinner_WithStyle(t *testing.T) {
	styles := []SpinnerStyle{
		SpinnerDots,
		SpinnerLine,
		SpinnerCircle,
		SpinnerBounce,
		SpinnerPulse,
		SpinnerGrow,
	}

	for _, style := range styles {
		s := NewSpinner().WithStyle(style)
		if s.style != style {
			t.Errorf("expected style %d, got %d", style, s.style)
		}
		if len(s.frames) == 0 {
			t.Errorf("expected frames for style %d", style)
		}
	}
}

func TestSpinner_WithLabel(t *testing.T) {
	s := NewSpinner().WithLabel("Loading...")
	if s.label != "Loading..." {
		t.Errorf("expected 'Loading...', got %q", s.label)
	}
}

func TestSpinner_WithInterval(t *testing.T) {
	s := NewSpinner().WithInterval(100 * time.Millisecond)
	if s.interval != 100*time.Millisecond {
		t.Errorf("expected 100ms, got %v", s.interval)
	}
}

func TestSpinner_WithTheme(t *testing.T) {
	theme := themes.DarkTheme()
	s := NewSpinner().WithTheme(theme)
	if s.Theme() != theme {
		t.Error("theme not set correctly")
	}
}

func TestSpinner_Chaining(t *testing.T) {
	theme := themes.DarkTheme()
	s := NewSpinner().
		WithStyle(SpinnerCircle).
		WithLabel("Processing").
		WithInterval(50 * time.Millisecond).
		WithTheme(theme)

	if s.style != SpinnerCircle {
		t.Error("style not set")
	}
	if s.label != "Processing" {
		t.Error("label not set")
	}
	if s.interval != 50*time.Millisecond {
		t.Error("interval not set")
	}
}

func TestNewCard(t *testing.T) {
	c := NewCard()
	if c == nil {
		t.Fatal("expected non-nil Card")
	}

	if !c.bordered {
		t.Error("expected bordered true by default")
	}
	if c.padding != 1 {
		t.Errorf("expected padding 1, got %d", c.padding)
	}
}

func TestCard_WithTitle(t *testing.T) {
	c := NewCard().WithTitle("Test Title")
	if c.title != "Test Title" {
		t.Errorf("expected 'Test Title', got %q", c.title)
	}
}

func TestCard_WithContent(t *testing.T) {
	c := NewCard().WithContent("Some content")
	if c.content != "Some content" {
		t.Errorf("expected 'Some content', got %q", c.content)
	}
}

func TestCard_WithFooter(t *testing.T) {
	c := NewCard().WithFooter("Footer text")
	if c.footer != "Footer text" {
		t.Errorf("expected 'Footer text', got %q", c.footer)
	}
}

func TestCard_WithBorder(t *testing.T) {
	c := NewCard().WithBorder(false)
	if c.bordered {
		t.Error("expected bordered false")
	}
}

func TestCard_WithShadow(t *testing.T) {
	c := NewCard().WithShadow(true)
	if !c.shadow {
		t.Error("expected shadow true")
	}
}

func TestCard_WithPadding(t *testing.T) {
	c := NewCard().WithPadding(3)
	if c.padding != 3 {
		t.Errorf("expected padding 3, got %d", c.padding)
	}
}

func TestCard_WithTheme(t *testing.T) {
	theme := themes.LightTheme()
	c := NewCard().WithTheme(theme)
	if c.Theme() != theme {
		t.Error("theme not set correctly")
	}
}

func TestCard_Chaining(t *testing.T) {
	theme := themes.DarkTheme()
	c := NewCard().
		WithTitle("Title").
		WithContent("Content").
		WithFooter("Footer").
		WithBorder(true).
		WithShadow(true).
		WithPadding(2).
		WithTheme(theme)

	if c.title != "Title" {
		t.Error("title not set")
	}
	if c.content != "Content" {
		t.Error("content not set")
	}
	if c.footer != "Footer" {
		t.Error("footer not set")
	}
	if !c.bordered {
		t.Error("bordered not set")
	}
	if !c.shadow {
		t.Error("shadow not set")
	}
	if c.padding != 2 {
		t.Error("padding not set")
	}
}

func TestCard_View(t *testing.T) {
	c := NewCard().
		WithTitle("Test").
		WithContent("Hello World").
		WithTheme(themes.DarkTheme())

	view := c.View(40)
	if view == "" {
		t.Error("expected non-empty view")
	}
}
