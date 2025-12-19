package app

import (
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
)

// Page represents a full-screen page in the TUI.
type Page interface {
	tea.Model

	// ID returns the unique page identifier.
	ID() string

	// Title returns the page title for display.
	Title() string

	// ShortHelp returns brief keybinding help for the status bar.
	ShortHelp() []KeyBinding

	// FullHelp returns complete keybinding help for the help dialog.
	FullHelp() [][]KeyBinding

	// SetSize updates the page dimensions.
	SetSize(width, height int)
}

// KeyBinding represents a keybinding for help display.
type KeyBinding struct {
	Key  string
	Help string
}

// Router manages page navigation.
type Router struct {
	app         *App
	pages       []Page
	currentPage int
	width       int
	height      int
}

// NewRouter creates a new router with default pages.
func NewRouter(app *App) *Router {
	r := &Router{
		app:         app,
		pages:       make([]Page, 0),
		currentPage: 0,
	}

	// Register default pages
	// Pages will be registered by the pages package

	return r
}

// RegisterPage adds a page to the router.
func (r *Router) RegisterPage(page Page) {
	r.pages = append(r.pages, page)
}

// RegisterPages adds multiple pages.
func (r *Router) RegisterPages(pages ...Page) {
	r.pages = append(r.pages, pages...)
}

// Pages returns all registered pages.
func (r *Router) Pages() []Page {
	return r.pages
}

// CurrentPage returns the current page.
func (r *Router) CurrentPage() Page {
	if r.currentPage >= 0 && r.currentPage < len(r.pages) {
		return r.pages[r.currentPage]
	}
	return nil
}

// CurrentIndex returns the current page index.
func (r *Router) CurrentIndex() int {
	return r.currentPage
}

// NavigateTo navigates to a page by ID.
func (r *Router) NavigateTo(pageID string) tea.Cmd {
	for i, page := range r.pages {
		if page.ID() == pageID {
			r.currentPage = i
			return page.Init()
		}
	}
	return nil
}

// NavigateToIndex navigates to a page by index.
func (r *Router) NavigateToIndex(index int) tea.Cmd {
	if index >= 0 && index < len(r.pages) {
		r.currentPage = index
		return r.pages[index].Init()
	}
	return nil
}

// Next navigates to the next page.
func (r *Router) Next() tea.Cmd {
	if len(r.pages) == 0 {
		return nil
	}
	r.currentPage = (r.currentPage + 1) % len(r.pages)
	return r.pages[r.currentPage].Init()
}

// Previous navigates to the previous page.
func (r *Router) Previous() tea.Cmd {
	if len(r.pages) == 0 {
		return nil
	}
	r.currentPage--
	if r.currentPage < 0 {
		r.currentPage = len(r.pages) - 1
	}
	return r.pages[r.currentPage].Init()
}

// SetSize updates the router dimensions.
func (r *Router) SetSize(width, height int) {
	r.width = width
	r.height = height

	// Propagate to all pages
	for _, page := range r.pages {
		page.SetSize(width, height)
	}
}

// Init implements tea.Model.
func (r *Router) Init() tea.Cmd {
	if len(r.pages) > 0 {
		return r.pages[r.currentPage].Init()
	}
	return nil
}

// Update implements tea.Model.
func (r *Router) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle tab navigation
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "tab":
			return r, r.Next()
		case "shift+tab":
			return r, r.Previous()
		}
	}

	// Forward to current page
	if page := r.CurrentPage(); page != nil {
		updatedPage, cmd := page.Update(msg)
		r.pages[r.currentPage] = updatedPage.(Page)
		return r, cmd
	}

	return r, nil
}

// View implements tea.Model.
func (r *Router) View() string {
	if page := r.CurrentPage(); page != nil {
		return r.renderWithChrome(page)
	}
	return "No pages registered"
}

// renderWithChrome renders the page with tab bar and status bar.
func (r *Router) renderWithChrome(page Page) string {
	theme := r.app.Theme()

	// Calculate content area height
	contentHeight := r.height - 2 // Tab bar + status bar

	// Render tab bar
	tabBar := r.renderTabBar(theme)

	// Render page content
	content := page.View()

	// Render status bar
	statusBar := r.renderStatusBar(theme, page)

	// Combine
	return tabBar + "\n" + content + "\n" + statusBar
	// Note: Proper height management would use lipgloss.Height()
	// For now, pages are responsible for their own height
	_ = contentHeight

	return tabBar + "\n" + content + "\n" + statusBar
}

// renderTabBar renders the tab navigation bar.
func (r *Router) renderTabBar(theme *themes.Theme) string {
	var tabs string

	for i, page := range r.pages {
		var style = theme.TabInactive
		if i == r.currentPage {
			style = theme.TabActive
		}

		// Add number shortcut
		tabText := page.Title()
		tabs += style.Render(tabText) + " "
	}

	return theme.TabBar.Width(r.width).Render(tabs)
}

// renderStatusBar renders the bottom status bar with help.
func (r *Router) renderStatusBar(theme *themes.Theme, page Page) string {
	// Get short help from current page
	bindings := page.ShortHelp()

	var help string
	for i, b := range bindings {
		if i > 0 {
			help += "  "
		}
		help += theme.HelpKey.Render(b.Key) + " " + theme.HelpDesc.Render(b.Help)
	}

	// Add global bindings
	help += "  " + theme.HelpKey.Render("?") + " " + theme.HelpDesc.Render("help")
	help += "  " + theme.HelpKey.Render("q") + " " + theme.HelpDesc.Render("quit")

	// Connection status on the right
	status := ""
	if r.app.State().Connected {
		status = theme.Success.Render("● Connected")
	} else {
		status = theme.Error.Render("○ Disconnected")
	}

	// Build status bar with help on left, status on right
	leftWidth := r.width - len(status) - 2
	if leftWidth < 0 {
		leftWidth = 0
	}

	return theme.Help.Width(r.width).Render(help)
}
