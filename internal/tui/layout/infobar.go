package layout

import (
	"fmt"
	"strings"
	"time"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InfoBarData contains the data displayed in the info bar
type InfoBarData struct {
	// User identity
	Username string
	NodeName string

	// Connection
	Connected bool
	PeerCount int

	// Network stats
	UploadSpeed   string
	DownloadSpeed string

	// Sync status
	SyncingItems int
	SyncingLabel string

	// System stats (for wide layouts)
	CPUPercent  int
	MemoryUsage string
	DiskPercent int
	NetSpeed    string

	// Time
	ShowTime bool
	ShowDate bool
}

// InfoBar displays user identity, connection status, and system info
type InfoBar struct {
	theme  *themes.Theme
	width  int
	height int
	data   InfoBarData
	icons  IconSet
}

// NewInfoBar creates a new info bar
func NewInfoBar() *InfoBar {
	return &InfoBar{
		theme:  themes.Global().Active(),
		height: 1,
		icons:  GetIcons(),
		data: InfoBarData{
			ShowTime: true,
		},
	}
}

// SetTheme sets the theme
func (i *InfoBar) SetTheme(theme *themes.Theme) {
	i.theme = theme
}

// SetSize sets the dimensions
func (i *InfoBar) SetSize(width, height int) {
	i.width = width
	i.height = height
}

// SetData updates the info bar data
func (i *InfoBar) SetData(data InfoBarData) {
	i.data = data
}

// Init implements tea.Model
func (i *InfoBar) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (i *InfoBar) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return i, nil
}

// View implements tea.Model
func (i *InfoBar) View() string {
	if i.width <= 0 {
		return ""
	}

	bp := GetBreakpoint(i.width)

	// Build left section (user info)
	leftParts := i.renderUserInfo(bp)

	// Build center section (network stats)
	centerParts := i.renderNetworkInfo(bp)

	// Build right section (time)
	rightParts := i.renderTimeInfo(bp)

	// Combine based on available width
	left := strings.Join(leftParts, "  ")
	center := strings.Join(centerParts, "  ")
	right := strings.Join(rightParts, "  ")

	// Calculate spacing
	leftWidth := lipgloss.Width(left)
	centerWidth := lipgloss.Width(center)
	rightWidth := lipgloss.Width(right)

	totalContent := leftWidth + centerWidth + rightWidth
	availableSpace := i.width - totalContent

	var content string
	if availableSpace < 4 {
		// Not enough space, just show left and right
		gap := i.width - leftWidth - rightWidth
		if gap < 0 {
			gap = 0
		}
		content = left + strings.Repeat(" ", gap) + right
	} else {
		// Space evenly
		leftGap := availableSpace / 2
		rightGap := availableSpace - leftGap
		content = left + strings.Repeat(" ", leftGap) + center + strings.Repeat(" ", rightGap) + right
	}

	style := lipgloss.NewStyle().
		Width(i.width).
		Foreground(i.theme.Palette.Text)

	return style.Render(content)
}

func (i *InfoBar) renderUserInfo(bp Breakpoint) []string {
	parts := make([]string, 0)

	// User icon and name
	userStyle := lipgloss.NewStyle().Foreground(i.theme.Palette.Primary)

	if bp >= BreakpointMD {
		// Full format: 👤 username@node
		userStr := fmt.Sprintf("%s %s", i.icons.User, i.data.Username)
		if i.data.NodeName != "" {
			userStr += "@" + i.data.NodeName
		}
		parts = append(parts, userStyle.Render(userStr))
	} else if bp >= BreakpointSM {
		// Short format: 👤 username
		parts = append(parts, userStyle.Render(fmt.Sprintf("%s %s", i.icons.User, i.data.Username)))
	} else {
		// Icon only
		parts = append(parts, userStyle.Render(i.icons.User))
	}

	// Connection status
	var connStyle lipgloss.Style
	var connIcon, connText string

	if i.data.Connected {
		connStyle = lipgloss.NewStyle().Foreground(i.theme.Palette.Success)
		connIcon = i.icons.Connected
		connText = "Connected"
	} else {
		connStyle = lipgloss.NewStyle().Foreground(i.theme.Palette.Error)
		connIcon = i.icons.Disconnected
		connText = "Disconnected"
	}

	if bp >= BreakpointMD {
		parts = append(parts, connStyle.Render(fmt.Sprintf("%s %s", connIcon, connText)))
	} else {
		parts = append(parts, connStyle.Render(connIcon))
	}

	// Peer count
	if i.data.PeerCount > 0 {
		peerStyle := lipgloss.NewStyle().Foreground(i.theme.Palette.Secondary)
		if bp >= BreakpointLG {
			parts = append(parts, peerStyle.Render(fmt.Sprintf("%s %d peers", i.icons.Peers, i.data.PeerCount)))
		} else if bp >= BreakpointMD {
			parts = append(parts, peerStyle.Render(fmt.Sprintf("%s %d", i.icons.Peers, i.data.PeerCount)))
		} else if bp >= BreakpointSM {
			parts = append(parts, peerStyle.Render(fmt.Sprintf("%d", i.data.PeerCount)))
		}
	}

	return parts
}

func (i *InfoBar) renderNetworkInfo(bp Breakpoint) []string {
	parts := make([]string, 0)

	if bp < BreakpointMD {
		return parts
	}

	netStyle := lipgloss.NewStyle().Foreground(i.theme.Palette.TextMuted)

	// Upload/download speeds
	if i.data.UploadSpeed != "" || i.data.DownloadSpeed != "" {
		if bp >= BreakpointLG {
			if i.data.UploadSpeed != "" {
				parts = append(parts, netStyle.Render(fmt.Sprintf("%s%s", i.icons.Upload, i.data.UploadSpeed)))
			}
			if i.data.DownloadSpeed != "" {
				parts = append(parts, netStyle.Render(fmt.Sprintf("%s%s", i.icons.Download, i.data.DownloadSpeed)))
			}
		}
	}

	// Sync status
	if i.data.SyncingItems > 0 {
		syncStyle := lipgloss.NewStyle().Foreground(i.theme.Palette.Warning)
		if bp >= BreakpointLG {
			parts = append(parts, syncStyle.Render(fmt.Sprintf("%s Syncing %d items", i.icons.Sync, i.data.SyncingItems)))
		} else {
			parts = append(parts, syncStyle.Render(fmt.Sprintf("%s %d", i.icons.Sync, i.data.SyncingItems)))
		}
	}

	// System stats for XL+
	if bp >= BreakpointXL {
		if i.data.CPUPercent > 0 {
			parts = append(parts, netStyle.Render(fmt.Sprintf("CPU: %d%%", i.data.CPUPercent)))
		}
		if i.data.MemoryUsage != "" {
			parts = append(parts, netStyle.Render(fmt.Sprintf("MEM: %s", i.data.MemoryUsage)))
		}
	}

	// Additional stats for XXL
	if bp >= BreakpointXXL {
		if i.data.DiskPercent > 0 {
			parts = append(parts, netStyle.Render(fmt.Sprintf("DISK: %d%%", i.data.DiskPercent)))
		}
		if i.data.NetSpeed != "" {
			parts = append(parts, netStyle.Render(fmt.Sprintf("NET: %s", i.data.NetSpeed)))
		}
	}

	return parts
}

func (i *InfoBar) renderTimeInfo(bp Breakpoint) []string {
	parts := make([]string, 0)

	if !i.data.ShowTime {
		return parts
	}

	timeStyle := lipgloss.NewStyle().Foreground(i.theme.Palette.TextMuted)
	now := time.Now()

	if bp >= BreakpointXXL && i.data.ShowDate {
		// Full date and time
		parts = append(parts, timeStyle.Render(now.Format("Monday, Jan 2 2006  15:04:05")))
	} else if bp >= BreakpointXL {
		// Time with seconds
		parts = append(parts, timeStyle.Render(now.Format("15:04:05")))
	} else if bp >= BreakpointSM {
		// Time without seconds
		parts = append(parts, timeStyle.Render(now.Format("15:04")))
	}

	return parts
}
