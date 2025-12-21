package component

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bib/internal/deploy"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TargetSelector is a TUI component for selecting deployment targets
type TargetSelector struct {
	// Targets contains information about available targets
	Targets []*deploy.TargetInfo

	// Selected is the currently selected target index
	Selected int

	// Detecting indicates if detection is in progress
	Detecting bool

	// DetectionDone indicates if detection has completed
	DetectionDone bool

	// Error contains any error from detection
	Error error

	// spinner for detection progress
	spinner spinner.Model

	// styles
	selectedStyle   lipgloss.Style
	unselectedStyle lipgloss.Style
	disabledStyle   lipgloss.Style
	headerStyle     lipgloss.Style
	descStyle       lipgloss.Style
}

// NewTargetSelector creates a new target selector
func NewTargetSelector() *TargetSelector {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &TargetSelector{
		Targets:   make([]*deploy.TargetInfo, 0),
		Selected:  0,
		Detecting: false,
		spinner:   s,
		selectedStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")),
		unselectedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		disabledStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			MarginBottom(1),
		descStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true),
	}
}

// TargetDetectionMsg is sent when target detection completes
type TargetDetectionMsg struct {
	Targets []*deploy.TargetInfo
	Error   error
}

// StartDetection starts target detection
func (s *TargetSelector) StartDetection() tea.Cmd {
	s.Detecting = true
	return tea.Batch(
		s.spinner.Tick,
		func() tea.Msg {
			detector := deploy.NewTargetDetector().WithTimeout(5 * time.Second)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			targets := detector.DetectAll(ctx)
			return TargetDetectionMsg{Targets: targets}
		},
	)
}

// Init implements tea.Model
func (s *TargetSelector) Init() tea.Cmd {
	return s.StartDetection()
}

// Update implements tea.Model
func (s *TargetSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TargetDetectionMsg:
		s.Detecting = false
		s.DetectionDone = true
		s.Targets = msg.Targets
		s.Error = msg.Error

		// Select first available target
		for i, t := range s.Targets {
			if t.Available {
				s.Selected = i
				break
			}
		}
		return s, nil

	case spinner.TickMsg:
		if s.Detecting {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return s, cmd
		}
		return s, nil

	case tea.KeyMsg:
		if s.Detecting {
			return s, nil
		}

		switch msg.String() {
		case "up", "k":
			s.moveUp()
		case "down", "j":
			s.moveDown()
		case "enter", " ":
			// Selection confirmed (handled by parent)
		}
	}

	return s, nil
}

// View implements tea.Model
func (s *TargetSelector) View() string {
	var sb strings.Builder

	sb.WriteString(s.headerStyle.Render("Select Deployment Target"))
	sb.WriteString("\n\n")

	if s.Detecting {
		sb.WriteString(s.spinner.View())
		sb.WriteString(" Detecting available deployment targets...\n")
		return sb.String()
	}

	if s.Error != nil {
		sb.WriteString(fmt.Sprintf("Error detecting targets: %v\n", s.Error))
		return sb.String()
	}

	if len(s.Targets) == 0 {
		sb.WriteString("No deployment targets found.\n")
		return sb.String()
	}

	for i, target := range s.Targets {
		cursor := "  "
		if i == s.Selected {
			cursor = "▸ "
		}

		// Icon
		icon := s.targetIcon(target.Type)

		// Status icon
		var statusIcon string
		if target.Available {
			statusIcon = "✓"
		} else {
			statusIcon = "✗"
		}

		// Format line
		name := deploy.TargetDisplayName(target.Type)
		line := fmt.Sprintf("%s%s %s %s", cursor, icon, statusIcon, name)

		// Apply style based on selection and availability
		var style lipgloss.Style
		if i == s.Selected {
			style = s.selectedStyle
		} else if target.Available {
			style = s.unselectedStyle
		} else {
			style = s.disabledStyle
		}

		sb.WriteString(style.Render(line))

		// Add status on same line
		status := fmt.Sprintf(" - %s", target.Status)
		if i == s.Selected {
			sb.WriteString(s.selectedStyle.Render(status))
		} else {
			sb.WriteString(s.disabledStyle.Render(status))
		}
		sb.WriteString("\n")

		// Show description for selected target
		if i == s.Selected {
			desc := deploy.TargetDescription(target.Type)
			sb.WriteString("     ")
			sb.WriteString(s.descStyle.Render(desc))
			sb.WriteString("\n")

			// Show details if available
			if len(target.Details) > 0 && target.Available {
				for k, v := range target.Details {
					sb.WriteString(fmt.Sprintf("     • %s: %s\n", k, v))
				}
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(s.disabledStyle.Render("↑/↓ navigate • enter select"))

	return sb.String()
}

// moveUp moves selection up to the next available target
func (s *TargetSelector) moveUp() {
	for i := s.Selected - 1; i >= 0; i-- {
		s.Selected = i
		return
	}
}

// moveDown moves selection down to the next available target
func (s *TargetSelector) moveDown() {
	for i := s.Selected + 1; i < len(s.Targets); i++ {
		s.Selected = i
		return
	}
}

// targetIcon returns the icon for a target type
func (s *TargetSelector) targetIcon(t deploy.TargetType) string {
	switch t {
	case deploy.TargetLocal:
		return "🖥️ "
	case deploy.TargetDocker:
		return "🐳"
	case deploy.TargetPodman:
		return "🦭"
	case deploy.TargetKubernetes:
		return "☸️ "
	default:
		return "  "
	}
}

// SelectedTarget returns the currently selected target
func (s *TargetSelector) SelectedTarget() *deploy.TargetInfo {
	if s.Selected >= 0 && s.Selected < len(s.Targets) {
		return s.Targets[s.Selected]
	}
	return nil
}

// SelectedTargetType returns the type of the selected target
func (s *TargetSelector) SelectedTargetType() deploy.TargetType {
	if target := s.SelectedTarget(); target != nil {
		return target.Type
	}
	return deploy.TargetLocal
}

// IsSelectedAvailable returns true if the selected target is available
func (s *TargetSelector) IsSelectedAvailable() bool {
	if target := s.SelectedTarget(); target != nil {
		return target.Available
	}
	return false
}

// SetSelected sets the selected target by type
func (s *TargetSelector) SetSelected(t deploy.TargetType) {
	for i, target := range s.Targets {
		if target.Type == t {
			s.Selected = i
			return
		}
	}
}

// GetAvailableTargets returns all available targets
func (s *TargetSelector) GetAvailableTargets() []*deploy.TargetInfo {
	available := make([]*deploy.TargetInfo, 0)
	for _, t := range s.Targets {
		if t.Available {
			available = append(available, t)
		}
	}
	return available
}

// HasAvailableTarget checks if a specific target type is available
func (s *TargetSelector) HasAvailableTarget(t deploy.TargetType) bool {
	for _, target := range s.Targets {
		if target.Type == t && target.Available {
			return true
		}
	}
	return false
}

// TargetSummary returns a summary of detected targets
func (s *TargetSelector) TargetSummary() string {
	if s.Detecting {
		return "Detecting..."
	}

	available := 0
	for _, t := range s.Targets {
		if t.Available {
			available++
		}
	}

	return fmt.Sprintf("%d/%d targets available", available, len(s.Targets))
}
