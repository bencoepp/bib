# TUI Component System

This document provides comprehensive documentation for the Terminal User Interface (TUI) component system used in Bib. This guide is intended for developers and coding agents working with the codebase.

---

## Overview

The TUI system is built on top of [Bubble Tea](https://github.com/charmbracelet/bubbletea) (a Go framework for terminal UIs based on The Elm Architecture) and [Lip Gloss](https://github.com/charmbracelet/lipgloss) (for styling). It also integrates with [Huh](https://github.com/charmbracelet/huh) for form handling.

### Architecture

```
internal/tui/
├── tui.go              # Main entry point, re-exports, convenience functions
├── setup.go            # Setup wizard forms (huh integration)
├── wizard.go           # Multi-step wizard component
├── tabs.go             # Tab navigation component
├── component/          # Reusable UI components
│   ├── base.go         # Base interfaces and component primitives
│   ├── containers.go   # Card, Box, Panel components
│   ├── helpers.go      # Utility functions (truncate, pad, wrap)
│   ├── list.go         # Interactive list component
│   ├── modal.go        # Modal dialog component
│   ├── navigation.go   # Breadcrumb, Tabs components
│   ├── splitview.go    # Resizable split pane layout
│   ├── status.go       # Spinner, ProgressBar, StatusMessage
│   ├── table.go        # Interactive data table
│   ├── toast.go        # Toast notification system
│   └── tree.go         # Hierarchical tree view
├── layout/             # Layout primitives
│   ├── flex.go         # Flexbox-like layout
│   ├── grid.go         # CSS Grid-like layout
│   ├── responsive.go   # Responsive breakpoints
│   ├── context.go      # Layout context with dimensions
│   └── container.go    # Container utilities
└── themes/             # Theming system
    ├── theme.go        # Theme struct with all styles
    ├── colors.go       # Color palettes
    ├── icons.go        # Unicode icons and spinners
    └── registry.go     # Global theme registry
```

## Quick Start

### Basic Usage

```go
import (
    "bib/internal/tui"
    "bib/internal/tui/themes"
    "bib/internal/tui/component"
    "bib/internal/tui/layout"
)

// Get the current theme
theme := tui.GetTheme()

// Create a simple card
card := tui.NewCard().
    WithTitle("Welcome").
    WithContent("Hello, World!").
    WithTheme(theme)

fmt.Println(card.View(80)) // Render at 80 columns width
```

### Setting Themes

```go
// Available presets: dark, light, dracula, nord, gruvbox
tui.SetTheme(themes.PresetDracula)

// Auto-detect based on terminal
tui.AutoDetectTheme()

// Get current theme
theme := tui.GetTheme()
```

## Component Types

### Stateless Components (Renderers)

These components don't maintain state and simply render content:

| Component | Description | Constructor |
|-----------|-------------|-------------|
| `Badge` | Inline status tag | `tui.NewBadge(text)` |
| `Box` | Simple bordered container | `tui.NewBox(content)` |
| `Card` | Content card with title/footer | `tui.NewCard()` |
| `Divider` | Horizontal line separator | `tui.NewDivider()` |
| `KeyValue` | Key-value pair display | `tui.NewComponentKeyValue(key, value)` |
| `ProgressBar` | Progress indicator | `tui.NewProgressBar()` |
| `StatusMessage` | Status with icon | `tui.Success(msg)`, `tui.Error(msg)`, etc. |
| `Breadcrumb` | Navigation path | `tui.NewBreadcrumb(items...)` |
| `StepIndicator` | Progress steps | `tui.NewStepIndicator(steps...)` |

### Stateful Components (tea.Model)

These implement `tea.Model` and maintain internal state:

| Component | Description | Constructor |
|-----------|-------------|-------------|
| `List` | Interactive item list with filtering | `tui.NewList()` |
| `Modal` | Overlay dialog | `tui.NewModal()` |
| `Spinner` | Animated loading indicator | `tui.NewSpinner()` |
| `SplitView` | Resizable split panes | `tui.NewSplitView(direction)` |
| `Table` | Data table with selection | `tui.NewTable()` |
| `Tabs` | Tab navigation | `tui.NewTabs(items...)` |
| `Toast` | Temporary notification | `tui.NewToast(msg, type)` |
| `ToastManager` | Manages multiple toasts | `tui.NewToastManager()` |
| `Tree` | Hierarchical tree view | `tui.NewTree()` |

## Component Interfaces

### Base Interfaces (component/base.go)

```go
// Renderer - base interface for width-aware rendering
type Renderer interface {
    ViewWidth(width int) string
}

// StatefulComponent - combines tea.Model with Renderer
type StatefulComponent interface {
    tea.Model
    Renderer
}

// FocusableComponent - can receive/lose focus
type FocusableComponent interface {
    StatefulComponent
    Focus() tea.Cmd
    Blur()
    Focused() bool
}

// ValidatableComponent - supports validation
type ValidatableComponent interface {
    Validate() error
    SetError(err error)
    ClearError()
}

// ResizableComponent - responds to size changes
type ResizableComponent interface {
    SetSize(width, height int)
    Width() int
    Height() int
}

// ScrollableComponent - supports scrolling
type ScrollableComponent interface {
    ScrollUp(lines int)
    ScrollDown(lines int)
    ScrollTop()
    ScrollBottom()
    ScrollOffset() int
    SetScrollOffset(offset int)
}

// AnimatableComponent - supports animations
type AnimatableComponent interface {
    StartAnimation() tea.Cmd
    StopAnimation()
    IsAnimating() bool
}
```

### BaseComponent

All components embed `BaseComponent` for common functionality:

```go
type BaseComponent struct {
    theme  *themes.Theme
    width  int
    height int
}

// Usage in custom component:
type MyComponent struct {
    component.BaseComponent
    // ... custom fields
}

func NewMyComponent() *MyComponent {
    return &MyComponent{
        BaseComponent: component.NewBaseComponent(),
    }
}
```

## Detailed Component Documentation

### Card

A bordered content container with optional title and footer.

```go
card := tui.NewCard().
    WithTitle("Title").
    WithContent("Main content goes here").
    WithFooter("Footer text").
    WithBorder(true).
    WithShadow(true).
    WithPadding(2).
    WithTheme(theme)

output := card.View(80) // Render at 80 columns
```

### Table

Interactive data table with keyboard navigation.

```go
table := tui.NewTable().
    WithColumns(
        component.TableColumn{Title: "ID", Width: 10},
        component.TableColumn{Title: "Name", Width: 30, Flex: 1},
        component.TableColumn{Title: "Status", Width: 15},
    ).
    WithRows(
        component.TableRow{ID: "1", Cells: []string{"1", "Alice", "Active"}},
        component.TableRow{ID: "2", Cells: []string{"2", "Bob", "Inactive"}},
    ).
    WithSize(80, 20).
    WithHeader(true).
    WithStriped(true).
    WithMultiSelect(false)

// In tea.Model Update:
table, cmd = table.Update(msg)

// Keyboard: up/down/j/k to navigate, space to select (multi), pgup/pgdown, home/end
```

### List

Interactive list with filtering and descriptions.

```go
list := tui.NewList().
    WithItems(
        component.ListItem{ID: "1", Title: "Item 1", Description: "Description", Icon: "📁"},
        component.ListItem{ID: "2", Title: "Item 2", Description: "Description", Badge: "new"},
    ).
    WithSize(60, 15).
    WithDescription(true).
    WithIcons(true).
    WithBadges(true)

// Keyboard: up/down/j/k, / to filter, esc to clear filter
```

### Modal

Overlay dialog with actions.

```go
modal := tui.NewModal().
    WithTitle("Confirm Action").
    WithContent("Are you sure you want to proceed?").
    WithSize(50, 0). // 0 height = auto
    WithActions(
        component.ModalAction{Label: "Cancel", Key: "esc", Handler: func() tea.Cmd { return nil }},
        component.ModalAction{Label: "Confirm", Key: "enter", Primary: true, Handler: confirmHandler},
    )

modal.Show()  // Display modal
modal.Hide()  // Hide modal

// Keyboard: tab/left/right to switch actions, enter to activate, esc to close
```

### Tree

Hierarchical tree view with expand/collapse.

```go
root := &component.TreeNode{
    ID: "root",
    Label: "Root",
    Children: []*component.TreeNode{
        {ID: "child1", Label: "Child 1"},
        {ID: "child2", Label: "Child 2", Children: []*component.TreeNode{
            {ID: "grandchild", Label: "Grandchild"},
        }},
    },
}

tree := tui.NewTree().
    WithRoot(root).
    WithSize(40, 15).
    WithShowRoot(true).
    WithIndent(2)

// Keyboard: up/down/j/k, enter/space to toggle expand, left to collapse, right to expand
```

### SplitView

Resizable split pane layout.

```go
split := tui.NewSplitView(layout.Row). // or layout.Column
    WithFirstPane(&component.SplitPane{
        Content: func(w, h int) string { return "Left pane" },
    }).
    WithSecondPane(&component.SplitPane{
        Content: func(w, h int) string { return "Right pane" },
    }).
    WithRatio(0.3). // 30% / 70% split
    WithDivider(true).
    WithSize(80, 24)

// Keyboard: tab to switch panes, ctrl+left/right to resize (horizontal), ctrl+up/down (vertical)
```

### Toast

Temporary notifications.

```go
toast := tui.NewToast("Operation successful!", component.ToastSuccess).
    WithDuration(3 * time.Second)

// In your model:
cmd := toast.Show() // Returns a tea.Cmd that dismisses after duration

// Toast types: ToastInfo, ToastSuccess, ToastWarning, ToastError
```

### Spinner

Animated loading indicator.

```go
spinner := tui.NewSpinner().
    WithStyle(component.SpinnerDots). // Dots, Line, Circle, Bounce, Pulse, Grow
    WithLabel("Loading...").
    WithInterval(80 * time.Millisecond)

cmd := spinner.Start() // Start animation
spinner.Stop()         // Stop animation
```

### Tabs

Tab navigation component.

```go
tabs := tui.NewTabs(
    component.TabItem{ID: "tab1", Title: "Overview", Content: overviewContent},
    component.TabItem{ID: "tab2", Title: "Details", Content: detailsContent},
    component.TabItem{ID: "tab3", Title: "Settings", Content: settingsContent},
)

// Keyboard: tab/right/l for next, shift+tab/left/h for previous
```

## Layout System

### Flex Layout

CSS Flexbox-inspired layout.

```go
flex := layout.NewFlex().
    Direction(layout.Row).     // Row or Column
    Justify(layout.JustifySpaceBetween). // Start, Center, End, SpaceBetween, SpaceAround, SpaceEvenly
    Align(layout.AlignCenter). // Start, Center, End, Stretch
    Gap(2).
    Width(80).
    Item("Left content").
    ItemWithGrow("Center (grows)", 1).
    Item("Right content")

output := flex.Render()
```

### Grid Layout

CSS Grid-inspired layout.

```go
grid := layout.NewGrid(3). // 3 columns
    Width(80).
    Gap(1).
    Item("Cell 1").
    Item("Cell 2").
    Item("Cell 3").
    ItemFull("Full width row"). // Spans all columns
    ItemSpan("Spans 2 cols", 2).
    Item("Last cell")

output := grid.Render()
```

### Responsive Breakpoints

```go
// Get current breakpoint
bp := layout.GetBreakpoint(terminalWidth)

// Breakpoint constants
// BreakpointXS: < 40 cols
// BreakpointSM: 40-59 cols
// BreakpointMD: 60-79 cols
// BreakpointLG: 80-119 cols
// BreakpointXL: 120+ cols

// Responsive value selection
columns := layout.ResponsiveValue(width, layout.Responsive[int]{
    XS: 1,
    SM: 1,
    MD: 2,
    LG: 3,
    XL: 4,
})

// Layout context for complex layouts
ctx := layout.NewContext(width, height)
if ctx.IsSmall() {
    // Render compact layout
} else if ctx.IsLarge() {
    // Render full layout with sidebar
}
```

## Theme System

### Theme Structure

```go
type Theme struct {
    Name    string
    Palette ColorPalette
    
    // Base styles
    Base, Title, Subtitle, Description lipgloss.Style
    
    // Status styles
    Success, Error, Warning, Info lipgloss.Style
    
    // Interactive styles
    Focused, Blurred, Selected, Cursor, Disabled lipgloss.Style
    
    // Button styles
    ButtonPrimary, ButtonSecondary, ButtonDanger, ButtonGhost lipgloss.Style
    
    // Component-specific styles
    Card, CardTitle lipgloss.Style
    TableHeader, TableRow, TableRowAlt, TableRowSelected lipgloss.Style
    ModalOverlay, ModalContainer, ModalTitle lipgloss.Style
    TabActive, TabInactive, TabBar lipgloss.Style
    ToastSuccess, ToastError, ToastWarning, ToastInfo lipgloss.Style
    // ... many more
}
```

### Color Palette

```go
type ColorPalette struct {
    Primary, Secondary, Accent    lipgloss.AdaptiveColor
    Success, Warning, Error, Info lipgloss.AdaptiveColor
    Text, TextMuted, TextSubtle   lipgloss.AdaptiveColor
    Background, BackgroundAlt     lipgloss.AdaptiveColor
    Border, BorderFocus           lipgloss.AdaptiveColor
    Selection, Cursor, Link       lipgloss.AdaptiveColor
}
```

### Available Theme Presets

| Preset | Description |
|--------|-------------|
| `PresetDark` | Default dark theme with purple accents |
| `PresetLight` | Light theme for bright terminals |
| `PresetDracula` | Popular Dracula color scheme |
| `PresetNord` | Nord color scheme (arctic blue) |
| `PresetGruvbox` | Gruvbox retro groove colors |

### Custom Themes

```go
// Create custom theme
customPalette := themes.ColorPalette{
    Primary: lipgloss.AdaptiveColor{Dark: "#FF6B6B", Light: "#FF6B6B"},
    // ... other colors
}

customTheme := themes.DarkTheme().WithPalette(customPalette)
themes.Global().RegisterCustom("mytheme", customTheme)
themes.Global().SetCustomActive("mytheme")
```

### Icons

Common icons are available in `themes/icons.go`:

```go
// Status icons
themes.IconCheck    // ✓
themes.IconCross    // ✗
themes.IconWarning  // ⚠
themes.IconInfo     // ℹ

// Navigation
themes.IconArrowRight    // →
themes.IconChevronRight  // ›
themes.IconTriangleRight // ▶

// Tree
themes.IconTreeBranch     // ├
themes.IconTreeLastBranch // └
themes.IconTreeExpanded   // ▼
themes.IconTreeCollapsed  // ▶

// Progress
themes.IconProgress      // █
themes.IconProgressEmpty // ░

// Spinner frames
themes.SpinnerDots()   // []string{"⠋", "⠙", "⠹", ...}
themes.SpinnerCircle() // []string{"◐", "◓", "◑", "◒"}
```

## Huh Form Integration

The TUI system integrates with [huh](https://github.com/charmbracelet/huh) for forms:

```go
import "github.com/charmbracelet/huh"

// Get matching huh theme
huhTheme := tui.HuhTheme()

form := huh.NewForm(
    huh.NewGroup(
        huh.NewInput().
            Title("Name").
            Value(&name),
        huh.NewSelect[string]().
            Title("Option").
            Options(
                huh.NewOption("Option A", "a"),
                huh.NewOption("Option B", "b"),
            ).
            Value(&option),
    ),
).WithTheme(huhTheme)

err := form.Run()
```

## Wizard Component

Multi-step wizard for complex configuration flows:

```go
steps := []tui.WizardStep{
    {
        ID:          "welcome",
        Title:       "Welcome",
        Description: "Getting started",
        HelpText:    "This wizard will guide you...",
        View:        func(width int) string { return "Welcome content" },
    },
    {
        ID:          "config",
        Title:       "Configuration",
        Description: "Configure settings",
        View:        func(width int) string { return configForm.View() },
        Validate:    func() error { return validateConfig() },
        ShouldSkip:  func() bool { return skipCondition },
    },
}

wizard := tui.NewWizard(
    "Setup Wizard",
    "Configure your application",
    steps,
    onComplete,
    tui.WithCardWidth(65),
    tui.WithHelpPanel(35),
    tui.WithCentering(true),
)

// wizard implements tea.Model
p := tea.NewProgram(wizard)
p.Run()
```

## Helper Functions

### Text Utilities (component/helpers.go)

```go
// Truncation
truncate("Hello World", 8)       // "Hello W…"
truncateLeft("Hello World", 8)   // "…o World"
truncateMiddle("Hello World", 8) // "Hel…rld"

// Padding
padRight("Hi", 10)  // "Hi        "
padLeft("Hi", 10)   // "        Hi"
padCenter("Hi", 10) // "    Hi    "

// Text wrapping
lines := wrapText("Long text...", 40) // []string

// Indentation
indent("line1\nline2", 4)  // "    line1\n    line2"
dedent("    line1\n    line2") // "line1\nline2"

// Helpers
visibleWidth("text with \x1b[31mANSI\x1b[0m") // 14 (excludes ANSI codes)
countLines("a\nb\nc") // 3
```

## Building a Complete TUI Application

### Basic tea.Model Pattern

```go
type Model struct {
    width, height int
    table         *component.Table
    toast         *component.ToastManager
    theme         *themes.Theme
}

func NewModel() *Model {
    return &Model{
        table: component.NewTable().
            WithColumns(/* ... */).
            WithRows(/* ... */),
        toast: component.NewToastManager(),
        theme: themes.Global().Active(),
    }
}

func (m *Model) Init() tea.Cmd {
    return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd
    
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.table.WithSize(m.width, m.height-4)
        
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        }
    }
    
    // Update child components
    tableModel, cmd := m.table.Update(msg)
    m.table = tableModel.(*component.Table)
    cmds = append(cmds, cmd)
    
    return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
    var b strings.Builder
    
    // Header
    b.WriteString(m.theme.Title.Render("My Application"))
    b.WriteString("\n\n")
    
    // Table
    b.WriteString(m.table.View())
    
    // Toasts (overlay)
    if toasts := m.toast.View(m.width); toasts != "" {
        b.WriteString("\n")
        b.WriteString(toasts)
    }
    
    return b.String()
}

func main() {
    p := tea.NewProgram(NewModel(), tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

## Best Practices

### 1. Always Handle Window Resize

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    // Propagate to child components
    m.table.WithSize(m.width, m.height-headerHeight)
```

### 2. Use Theme Consistently

```go
// Get theme from component's base
theme := m.table.Theme()

// Or use global theme
theme := themes.Global().Active()
```

### 3. Propagate Updates to Child Components

```go
// Update all child models
var cmds []tea.Cmd
m.table, cmd = m.table.Update(msg)
cmds = append(cmds, cmd)
m.modal, cmd = m.modal.Update(msg)
cmds = append(cmds, cmd)
return m, tea.Batch(cmds...)
```

### 4. Use Responsive Layouts

```go
ctx := layout.NewContext(m.width, m.height)
if ctx.IsSmall() {
    // Single column, no sidebar
} else {
    // Full layout with sidebar
    sidebarWidth := ctx.SidebarWidth()
    mainWidth := ctx.MainWidth()
}
```

### 5. Focus Management

```go
// Only one component should have focus at a time
func (m *Model) focusTable() {
    m.table.Focus()
    m.list.Blur()
}

func (m *Model) focusList() {
    m.table.Blur()
    m.list.Focus()
}
```

## Testing Components

Components can be tested by creating them and checking their rendered output:

```go
func TestCard(t *testing.T) {
    card := component.NewCard().
        WithTitle("Test").
        WithContent("Content")
    
    output := card.View(40)
    
    if !strings.Contains(output, "Test") {
        t.Error("Card should contain title")
    }
    if !strings.Contains(output, "Content") {
        t.Error("Card should contain content")
    }
}
```

For stateful components, simulate messages:

```go
func TestTableNavigation(t *testing.T) {
    table := component.NewTable().
        WithRows(
            component.TableRow{Cells: []string{"1"}},
            component.TableRow{Cells: []string{"2"}},
        )
    table.Focus()
    
    // Simulate down key
    table.Update(tea.KeyMsg{Type: tea.KeyDown})
    
    if table.SelectedIndex() != 1 {
        t.Error("Table should select second row")
    }
}
```

---

## Related Documentation

| Document | Topic |
|----------|-------|
| [CLI Reference](../guides/cli-reference.md) | Command-line interface |
| [Configuration](../getting-started/configuration.md) | Configuration options |
| [Developer Guide](developer-guide.md) | Development patterns |
| [Architecture Overview](../concepts/architecture.md) | System architecture |

### External Resources

- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss Documentation](https://github.com/charmbracelet/lipgloss) - Styling library
- [Huh Documentation](https://github.com/charmbracelet/huh) - Form library


