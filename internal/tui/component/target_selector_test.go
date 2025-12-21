package component

import (
	"testing"

	"bib/internal/deploy"
)

func TestNewTargetSelector(t *testing.T) {
	selector := NewTargetSelector()

	if selector == nil {
		t.Fatal("selector is nil")
	}

	if selector.Selected != 0 {
		t.Errorf("expected selected 0, got %d", selector.Selected)
	}

	if selector.Detecting {
		t.Error("should not be detecting initially")
	}

	if selector.DetectionDone {
		t.Error("detection should not be done initially")
	}
}

func TestTargetSelector_SelectedTarget(t *testing.T) {
	selector := NewTargetSelector()
	selector.Targets = []*deploy.TargetInfo{
		{Type: deploy.TargetLocal, Available: true},
		{Type: deploy.TargetDocker, Available: true},
	}
	selector.Selected = 1

	target := selector.SelectedTarget()

	if target == nil {
		t.Fatal("target is nil")
	}

	if target.Type != deploy.TargetDocker {
		t.Errorf("expected type %q, got %q", deploy.TargetDocker, target.Type)
	}
}

func TestTargetSelector_SelectedTarget_Empty(t *testing.T) {
	selector := NewTargetSelector()

	target := selector.SelectedTarget()

	if target != nil {
		t.Error("expected nil for empty targets")
	}
}

func TestTargetSelector_SelectedTargetType(t *testing.T) {
	selector := NewTargetSelector()
	selector.Targets = []*deploy.TargetInfo{
		{Type: deploy.TargetLocal, Available: true},
		{Type: deploy.TargetDocker, Available: true},
		{Type: deploy.TargetPodman, Available: false},
	}
	selector.Selected = 2

	targetType := selector.SelectedTargetType()

	if targetType != deploy.TargetPodman {
		t.Errorf("expected type %q, got %q", deploy.TargetPodman, targetType)
	}
}

func TestTargetSelector_SelectedTargetType_Default(t *testing.T) {
	selector := NewTargetSelector()

	targetType := selector.SelectedTargetType()

	// Should default to local when no targets
	if targetType != deploy.TargetLocal {
		t.Errorf("expected default type %q, got %q", deploy.TargetLocal, targetType)
	}
}

func TestTargetSelector_IsSelectedAvailable(t *testing.T) {
	selector := NewTargetSelector()
	selector.Targets = []*deploy.TargetInfo{
		{Type: deploy.TargetLocal, Available: true},
		{Type: deploy.TargetDocker, Available: false},
	}

	selector.Selected = 0
	if !selector.IsSelectedAvailable() {
		t.Error("expected local to be available")
	}

	selector.Selected = 1
	if selector.IsSelectedAvailable() {
		t.Error("expected docker to not be available")
	}
}

func TestTargetSelector_SetSelected(t *testing.T) {
	selector := NewTargetSelector()
	selector.Targets = []*deploy.TargetInfo{
		{Type: deploy.TargetLocal, Available: true},
		{Type: deploy.TargetDocker, Available: true},
		{Type: deploy.TargetPodman, Available: true},
		{Type: deploy.TargetKubernetes, Available: true},
	}

	selector.SetSelected(deploy.TargetKubernetes)

	if selector.Selected != 3 {
		t.Errorf("expected selected 3, got %d", selector.Selected)
	}

	selector.SetSelected(deploy.TargetDocker)

	if selector.Selected != 1 {
		t.Errorf("expected selected 1, got %d", selector.Selected)
	}
}

func TestTargetSelector_GetAvailableTargets(t *testing.T) {
	selector := NewTargetSelector()
	selector.Targets = []*deploy.TargetInfo{
		{Type: deploy.TargetLocal, Available: true},
		{Type: deploy.TargetDocker, Available: false},
		{Type: deploy.TargetPodman, Available: true},
		{Type: deploy.TargetKubernetes, Available: false},
	}

	available := selector.GetAvailableTargets()

	if len(available) != 2 {
		t.Fatalf("expected 2 available targets, got %d", len(available))
	}

	if available[0].Type != deploy.TargetLocal {
		t.Errorf("expected first available to be local")
	}

	if available[1].Type != deploy.TargetPodman {
		t.Errorf("expected second available to be podman")
	}
}

func TestTargetSelector_HasAvailableTarget(t *testing.T) {
	selector := NewTargetSelector()
	selector.Targets = []*deploy.TargetInfo{
		{Type: deploy.TargetLocal, Available: true},
		{Type: deploy.TargetDocker, Available: false},
		{Type: deploy.TargetPodman, Available: true},
		{Type: deploy.TargetKubernetes, Available: false},
	}

	if !selector.HasAvailableTarget(deploy.TargetLocal) {
		t.Error("expected local to be available")
	}

	if selector.HasAvailableTarget(deploy.TargetDocker) {
		t.Error("expected docker to not be available")
	}

	if !selector.HasAvailableTarget(deploy.TargetPodman) {
		t.Error("expected podman to be available")
	}

	if selector.HasAvailableTarget(deploy.TargetKubernetes) {
		t.Error("expected kubernetes to not be available")
	}
}

func TestTargetSelector_TargetSummary(t *testing.T) {
	t.Run("detecting", func(t *testing.T) {
		selector := NewTargetSelector()
		selector.Detecting = true

		summary := selector.TargetSummary()

		if summary != "Detecting..." {
			t.Errorf("expected 'Detecting...', got %q", summary)
		}
	})

	t.Run("with targets", func(t *testing.T) {
		selector := NewTargetSelector()
		selector.Targets = []*deploy.TargetInfo{
			{Type: deploy.TargetLocal, Available: true},
			{Type: deploy.TargetDocker, Available: false},
			{Type: deploy.TargetPodman, Available: true},
			{Type: deploy.TargetKubernetes, Available: false},
		}

		summary := selector.TargetSummary()

		if summary != "2/4 targets available" {
			t.Errorf("expected '2/4 targets available', got %q", summary)
		}
	})
}

func TestTargetSelector_MoveUp(t *testing.T) {
	selector := NewTargetSelector()
	selector.Targets = []*deploy.TargetInfo{
		{Type: deploy.TargetLocal, Available: true},
		{Type: deploy.TargetDocker, Available: true},
		{Type: deploy.TargetPodman, Available: true},
	}
	selector.Selected = 2

	selector.moveUp()
	if selector.Selected != 1 {
		t.Errorf("expected selected 1, got %d", selector.Selected)
	}

	selector.moveUp()
	if selector.Selected != 0 {
		t.Errorf("expected selected 0, got %d", selector.Selected)
	}

	// Can't go above 0
	selector.moveUp()
	if selector.Selected < 0 {
		t.Errorf("should not go below 0, got %d", selector.Selected)
	}
}

func TestTargetSelector_MoveDown(t *testing.T) {
	selector := NewTargetSelector()
	selector.Targets = []*deploy.TargetInfo{
		{Type: deploy.TargetLocal, Available: true},
		{Type: deploy.TargetDocker, Available: true},
		{Type: deploy.TargetPodman, Available: true},
	}
	selector.Selected = 0

	selector.moveDown()
	if selector.Selected != 1 {
		t.Errorf("expected selected 1, got %d", selector.Selected)
	}

	selector.moveDown()
	if selector.Selected != 2 {
		t.Errorf("expected selected 2, got %d", selector.Selected)
	}

	// Can't go past last
	selector.moveDown()
	if selector.Selected > 2 {
		t.Errorf("should not go past last, got %d", selector.Selected)
	}
}

func TestTargetSelector_View_Detecting(t *testing.T) {
	selector := NewTargetSelector()
	selector.Detecting = true

	view := selector.View()

	if !containsSubstr(view, "Detecting") {
		t.Error("expected 'Detecting' in view")
	}
}

func TestTargetSelector_View_WithTargets(t *testing.T) {
	selector := NewTargetSelector()
	selector.DetectionDone = true
	selector.Targets = []*deploy.TargetInfo{
		{Type: deploy.TargetLocal, Available: true, Status: "Available"},
		{Type: deploy.TargetDocker, Available: false, Status: "Not installed"},
	}
	selector.Selected = 0

	view := selector.View()

	if !containsSubstr(view, "Local") {
		t.Error("expected 'Local' in view")
	}
	if !containsSubstr(view, "Docker") {
		t.Error("expected 'Docker' in view")
	}
	if !containsSubstr(view, "▸") {
		t.Error("expected cursor in view")
	}
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
