// Package tui provides reusable terminal UI components for bib and bibd.
//
// # Architecture
//
// The TUI system is organized into three main packages:
//
//   - themes: Color palettes, theme presets, and the theme registry
//   - layout: Flex, grid, and responsive layout primitives
//   - component: Reusable UI components (stateless and stateful)
//
// # Usage
//
// Basic usage with the global theme:
//
//	import (
//	    "bib/internal/tui"
//	    "bib/internal/tui/themes"
//	    "bib/internal/tui/component"
//	)
//
//	// Use default theme
//	theme := themes.Global().Active()
//
//	// Create a card
//	card := component.NewCard().
//	    WithTitle("Hello").
//	    WithContent("World").
//	    WithTheme(theme)
//
//	fmt.Println(card.View(80))
//
// # Theme System
//
// Multiple theme presets are available:
//
//	themes.Global().SetActive(themes.PresetDark)    // Dark theme
//	themes.Global().SetActive(themes.PresetLight)   // Light theme
//	themes.Global().SetActive(themes.PresetDracula) // Dracula theme
//	themes.Global().SetActive(themes.PresetNord)    // Nord theme
//	themes.Global().SetActive(themes.PresetGruvbox) // Gruvbox theme
//
// # Components
//
// Stateless components (render functions):
//   - Badge: Inline status tag
//   - Box: Bordered container
//   - Card: Content card with title/footer
//   - Divider: Horizontal line
//   - KeyValue: Key-value pair display
//   - ProgressBar: Progress indicator
//   - StatusMessage: Status with icon
//
// Stateful components (tea.Model):
//   - List: Interactive item list
//   - Modal: Overlay dialog
//   - Spinner: Loading animation
//   - SplitView: Resizable panes
//   - Table: Data table with selection
//   - Tabs: Tab navigation
//   - Toast: Temporary notification
//   - Tree: Hierarchical tree view
//
// # Layout System
//
// Flex layout:
//
//	layout.NewFlex().
//	    Direction(layout.Row).
//	    Gap(2).
//	    Items("Left", "Center", "Right").
//	    Render()
//
// Grid layout:
//
//	layout.NewGrid(3).
//	    Width(80).
//	    Items("A", "B", "C", "D", "E", "F").
//	    Render()
//
// Responsive breakpoints:
//
//	bp := layout.GetBreakpoint(termWidth)
//	if bp >= layout.BreakpointMD {
//	    // Show sidebar
//	}
package tui

import (
	"bib/internal/tui/component"
	"bib/internal/tui/layout"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
)

// Re-export commonly used types for convenience

// Theme is the main theme type
type Theme = themes.Theme

// GetTheme returns the currently active theme
func GetTheme() *Theme {
	return themes.Global().Active()
}

// SetTheme sets the active theme by preset name
func SetTheme(name themes.PresetName) {
	themes.Global().SetActive(name)
}

// AutoDetectTheme sets the theme based on terminal background
func AutoDetectTheme() {
	themes.Global().AutoDetect()
}

// Layout context
type LayoutContext = layout.Context

// NewLayoutContext creates a layout context for the given dimensions
func NewLayoutContext(width, height int) *LayoutContext {
	return layout.NewContext(width, height)
}

// GetBreakpoint returns the breakpoint for a width
func GetBreakpoint(width int) layout.Breakpoint {
	return layout.GetBreakpoint(width)
}

// Component types for convenience
type (
	// Badge is an inline status tag
	Badge = component.Badge

	// Box is a bordered container
	Box = component.Box

	// Card is a content card
	Card = component.Card

	// Divider is a horizontal line
	Divider = component.Divider

	// KeyValue displays a key-value pair
	KeyValue = component.KeyValue

	// KeyValueList displays multiple key-value pairs
	KeyValueList = component.KeyValueList

	// List is an interactive item list
	List = component.List

	// ListItem is an item in a list
	ListItem = component.ListItem

	// Modal is an overlay dialog
	Modal = component.Modal

	// ModalAction is a button in a modal
	ModalAction = component.ModalAction

	// Panel is a section container
	Panel = component.Panel

	// ProgressBar shows progress
	ProgressBar = component.ProgressBar

	// Spinner is a loading indicator
	Spinner = component.Spinner

	// SplitView provides resizable panes
	SplitView = component.SplitView

	// SplitPane is a pane in a split view
	SplitPane = component.SplitPane

	// StatusMessage shows a status with icon
	StatusMessage = component.StatusMessage

	// Table is a data table
	Table = component.Table

	// TableColumn defines a table column
	TableColumn = component.TableColumn

	// TableRow is a row in a table
	TableRow = component.TableRow

	// Tabs provides tab navigation
	Tabs = component.Tabs

	// TabItem is a tab in the tab bar
	TabItem = component.TabItem

	// Toast is a temporary notification
	Toast = component.Toast

	// ToastManager manages multiple toasts
	ToastManager = component.ToastManager

	// Tree is a hierarchical tree view
	Tree = component.Tree

	// TreeNode is a node in the tree
	TreeNode = component.TreeNode

	// Breadcrumb shows navigation path
	Breadcrumb = component.Breadcrumb

	// StepIndicator shows progress steps
	StepIndicator = component.StepIndicator
)

// Constructor functions for convenience

// NewBadge creates a new badge
func NewBadge(text string) *Badge {
	return component.NewBadge(text)
}

// NewBox creates a new box
func NewBox(content string) *Box {
	return component.NewBox(content)
}

// NewCard creates a new card
func NewCard() *Card {
	return component.NewCard()
}

// NewDivider creates a new divider
func NewDivider() *Divider {
	return component.NewDivider()
}

// NewComponentKeyValue creates a new key-value display component
func NewComponentKeyValue(key, value string) *KeyValue {
	return component.NewKeyValue(key, value)
}

// NewKeyValueList creates a new key-value list
func NewKeyValueList() *KeyValueList {
	return component.NewKeyValueList()
}

// NewList creates a new list
func NewList() *List {
	return component.NewList()
}

// NewModal creates a new modal
func NewModal() *Modal {
	return component.NewModal()
}

// ConfirmModal creates a confirmation dialog
func ConfirmModal(title, message string, onConfirm, onCancel func() tea.Cmd) *Modal {
	return component.ConfirmModal(title, message, onConfirm, onCancel)
}

// NewPanel creates a new panel
func NewPanel() *Panel {
	return component.NewPanel()
}

// NewProgressBar creates a new progress bar
func NewProgressBar() *ProgressBar {
	return component.NewProgressBar()
}

// NewSpinner creates a new spinner
func NewSpinner() *Spinner {
	return component.NewSpinner()
}

// NewSplitView creates a new split view
func NewSplitView(direction layout.Direction) *SplitView {
	if direction == layout.Column {
		return component.NewSplitView(component.SplitVertical)
	}
	return component.NewSplitView(component.SplitHorizontal)
}

// NewTable creates a new table
func NewTable() *Table {
	return component.NewTable()
}

// NewTabs creates a new tab component
func NewTabs(items ...TabItem) *Tabs {
	return component.NewTabs(items...)
}

// NewToast creates a new toast
func NewToast(message string, toastType component.ToastType) *Toast {
	return component.NewToast(message, toastType)
}

// NewToastManager creates a new toast manager
func NewToastManager() *ToastManager {
	return component.NewToastManager()
}

// NewTree creates a new tree
func NewTree() *Tree {
	return component.NewTree()
}

// NewBreadcrumb creates a new breadcrumb
func NewBreadcrumb(items ...string) *Breadcrumb {
	return component.NewBreadcrumb(items...)
}

// NewStepIndicator creates a new step indicator
func NewStepIndicator(steps ...string) *StepIndicator {
	return component.NewStepIndicator(steps...)
}

// Status message constructors
func Success(message string) *StatusMessage { return component.Success(message) }
func Error(message string) *StatusMessage   { return component.Error(message) }
func Warning(message string) *StatusMessage { return component.Warning(message) }
func Info(message string) *StatusMessage    { return component.Info(message) }
func Pending(message string) *StatusMessage { return component.Pending(message) }
