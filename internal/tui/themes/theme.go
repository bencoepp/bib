package themes

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Theme contains all the styles for the TUI components
type Theme struct {
	// Name is the theme identifier
	Name string

	// Palette is the color palette for this theme
	Palette ColorPalette

	// Base styles
	Base        lipgloss.Style
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Description lipgloss.Style

	// Status styles
	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style

	// Interactive styles
	Focused  lipgloss.Style
	Blurred  lipgloss.Style
	Selected lipgloss.Style
	Cursor   lipgloss.Style
	Disabled lipgloss.Style

	// Button styles
	ButtonPrimary   lipgloss.Style
	ButtonSecondary lipgloss.Style
	ButtonDanger    lipgloss.Style
	ButtonGhost     lipgloss.Style

	// Section styles
	Section       lipgloss.Style
	SectionTitle  lipgloss.Style
	SectionBorder lipgloss.Style

	// Help styles
	Help     lipgloss.Style
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style

	// Progress styles
	ProgressBar      lipgloss.Style
	ProgressFilled   lipgloss.Style
	ProgressEmpty    lipgloss.Style
	ProgressComplete lipgloss.Style
	ProgressPending  lipgloss.Style

	// Box/Panel styles
	Box       lipgloss.Style
	BoxTitle  lipgloss.Style
	BoxBorder lipgloss.Style
	Card      lipgloss.Style
	CardTitle lipgloss.Style

	// Tab styles
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	TabBar      lipgloss.Style

	// Table styles
	TableHeader      lipgloss.Style
	TableRow         lipgloss.Style
	TableRowAlt      lipgloss.Style
	TableRowSelected lipgloss.Style
	TableCell        lipgloss.Style
	TableBorder      lipgloss.Style

	// Modal styles
	ModalOverlay   lipgloss.Style
	ModalContainer lipgloss.Style
	ModalTitle     lipgloss.Style
	ModalContent   lipgloss.Style
	ModalFooter    lipgloss.Style

	// Toast/Notification styles
	ToastSuccess lipgloss.Style
	ToastError   lipgloss.Style
	ToastWarning lipgloss.Style
	ToastInfo    lipgloss.Style

	// Breadcrumb styles
	BreadcrumbItem      lipgloss.Style
	BreadcrumbSeparator lipgloss.Style
	BreadcrumbActive    lipgloss.Style

	// Tree styles
	TreeNode     lipgloss.Style
	TreeBranch   lipgloss.Style
	TreeLeaf     lipgloss.Style
	TreeExpanded lipgloss.Style

	// Wizard step styles
	StepComplete lipgloss.Style
	StepCurrent  lipgloss.Style
	StepPending  lipgloss.Style
	StepNumber   lipgloss.Style

	// Form styles
	FormLabel       lipgloss.Style
	FormInput       lipgloss.Style
	FormInputFocus  lipgloss.Style
	FormPlaceholder lipgloss.Style
	FormError       lipgloss.Style
	FormHelp        lipgloss.Style

	// Badge styles
	BadgePrimary lipgloss.Style
	BadgeSuccess lipgloss.Style
	BadgeWarning lipgloss.Style
	BadgeError   lipgloss.Style
	BadgeInfo    lipgloss.Style
	BadgeNeutral lipgloss.Style

	// Spinner styles
	Spinner lipgloss.Style

	// Divider styles
	Divider lipgloss.Style
}

// Clone creates a deep copy of the theme
func (t *Theme) Clone() *Theme {
	clone := *t
	return &clone
}

// WithPalette creates a new theme with the given palette
func (t *Theme) WithPalette(p ColorPalette) *Theme {
	clone := t.Clone()
	clone.Palette = p
	clone.rebuildStyles()
	return clone
}

// rebuildStyles rebuilds all styles from the palette
func (t *Theme) rebuildStyles() {
	p := t.Palette

	t.Base = lipgloss.NewStyle().Foreground(p.Text)
	t.Title = lipgloss.NewStyle().Foreground(p.Primary).Bold(true).MarginBottom(1)
	t.Subtitle = lipgloss.NewStyle().Foreground(p.Secondary).Bold(true)
	t.Description = lipgloss.NewStyle().Foreground(p.TextMuted).Italic(true)

	t.Success = lipgloss.NewStyle().Foreground(p.Success).Bold(true)
	t.Error = lipgloss.NewStyle().Foreground(p.Error).Bold(true)
	t.Warning = lipgloss.NewStyle().Foreground(p.Warning).Bold(true)
	t.Info = lipgloss.NewStyle().Foreground(p.Info)

	t.Focused = lipgloss.NewStyle().Foreground(p.Primary).Bold(true)
	t.Blurred = lipgloss.NewStyle().Foreground(p.TextMuted)
	t.Selected = lipgloss.NewStyle().Foreground(p.Primary).Background(p.Selection)
	t.Cursor = lipgloss.NewStyle().Foreground(p.Cursor)
	t.Disabled = lipgloss.NewStyle().Foreground(p.TextSubtle)

	t.ButtonPrimary = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(p.Primary).
		Padding(0, 3).
		Bold(true)
	t.ButtonSecondary = lipgloss.NewStyle().
		Foreground(p.Text).
		Background(p.BackgroundAlt).
		Padding(0, 3)
	t.ButtonDanger = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(p.Error).
		Padding(0, 3).
		Bold(true)
	t.ButtonGhost = lipgloss.NewStyle().
		Foreground(p.Primary).
		Padding(0, 3)

	t.Section = lipgloss.NewStyle().MarginTop(1).MarginBottom(1)
	t.SectionTitle = lipgloss.NewStyle().Foreground(p.Secondary).Bold(true).MarginBottom(1)
	t.SectionBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Border).
		Padding(1, 2)

	t.Help = lipgloss.NewStyle().Foreground(p.TextSubtle).MarginTop(1)
	t.HelpKey = lipgloss.NewStyle().Foreground(p.Primary).Bold(true)
	t.HelpDesc = lipgloss.NewStyle().Foreground(p.TextMuted)

	t.ProgressBar = lipgloss.NewStyle().Foreground(p.Primary)
	t.ProgressFilled = lipgloss.NewStyle().Foreground(p.Primary)
	t.ProgressEmpty = lipgloss.NewStyle().Foreground(p.TextSubtle)
	t.ProgressComplete = lipgloss.NewStyle().Foreground(p.Success)
	t.ProgressPending = lipgloss.NewStyle().Foreground(p.TextSubtle)

	t.Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Border).
		Padding(1, 2)
	t.BoxTitle = lipgloss.NewStyle().Foreground(p.Primary).Bold(true)
	t.BoxBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.BorderFocus)
	t.Card = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Border).
		Padding(1, 2).
		Background(p.Surface)
	t.CardTitle = lipgloss.NewStyle().Foreground(p.Primary).Bold(true).MarginBottom(1)

	t.TabActive = lipgloss.NewStyle().
		Foreground(p.Primary).
		Background(p.BackgroundAlt).
		Bold(true).
		Padding(0, 2)
	t.TabInactive = lipgloss.NewStyle().
		Foreground(p.TextMuted).
		Padding(0, 2)
	t.TabBar = lipgloss.NewStyle().
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(p.Border)

	t.TableHeader = lipgloss.NewStyle().
		Foreground(p.Primary).
		Bold(true).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(p.Border)
	t.TableRow = lipgloss.NewStyle().Foreground(p.Text)
	t.TableRowAlt = lipgloss.NewStyle().Foreground(p.Text).Background(p.BackgroundAlt)
	t.TableRowSelected = lipgloss.NewStyle().Foreground(p.Primary).Background(p.Selection)
	t.TableCell = lipgloss.NewStyle().Padding(0, 1)
	t.TableBorder = lipgloss.NewStyle().Foreground(p.Border)

	t.ModalOverlay = lipgloss.NewStyle().Background(p.Overlay)
	t.ModalContainer = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.BorderFocus).
		Padding(1, 2).
		Background(p.Surface)
	t.ModalTitle = lipgloss.NewStyle().Foreground(p.Primary).Bold(true).MarginBottom(1)
	t.ModalContent = lipgloss.NewStyle().Foreground(p.Text)
	t.ModalFooter = lipgloss.NewStyle().MarginTop(1)

	t.ToastSuccess = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(p.Success).
		Padding(0, 2).
		Bold(true)
	t.ToastError = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(p.Error).
		Padding(0, 2).
		Bold(true)
	t.ToastWarning = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(p.Warning).
		Padding(0, 2).
		Bold(true)
	t.ToastInfo = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(p.Info).
		Padding(0, 2)

	t.BreadcrumbItem = lipgloss.NewStyle().Foreground(p.TextMuted)
	t.BreadcrumbSeparator = lipgloss.NewStyle().Foreground(p.TextSubtle).Padding(0, 1)
	t.BreadcrumbActive = lipgloss.NewStyle().Foreground(p.Primary).Bold(true)

	t.TreeNode = lipgloss.NewStyle().Foreground(p.Text)
	t.TreeBranch = lipgloss.NewStyle().Foreground(p.TextSubtle)
	t.TreeLeaf = lipgloss.NewStyle().Foreground(p.Text)
	t.TreeExpanded = lipgloss.NewStyle().Foreground(p.Primary)

	t.StepComplete = lipgloss.NewStyle().Foreground(p.Success)
	t.StepCurrent = lipgloss.NewStyle().Foreground(p.Primary).Bold(true)
	t.StepPending = lipgloss.NewStyle().Foreground(p.TextSubtle)
	t.StepNumber = lipgloss.NewStyle().Width(3).Align(lipgloss.Center)

	t.FormLabel = lipgloss.NewStyle().Foreground(p.Text).Bold(true)
	t.FormInput = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Border).
		Padding(0, 1)
	t.FormInputFocus = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.BorderFocus).
		Padding(0, 1)
	t.FormPlaceholder = lipgloss.NewStyle().Foreground(p.TextSubtle)
	t.FormError = lipgloss.NewStyle().Foreground(p.Error)
	t.FormHelp = lipgloss.NewStyle().Foreground(p.TextMuted).Italic(true)

	t.BadgePrimary = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(p.Primary).
		Padding(0, 1)
	t.BadgeSuccess = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(p.Success).
		Padding(0, 1)
	t.BadgeWarning = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(p.Warning).
		Padding(0, 1)
	t.BadgeError = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(p.Error).
		Padding(0, 1)
	t.BadgeInfo = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(p.Info).
		Padding(0, 1)
	t.BadgeNeutral = lipgloss.NewStyle().
		Foreground(p.Text).
		Background(p.BackgroundAlt).
		Padding(0, 1)

	t.Spinner = lipgloss.NewStyle().Foreground(p.Primary)

	t.Divider = lipgloss.NewStyle().Foreground(p.Border)
}

// HuhTheme returns a huh.Theme based on this theme
func (t *Theme) HuhTheme() *huh.Theme {
	ht := huh.ThemeBase()
	p := t.Palette

	ht.Focused.Title = ht.Focused.Title.Foreground(p.Primary).Bold(true)
	ht.Focused.Description = ht.Focused.Description.Foreground(p.TextMuted)
	ht.Focused.Base = ht.Focused.Base.BorderForeground(p.Primary)
	ht.Focused.SelectedOption = ht.Focused.SelectedOption.Foreground(p.Primary)
	ht.Focused.SelectSelector = ht.Focused.SelectSelector.Foreground(p.Primary)
	ht.Focused.TextInput.Cursor = ht.Focused.TextInput.Cursor.Foreground(p.Primary)
	ht.Focused.TextInput.Placeholder = ht.Focused.TextInput.Placeholder.Foreground(p.TextSubtle)

	ht.Blurred.Title = ht.Blurred.Title.Foreground(p.TextMuted)
	ht.Blurred.Description = ht.Blurred.Description.Foreground(p.TextSubtle)

	return ht
}

// buildTheme creates a Theme from a palette
func buildTheme(name string, palette ColorPalette) *Theme {
	t := &Theme{
		Name:    name,
		Palette: palette,
	}
	t.rebuildStyles()
	return t
}

// DarkTheme returns the default dark theme
func DarkTheme() *Theme {
	return buildTheme("dark", DefaultDarkPalette())
}

// LightTheme returns the default light theme
func LightTheme() *Theme {
	return buildTheme("light", DefaultLightPalette())
}

// DraculaTheme returns the Dracula theme
func DraculaTheme() *Theme {
	return buildTheme("dracula", DraculaPalette())
}

// NordTheme returns the Nord theme
func NordTheme() *Theme {
	return buildTheme("nord", NordPalette())
}

// GruvboxTheme returns the Gruvbox theme
func GruvboxTheme() *Theme {
	return buildTheme("gruvbox", GruvboxPalette())
}
