package cmd

import (
	"fmt"
	"strings"
	"time"

	"bib/internal/tui/component"
	"bib/internal/tui/layout"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// demoCmd represents the demo command
var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Showcase TUI components",
	Long:  `Interactive demonstration of all TUI components. Use this to verify the component library is working correctly.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		themeName, _ := cmd.Flags().GetString("theme")

		// Set theme based on flag
		switch themeName {
		case "light":
			themes.Global().SetActive(themes.PresetLight)
		case "dracula":
			themes.Global().SetActive(themes.PresetDracula)
		case "nord":
			themes.Global().SetActive(themes.PresetNord)
		case "gruvbox":
			themes.Global().SetActive(themes.PresetGruvbox)
		default:
			themes.Global().SetActive(themes.PresetDark)
		}

		p := tea.NewProgram(newDemoModel(), tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(demoCmd)
	demoCmd.Flags().StringP("theme", "t", "dark", "theme preset (dark, light, dracula, nord, gruvbox)")
}

// Demo sections
type demoSection int

const (
	sectionOverview demoSection = iota
	sectionStatus
	sectionContainers
	sectionNavigation
	sectionTable
	sectionList
	sectionTree
	sectionModal
	sectionToast
	sectionLayout
	sectionThemes
)

var sectionNames = []string{
	"Overview",
	"Status Components",
	"Containers",
	"Navigation",
	"Table",
	"List",
	"Tree",
	"Modal",
	"Toast",
	"Layout",
	"Themes",
}

// demoModel is the main Bubble Tea model for the demo
type demoModel struct {
	theme   *themes.Theme
	section demoSection
	width   int
	height  int

	// Components
	spinner      *component.Spinner
	progress     float64
	table        *component.Table
	list         *component.List
	tree         *component.Tree
	modal        *component.Modal
	toastManager *component.ToastManager
	tabs         *component.Tabs

	// State
	showModal bool
	quitting  bool
}

func newDemoModel() *demoModel {
	theme := themes.Global().Active()

	m := &demoModel{
		theme:        theme,
		section:      sectionOverview,
		spinner:      component.NewSpinner().WithLabel("Loading..."),
		progress:     0.35,
		toastManager: component.NewToastManager(),
	}

	// Initialize table
	m.table = component.NewTable().
		WithColumns(
			component.TableColumn{Title: "ID", Width: 6},
			component.TableColumn{Title: "Name", Width: 20, Flex: 1},
			component.TableColumn{Title: "Status", Width: 10},
			component.TableColumn{Title: "Created", Width: 12},
		).
		WithRows(
			component.TableRow{ID: "1", Cells: []string{"1", "Dataset Alpha", "Active", "2024-01-15"}},
			component.TableRow{ID: "2", Cells: []string{"2", "Dataset Beta", "Pending", "2024-02-20"}},
			component.TableRow{ID: "3", Cells: []string{"3", "Dataset Gamma", "Active", "2024-03-10"}},
			component.TableRow{ID: "4", Cells: []string{"4", "Dataset Delta", "Inactive", "2024-04-05"}},
			component.TableRow{ID: "5", Cells: []string{"5", "Dataset Epsilon", "Active", "2024-05-01"}},
		).
		WithStriped(true)

	// Initialize list
	m.list = component.NewList().
		WithItems(
			component.ListItem{ID: "1", Title: "Create new dataset", Description: "Initialize a new dataset from scratch", Icon: "📦"},
			component.ListItem{ID: "2", Title: "Import from file", Description: "Import data from CSV, JSON, or YAML", Icon: "📥"},
			component.ListItem{ID: "3", Title: "Clone repository", Description: "Clone an existing dataset repository", Icon: "📋"},
			component.ListItem{ID: "4", Title: "Connect to remote", Description: "Connect to a remote bib daemon", Icon: "🔗"},
			component.ListItem{ID: "5", Title: "View documentation", Description: "Open the bib documentation", Icon: "📖"},
		)

	// Initialize tree
	root := &component.TreeNode{
		ID:    "root",
		Label: "bib",
		Children: []*component.TreeNode{
			{
				ID:    "datasets",
				Label: "datasets",
				Children: []*component.TreeNode{
					{ID: "d1", Label: "alpha"},
					{ID: "d2", Label: "beta"},
					{ID: "d3", Label: "gamma"},
				},
			},
			{
				ID:    "jobs",
				Label: "jobs",
				Children: []*component.TreeNode{
					{ID: "j1", Label: "sync-daily"},
					{ID: "j2", Label: "backup-weekly"},
				},
			},
			{
				ID:    "config",
				Label: "config",
				Children: []*component.TreeNode{
					{ID: "c1", Label: "local.yaml"},
					{ID: "c2", Label: "remote.yaml"},
				},
			},
		},
	}
	root.Children[0].Expanded = true
	m.tree = component.NewTree().WithRoot(root).WithShowRoot(true)

	// Initialize modal
	m.modal = component.NewModal().
		WithTitle("Confirm Action").
		WithContent("Are you sure you want to proceed with this action? This operation cannot be undone.").
		AddAction("Cancel", "esc", func() tea.Cmd {
			return func() tea.Msg { return hideModalMsg{} }
		}).
		AddPrimaryAction("Confirm", "enter", func() tea.Cmd {
			return func() tea.Msg { return hideModalMsg{} }
		})

	// Initialize tabs for navigation demo
	m.tabs = component.NewTabs(
		component.TabItem{ID: "tab1", Title: "General", Badge: "3"},
		component.TabItem{ID: "tab2", Title: "Advanced"},
		component.TabItem{ID: "tab3", Title: "Settings"},
	)

	return m
}

type hideModalMsg struct{}
type tickMsg time.Time

func (m *demoModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Init(),
		tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

func (m *demoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global keys
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
			idx := int(msg.String()[0] - '0')
			if idx == 0 {
				idx = 10
			} else {
				idx--
			}
			if idx < len(sectionNames) {
				m.section = demoSection(idx)
			}

		case "left", "h":
			if m.section > 0 {
				m.section--
			}

		case "right", "l":
			if int(m.section) < len(sectionNames)-1 {
				m.section++
			}

		case "m":
			// Show modal
			m.showModal = true
			m.modal.Show()

		case "t":
			// Show toast
			cmds = append(cmds, m.toastManager.AddSuccess("Action completed successfully!"))

		case "e":
			// Show error toast
			cmds = append(cmds, m.toastManager.AddError("An error occurred"))

		case "w":
			// Show warning toast
			cmds = append(cmds, m.toastManager.AddWarning("Warning: Check your settings"))

		case "i":
			// Show info toast
			cmds = append(cmds, m.toastManager.AddInfo("New update available"))
		}

		// Section-specific key handling
		if m.showModal {
			var cmd tea.Cmd
			_, cmd = m.modal.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			switch m.section {
			case sectionTable:
				m.table.Focus()
				_, cmd := m.table.Update(msg)
				cmds = append(cmds, cmd)
			case sectionList:
				m.list.Focus()
				_, cmd := m.list.Update(msg)
				cmds = append(cmds, cmd)
			case sectionTree:
				m.tree.Focus()
				_, cmd := m.tree.Update(msg)
				cmds = append(cmds, cmd)
			case sectionNavigation:
				m.tabs.Focus()
				_, cmd := m.tabs.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.WithSize(msg.Width-4, 10)
		m.list.WithSize(msg.Width-4, 10)
		m.tree.WithSize(msg.Width-4, 10)
		m.modal.SetContainerSize(msg.Width, msg.Height)
		m.toastManager.WithWidth(msg.Width)

	case tickMsg:
		// Update progress
		m.progress += 0.01
		if m.progress > 1 {
			m.progress = 0
		}

		// Update spinner
		var cmd tea.Cmd
		_, cmd = m.spinner.Update(component.SpinnerTickMsg{})
		cmds = append(cmds, cmd)

		cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}))

	case hideModalMsg:
		m.showModal = false
		m.modal.Hide()

	case component.ToastDismissMsg:
		m.toastManager.Update(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m *demoModel) View() string {
	if m.quitting {
		return ""
	}

	theme := m.theme
	var b strings.Builder

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n\n")

	// Section tabs
	sectionTabs := m.renderSectionTabs()
	b.WriteString(sectionTabs)
	b.WriteString("\n\n")

	// Content
	content := m.renderSection()
	b.WriteString(content)

	// Footer
	b.WriteString("\n\n")
	footer := m.renderFooter()
	b.WriteString(footer)

	// Toast overlay
	if m.toastManager.HasToasts() {
		toasts := m.toastManager.View(m.width)
		if toasts != "" {
			b.WriteString("\n")
			b.WriteString(toasts)
		}
	}

	// Modal overlay
	if m.showModal {
		return m.modal.ViewWidth(m.width)
	}

	// Apply container padding
	result := b.String()
	if m.width > 0 {
		result = lipgloss.NewStyle().
			Width(m.width).
			Padding(1, 2).
			Render(result)
	}

	return theme.Base.Render(result)
}

func (m *demoModel) renderHeader() string {
	theme := m.theme

	title := theme.Title.Render("◆ bib TUI Component Demo")
	subtitle := theme.Description.Render("Interactive showcase of all TUI components")

	return title + "\n" + subtitle
}

func (m *demoModel) renderSectionTabs() string {
	theme := m.theme
	var tabs []string

	for i, name := range sectionNames {
		var style lipgloss.Style
		if demoSection(i) == m.section {
			style = theme.TabActive
		} else {
			style = theme.TabInactive
		}

		label := fmt.Sprintf("%d:%s", (i+1)%10, name)
		tabs = append(tabs, style.Render(label))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

func (m *demoModel) renderSection() string {
	switch m.section {
	case sectionOverview:
		return m.renderOverview()
	case sectionStatus:
		return m.renderStatusComponents()
	case sectionContainers:
		return m.renderContainers()
	case sectionNavigation:
		return m.renderNavigation()
	case sectionTable:
		return m.renderTableDemo()
	case sectionList:
		return m.renderListDemo()
	case sectionTree:
		return m.renderTreeDemo()
	case sectionModal:
		return m.renderModalDemo()
	case sectionToast:
		return m.renderToastDemo()
	case sectionLayout:
		return m.renderLayoutDemo()
	case sectionThemes:
		return m.renderThemesDemo()
	default:
		return ""
	}
}

func (m *demoModel) renderOverview() string {
	theme := m.theme
	var b strings.Builder

	b.WriteString(theme.SectionTitle.Render("Component Library Overview"))
	b.WriteString("\n\n")

	// Stats
	stats := component.NewKeyValueList().
		Add("Total Components", "15+").
		Add("Theme Presets", "5").
		Add("Layout Primitives", "4").
		Add("Current Theme", string(themes.Global().ActiveName()))

	b.WriteString(stats.View(40))
	b.WriteString("\n\n")

	// Component categories
	categories := []struct {
		name  string
		items []string
	}{
		{"Status", []string{"Spinner", "ProgressBar", "Badge", "StatusMessage"}},
		{"Containers", []string{"Card", "Box", "Panel", "KeyValue", "Divider"}},
		{"Navigation", []string{"Breadcrumb", "Tabs", "StepIndicator"}},
		{"Interactive", []string{"Table", "List", "Tree", "Modal", "Toast", "SplitView"}},
		{"Layout", []string{"Flex", "Grid", "Container", "Responsive"}},
	}

	for _, cat := range categories {
		b.WriteString(theme.Focused.Render(cat.name + ": "))
		b.WriteString(theme.Base.Render(strings.Join(cat.items, ", ")))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *demoModel) renderStatusComponents() string {
	theme := m.theme
	var b strings.Builder

	b.WriteString(theme.SectionTitle.Render("Status Components"))
	b.WriteString("\n\n")

	// Spinner
	b.WriteString(theme.Focused.Render("Spinner: "))
	b.WriteString(m.spinner.View())
	b.WriteString("\n\n")

	// Progress bars
	b.WriteString(theme.Focused.Render("Progress Bar:"))
	b.WriteString("\n")

	progress := component.NewProgressBar().
		WithProgress(m.progress).
		WithWidth(40).
		WithPercent(true)
	b.WriteString(progress.View(60))
	b.WriteString("\n")

	progressLabeled := component.NewProgressBar().
		WithProgress(0.75).
		WithWidth(30).
		WithLabel("Upload")
	b.WriteString(progressLabeled.View(60))
	b.WriteString("\n\n")

	// Badges
	b.WriteString(theme.Focused.Render("Badges: "))
	badges := []string{
		component.NewBadge("Primary").Primary().View(0),
		component.NewBadge("Success").Success().View(0),
		component.NewBadge("Warning").Warning().View(0),
		component.NewBadge("Error").Error().View(0),
		component.NewBadge("Info").Info().View(0),
		component.NewBadge("Neutral").View(0),
	}
	b.WriteString(strings.Join(badges, " "))
	b.WriteString("\n\n")

	// Status messages
	b.WriteString(theme.Focused.Render("Status Messages:"))
	b.WriteString("\n")
	b.WriteString(component.Success("Operation completed successfully").View(0))
	b.WriteString("\n")
	b.WriteString(component.Error("Failed to connect to server").View(0))
	b.WriteString("\n")
	b.WriteString(component.Warning("Configuration may be outdated").View(0))
	b.WriteString("\n")
	b.WriteString(component.Info("New version available").View(0))
	b.WriteString("\n")
	b.WriteString(component.Pending("Waiting for response...").View(0))

	return b.String()
}

func (m *demoModel) renderContainers() string {
	theme := m.theme
	var b strings.Builder

	b.WriteString(theme.SectionTitle.Render("Container Components"))
	b.WriteString("\n\n")

	// Card
	card := component.NewCard().
		WithTitle("Sample Card").
		WithContent("This is a card component with a title, content, and footer. Cards are great for grouping related information.").
		WithFooter("Last updated: just now")
	b.WriteString(card.View(50))
	b.WriteString("\n\n")

	// Box
	box := component.NewBox("Simple bordered container").WithTitle("Box")
	b.WriteString(box.View(40))
	b.WriteString("\n\n")

	// Key-Value
	b.WriteString(theme.Focused.Render("Key-Value Pairs:"))
	b.WriteString("\n")
	kv := component.NewKeyValueList().
		Add("Name", "bib-demo").
		Add("Version", "1.0.0").
		Add("Status", "Active")
	b.WriteString(kv.View(40))
	b.WriteString("\n\n")

	// Dividers
	b.WriteString(theme.Focused.Render("Dividers:"))
	b.WriteString("\n")
	b.WriteString(component.NewDivider().View(40))
	b.WriteString("\n")
	b.WriteString(component.NewDivider().WithStyle(component.DividerDouble).View(40))
	b.WriteString("\n")
	b.WriteString(component.NewDivider().WithStyle(component.DividerDashed).View(40))

	return b.String()
}

func (m *demoModel) renderNavigation() string {
	theme := m.theme
	var b strings.Builder

	b.WriteString(theme.SectionTitle.Render("Navigation Components"))
	b.WriteString("\n\n")

	// Breadcrumb
	b.WriteString(theme.Focused.Render("Breadcrumb:"))
	b.WriteString("\n")
	breadcrumb := component.NewBreadcrumb("Home", "Projects", "bib", "Settings")
	b.WriteString(breadcrumb.View(60))
	b.WriteString("\n\n")

	// Tabs
	b.WriteString(theme.Focused.Render("Tabs (use ←/→ to navigate):"))
	b.WriteString("\n")
	b.WriteString(m.tabs.View())
	b.WriteString("\n\n")

	// Step Indicator
	b.WriteString(theme.Focused.Render("Step Indicator:"))
	b.WriteString("\n")
	steps := component.NewStepIndicator("Setup", "Configure", "Review", "Complete").
		WithCurrent(1)
	b.WriteString(steps.View(60))
	b.WriteString("\n\n")

	// Vertical Step Indicator
	b.WriteString(theme.Focused.Render("Vertical Steps:"))
	b.WriteString("\n")
	vsteps := component.NewVerticalStepIndicator(
		component.StepInfo{Title: "Identity", Description: "Configure your name and email"},
		component.StepInfo{Title: "Server", Description: "Set up connection settings"},
		component.StepInfo{Title: "Storage", Description: "Choose database backend"},
		component.StepInfo{Title: "Complete", Description: "Review and save"},
	).WithCurrent(1)
	b.WriteString(vsteps.View(60))

	return b.String()
}

func (m *demoModel) renderTableDemo() string {
	theme := m.theme
	var b strings.Builder

	b.WriteString(theme.SectionTitle.Render("Table Component"))
	b.WriteString("\n")
	b.WriteString(theme.Description.Render("Use ↑/↓ to navigate, Enter to select"))
	b.WriteString("\n\n")

	b.WriteString(m.table.View())

	if row := m.table.SelectedRow(); row != nil {
		b.WriteString("\n\n")
		b.WriteString(theme.Focused.Render("Selected: "))
		b.WriteString(row.Cells[1])
	}

	return b.String()
}

func (m *demoModel) renderListDemo() string {
	theme := m.theme
	var b strings.Builder

	b.WriteString(theme.SectionTitle.Render("List Component"))
	b.WriteString("\n")
	b.WriteString(theme.Description.Render("Use ↑/↓ to navigate"))
	b.WriteString("\n\n")

	b.WriteString(m.list.View())

	if item := m.list.SelectedItem(); item != nil {
		b.WriteString("\n\n")
		b.WriteString(theme.Focused.Render("Selected: "))
		b.WriteString(item.Title)
	}

	return b.String()
}

func (m *demoModel) renderTreeDemo() string {
	theme := m.theme
	var b strings.Builder

	b.WriteString(theme.SectionTitle.Render("Tree Component"))
	b.WriteString("\n")
	b.WriteString(theme.Description.Render("Use ↑/↓ to navigate, Enter/→ to expand, ←/h to collapse"))
	b.WriteString("\n\n")

	b.WriteString(m.tree.View())

	if node := m.tree.SelectedNode(); node != nil {
		b.WriteString("\n\n")
		b.WriteString(theme.Focused.Render("Selected: "))
		b.WriteString(strings.Join(m.tree.SelectedPath(), " → "))
	}

	return b.String()
}

func (m *demoModel) renderModalDemo() string {
	theme := m.theme
	var b strings.Builder

	b.WriteString(theme.SectionTitle.Render("Modal Component"))
	b.WriteString("\n\n")

	b.WriteString(theme.Base.Render("Press "))
	b.WriteString(theme.HelpKey.Render("m"))
	b.WriteString(theme.Base.Render(" to open a modal dialog"))
	b.WriteString("\n\n")

	// Show a preview
	preview := component.NewModal().
		WithTitle("Example Modal").
		WithContent("This is what a modal looks like. Press 'm' to see it in action!")
	b.WriteString(preview.ViewWidth(50))

	return b.String()
}

func (m *demoModel) renderToastDemo() string {
	theme := m.theme
	var b strings.Builder

	b.WriteString(theme.SectionTitle.Render("Toast Notifications"))
	b.WriteString("\n\n")

	b.WriteString(theme.Base.Render("Press these keys to show toasts:"))
	b.WriteString("\n\n")

	keys := []struct {
		key  string
		desc string
	}{
		{"t", "Success toast"},
		{"e", "Error toast"},
		{"w", "Warning toast"},
		{"i", "Info toast"},
	}

	for _, k := range keys {
		b.WriteString("  ")
		b.WriteString(theme.HelpKey.Render(k.key))
		b.WriteString(theme.Base.Render(" - " + k.desc))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(theme.Description.Render("Toasts appear at the top and auto-dismiss after 3 seconds"))

	return b.String()
}

func (m *demoModel) renderLayoutDemo() string {
	theme := m.theme
	var b strings.Builder

	b.WriteString(theme.SectionTitle.Render("Layout System"))
	b.WriteString("\n\n")

	// Current breakpoint
	bp := layout.GetBreakpoint(m.width)
	b.WriteString(theme.Focused.Render("Current Breakpoint: "))
	b.WriteString(theme.Base.Render(layout.BreakpointName(bp)))
	b.WriteString(fmt.Sprintf(" (%dx%d)", m.width, m.height))
	b.WriteString("\n\n")

	// Flex demo
	b.WriteString(theme.Focused.Render("Flex Row:"))
	b.WriteString("\n")
	flexRow := layout.NewFlex().
		Direction(layout.Row).
		Gap(2).
		Items(
			theme.BadgePrimary.Render(" Item 1 "),
			theme.BadgeSuccess.Render(" Item 2 "),
			theme.BadgeWarning.Render(" Item 3 "),
		).
		Render()
	b.WriteString(flexRow)
	b.WriteString("\n\n")

	// Grid demo
	b.WriteString(theme.Focused.Render("Grid (3 columns):"))
	b.WriteString("\n")
	gridItems := []string{
		theme.Box.Width(15).Render("Cell 1"),
		theme.Box.Width(15).Render("Cell 2"),
		theme.Box.Width(15).Render("Cell 3"),
		theme.Box.Width(15).Render("Cell 4"),
		theme.Box.Width(15).Render("Cell 5"),
		theme.Box.Width(15).Render("Cell 6"),
	}
	grid := layout.NewGrid(3).
		Width(60).
		Gap(1).
		Items(gridItems...).
		Render()
	b.WriteString(grid)
	b.WriteString("\n\n")

	// Responsive info
	b.WriteString(theme.Focused.Render("Responsive Recommendations:"))
	b.WriteString("\n")
	ctx := layout.NewContext(m.width, m.height)
	recs := component.NewKeyValueList().
		Add("Columns", fmt.Sprintf("%d", ctx.Columns())).
		Add("Padding", fmt.Sprintf("%d", ctx.Padding())).
		Add("Gap", fmt.Sprintf("%d", ctx.Gap())).
		Add("Sidebar Width", fmt.Sprintf("%d", ctx.SidebarWidth())).
		Add("Modal Width", fmt.Sprintf("%d", ctx.ModalWidth()))
	b.WriteString(recs.View(40))

	return b.String()
}

func (m *demoModel) renderThemesDemo() string {
	theme := m.theme
	var b strings.Builder

	b.WriteString(theme.SectionTitle.Render("Theme System"))
	b.WriteString("\n\n")

	// Current theme
	b.WriteString(theme.Focused.Render("Active Theme: "))
	b.WriteString(theme.Base.Render(theme.Name))
	b.WriteString("\n\n")

	// Available presets
	b.WriteString(theme.Focused.Render("Available Presets:"))
	b.WriteString("\n")
	presets := themes.Global().ListPresets()
	for _, p := range presets {
		prefix := "  "
		if p == themes.Global().ActiveName() {
			prefix = themes.IconCheck + " "
		}
		b.WriteString(prefix + string(p) + "\n")
	}
	b.WriteString("\n")

	// Color palette preview
	b.WriteString(theme.Focused.Render("Color Palette:"))
	b.WriteString("\n")
	palette := theme.Palette
	colors := []struct {
		name  string
		style lipgloss.Style
	}{
		{"Primary", lipgloss.NewStyle().Foreground(palette.Primary)},
		{"Secondary", lipgloss.NewStyle().Foreground(palette.Secondary)},
		{"Success", lipgloss.NewStyle().Foreground(palette.Success)},
		{"Warning", lipgloss.NewStyle().Foreground(palette.Warning)},
		{"Error", lipgloss.NewStyle().Foreground(palette.Error)},
		{"Info", lipgloss.NewStyle().Foreground(palette.Info)},
	}
	for _, c := range colors {
		b.WriteString("  ")
		b.WriteString(c.style.Render("███"))
		b.WriteString(" ")
		b.WriteString(c.name)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(theme.Description.Render("Run with --theme flag to try different themes:"))
	b.WriteString("\n")
	b.WriteString(theme.Focused.Render("  bib demo --theme dracula"))

	return b.String()
}

func (m *demoModel) renderFooter() string {
	theme := m.theme

	keys := []string{
		theme.HelpKey.Render("1-0") + theme.HelpDesc.Render(" sections"),
		theme.HelpKey.Render("←/→") + theme.HelpDesc.Render(" navigate"),
		theme.HelpKey.Render("m") + theme.HelpDesc.Render(" modal"),
		theme.HelpKey.Render("t/e/w/i") + theme.HelpDesc.Render(" toasts"),
		theme.HelpKey.Render("q") + theme.HelpDesc.Render(" quit"),
	}

	return theme.Help.Render(strings.Join(keys, "  •  "))
}
