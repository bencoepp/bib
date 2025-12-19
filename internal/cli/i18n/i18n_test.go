package i18n

import (
	"testing"

	tuii18n "bib/internal/tui/i18n"

	"github.com/spf13/cobra"
)

func TestTranslateCommands(t *testing.T) {
	// Create a test command with i18n annotation
	cmd := &cobra.Command{
		Use:         "test",
		Short:       "bib.short",
		Long:        "bib.long",
		Annotations: MarkForTranslation(),
	}

	child := &cobra.Command{
		Use:         "child",
		Short:       "tui.short",
		Long:        "tui.long",
		Annotations: MarkForTranslation(),
	}
	cmd.AddCommand(child)

	// Initialize i18n
	i := tuii18n.New()
	tuii18n.SetGlobal(i)

	// Verify keys exist before translation
	if !i.Has("cmd.bib.short") {
		t.Fatal("expected cmd.bib.short to exist")
	}

	// Translate
	TranslateCommands(cmd)

	// Check translation happened
	if cmd.Short == "bib.short" {
		t.Errorf("expected Short to be translated, got %q", cmd.Short)
	}
	if cmd.Short != "bib is a CLI client" {
		t.Errorf("expected 'bib is a CLI client', got %q", cmd.Short)
	}

	if child.Short == "tui.short" {
		t.Errorf("expected child Short to be translated, got %q", child.Short)
	}
	if child.Short != "Launch interactive dashboard" {
		t.Errorf("expected 'Launch interactive dashboard', got %q", child.Short)
	}
}

func TestMarkForTranslation(t *testing.T) {
	ann := MarkForTranslation()
	if ann[AnnotationKey] != "true" {
		t.Errorf("expected annotation %q to be 'true', got %q", AnnotationKey, ann[AnnotationKey])
	}
}

func TestMergeAnnotations(t *testing.T) {
	existing := map[string]string{"foo": "bar"}
	merged := MergeAnnotations(existing)

	if merged["foo"] != "bar" {
		t.Error("expected existing annotation to be preserved")
	}
	if merged[AnnotationKey] != "true" {
		t.Error("expected i18n annotation to be added")
	}
}
