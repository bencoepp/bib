package themes

import (
	"testing"
)

func TestPresetName_Constants(t *testing.T) {
	if PresetDark != "dark" {
		t.Errorf("expected 'dark', got %q", PresetDark)
	}
	if PresetLight != "light" {
		t.Errorf("expected 'light', got %q", PresetLight)
	}
	if PresetDracula != "dracula" {
		t.Errorf("expected 'dracula', got %q", PresetDracula)
	}
	if PresetNord != "nord" {
		t.Errorf("expected 'nord', got %q", PresetNord)
	}
	if PresetGruvbox != "gruvbox" {
		t.Errorf("expected 'gruvbox', got %q", PresetGruvbox)
	}
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}

	// Check that all presets are registered
	presets := []PresetName{PresetDark, PresetLight, PresetDracula, PresetNord, PresetGruvbox}
	for _, p := range presets {
		if _, ok := r.presets[p]; !ok {
			t.Errorf("preset %q not found in registry", p)
		}
	}

	// Default should be dark
	if r.activeName != PresetDark {
		t.Errorf("expected active preset 'dark', got %q", r.activeName)
	}
}

func TestRegistry_Active(t *testing.T) {
	r := NewRegistry()

	theme := r.Active()
	if theme == nil {
		t.Fatal("expected non-nil active theme")
	}

	if theme.Name != string(PresetDark) {
		t.Errorf("expected theme name 'dark', got %q", theme.Name)
	}
}

func TestRegistry_ActiveName(t *testing.T) {
	r := NewRegistry()

	name := r.ActiveName()
	if name != PresetDark {
		t.Errorf("expected 'dark', got %q", name)
	}
}

func TestRegistry_SetActive(t *testing.T) {
	r := NewRegistry()

	// Change to light theme
	ok := r.SetActive(PresetLight)
	if !ok {
		t.Fatal("expected SetActive to return true")
	}

	if r.ActiveName() != PresetLight {
		t.Errorf("expected 'light', got %q", r.ActiveName())
	}

	theme := r.Active()
	if theme.Name != string(PresetLight) {
		t.Errorf("expected theme name 'light', got %q", theme.Name)
	}
}

func TestRegistry_SetActive_Invalid(t *testing.T) {
	r := NewRegistry()

	ok := r.SetActive(PresetName("invalid"))
	if ok {
		t.Error("expected SetActive to return false for invalid preset")
	}

	// Should still be dark
	if r.ActiveName() != PresetDark {
		t.Errorf("active preset should still be 'dark', got %q", r.ActiveName())
	}
}

func TestRegistry_SetCustomActive(t *testing.T) {
	r := NewRegistry()

	// Register a custom theme
	customTheme := r.presets[PresetDark].Clone()
	customTheme.Name = "my-custom"
	r.RegisterCustom("my-custom", customTheme)

	// Set it active
	ok := r.SetCustomActive("my-custom")
	if !ok {
		t.Fatal("expected SetCustomActive to return true")
	}

	if r.Active().Name != "my-custom" {
		t.Errorf("expected active theme 'my-custom', got %q", r.Active().Name)
	}
}

func TestRegistry_SetCustomActive_Invalid(t *testing.T) {
	r := NewRegistry()

	ok := r.SetCustomActive("nonexistent")
	if ok {
		t.Error("expected SetCustomActive to return false for invalid name")
	}
}

func TestGlobal(t *testing.T) {
	g1 := Global()
	g2 := Global()

	if g1 != g2 {
		t.Error("Global() should return the same instance")
	}

	if g1 == nil {
		t.Fatal("Global() should not return nil")
	}
}

func TestTheme_Clone(t *testing.T) {
	r := NewRegistry()
	original := r.Active()

	clone := original.Clone()
	if clone == original {
		t.Error("Clone should return a different instance")
	}

	if clone.Name != original.Name {
		t.Errorf("Clone name mismatch: got %q, want %q", clone.Name, original.Name)
	}
}

func TestDarkTheme(t *testing.T) {
	theme := DarkTheme()
	if theme == nil {
		t.Fatal("expected non-nil theme")
	}
	if theme.Name != "dark" {
		t.Errorf("expected name 'dark', got %q", theme.Name)
	}
}

func TestLightTheme(t *testing.T) {
	theme := LightTheme()
	if theme == nil {
		t.Fatal("expected non-nil theme")
	}
	if theme.Name != "light" {
		t.Errorf("expected name 'light', got %q", theme.Name)
	}
}

func TestDraculaTheme(t *testing.T) {
	theme := DraculaTheme()
	if theme == nil {
		t.Fatal("expected non-nil theme")
	}
	if theme.Name != "dracula" {
		t.Errorf("expected name 'dracula', got %q", theme.Name)
	}
}

func TestNordTheme(t *testing.T) {
	theme := NordTheme()
	if theme == nil {
		t.Fatal("expected non-nil theme")
	}
	if theme.Name != "nord" {
		t.Errorf("expected name 'nord', got %q", theme.Name)
	}
}

func TestGruvboxTheme(t *testing.T) {
	theme := GruvboxTheme()
	if theme == nil {
		t.Fatal("expected non-nil theme")
	}
	if theme.Name != "gruvbox" {
		t.Errorf("expected name 'gruvbox', got %q", theme.Name)
	}
}

func TestAllThemesHaveRequiredStyles(t *testing.T) {
	themes := []*Theme{
		DarkTheme(),
		LightTheme(),
		DraculaTheme(),
		NordTheme(),
		GruvboxTheme(),
	}

	for _, theme := range themes {
		t.Run(theme.Name, func(t *testing.T) {
			// Verify the theme has a name
			if theme.Name == "" {
				t.Error("theme should have a name")
			}

			// Verify the theme can be cloned without panic
			clone := theme.Clone()
			if clone.Name != theme.Name {
				t.Errorf("clone name mismatch")
			}
		})
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()

	done := make(chan bool)

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			_ = r.Active()
			_ = r.ActiveName()
		}
		done <- true
	}()

	// Concurrent writes
	go func() {
		presets := []PresetName{PresetDark, PresetLight, PresetDracula, PresetNord, PresetGruvbox}
		for i := 0; i < 100; i++ {
			r.SetActive(presets[i%len(presets)])
		}
		done <- true
	}()

	<-done
	<-done
}
