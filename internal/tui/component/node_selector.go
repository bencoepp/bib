// Package component provides TUI components for bib.
package component

import (
	"fmt"
	"strings"
	"time"

	"bib/internal/discovery"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NodeSelectorItem represents a node in the selector
type NodeSelectorItem struct {
	// Node is the discovered node information
	Node discovery.DiscoveredNode

	// Selected indicates if this node is selected
	Selected bool

	// IsDefault indicates if this is the default node
	IsDefault bool

	// Alias is a user-friendly name for the node
	Alias string

	// Status is the connection status (connected, disconnected, unknown)
	Status string

	// Error holds any error message for this node
	Error string
}

// NodeSelector is an interactive multi-select component for choosing nodes
type NodeSelector struct {
	BaseComponent
	FocusState

	items         []NodeSelectorItem
	cursorIndex   int
	width         int
	height        int
	showLatency   bool
	showMethod    bool
	showNodeInfo  bool
	allowMultiple bool

	// Public network option
	showBibDev     bool
	bibDevSelected bool
	bibDevItem     NodeSelectorItem

	// Custom address entry
	showAddCustom bool
	customAddress string
	inCustomMode  bool

	// Callbacks
	onSelectionChange func([]NodeSelectorItem)
}

// NewNodeSelector creates a new node selector
func NewNodeSelector() *NodeSelector {
	return &NodeSelector{
		BaseComponent: NewBaseComponent(),
		items:         make([]NodeSelectorItem, 0),
		showLatency:   true,
		showMethod:    true,
		showNodeInfo:  true,
		allowMultiple: true,
		showBibDev:    true,
		showAddCustom: true,
		bibDevItem: NodeSelectorItem{
			Node: discovery.DiscoveredNode{
				Address: "bib.dev:4000",
				Method:  discovery.MethodPublic,
			},
			Alias:  "bib.dev (Public Network)",
			Status: "unknown",
		},
	}
}

// WithNodes sets the discovered nodes
func (n *NodeSelector) WithNodes(nodes []discovery.DiscoveredNode) *NodeSelector {
	n.items = make([]NodeSelectorItem, len(nodes))
	for i, node := range nodes {
		n.items[i] = NodeSelectorItem{
			Node:   node,
			Alias:  n.generateAlias(node),
			Status: "unknown",
		}
	}
	return n
}

// WithItems sets the node selector items directly
func (n *NodeSelector) WithItems(items []NodeSelectorItem) *NodeSelector {
	n.items = items
	return n
}

// WithSize sets the selector dimensions
func (n *NodeSelector) WithSize(width, height int) *NodeSelector {
	n.width = width
	n.height = height
	return n
}

// WithMultiSelect enables/disables multiple selection
func (n *NodeSelector) WithMultiSelect(allow bool) *NodeSelector {
	n.allowMultiple = allow
	return n
}

// WithBibDev enables/disables the bib.dev option
func (n *NodeSelector) WithBibDev(show bool) *NodeSelector {
	n.showBibDev = show
	return n
}

// WithAddCustom enables/disables the add custom option
func (n *NodeSelector) WithAddCustom(show bool) *NodeSelector {
	n.showAddCustom = show
	return n
}

// WithLatency enables/disables latency display
func (n *NodeSelector) WithLatency(show bool) *NodeSelector {
	n.showLatency = show
	return n
}

// WithTheme sets the theme
func (n *NodeSelector) WithTheme(theme *themes.Theme) *NodeSelector {
	n.SetTheme(theme)
	return n
}

// OnSelectionChange sets a callback for selection changes
func (n *NodeSelector) OnSelectionChange(fn func([]NodeSelectorItem)) *NodeSelector {
	n.onSelectionChange = fn
	return n
}

// generateAlias generates a user-friendly alias for a node
func (n *NodeSelector) generateAlias(node discovery.DiscoveredNode) string {
	switch node.Method {
	case discovery.MethodLocal:
		return fmt.Sprintf("Local (%s)", node.Address)
	case discovery.MethodMDNS:
		if node.NodeInfo != nil && node.NodeInfo.Name != "" {
			return node.NodeInfo.Name
		}
		return fmt.Sprintf("Network (%s)", node.Address)
	case discovery.MethodP2P:
		if node.NodeInfo != nil && node.NodeInfo.Name != "" {
			return node.NodeInfo.Name
		}
		return fmt.Sprintf("Peer (%s)", node.Address)
	case discovery.MethodPublic:
		return "bib.dev (Public Network)"
	default:
		return node.Address
	}
}

// totalItems returns the total number of items including special options
func (n *NodeSelector) totalItems() int {
	count := len(n.items)
	if n.showBibDev {
		count++
	}
	if n.showAddCustom {
		count++
	}
	return count
}

// getItemAtIndex returns the item at the given index, accounting for special options
func (n *NodeSelector) getItemAtIndex(index int) (item *NodeSelectorItem, isBibDev, isAddCustom bool) {
	nodeCount := len(n.items)

	if index < nodeCount {
		return &n.items[index], false, false
	}

	index -= nodeCount

	if n.showBibDev {
		if index == 0 {
			return &n.bibDevItem, true, false
		}
		index--
	}

	if n.showAddCustom && index == 0 {
		return nil, false, true
	}

	return nil, false, false
}

// Init implements tea.Model
func (n *NodeSelector) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (n *NodeSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if n.inCustomMode {
			return n.updateCustomMode(msg)
		}

		switch msg.String() {
		case "up", "k":
			if n.cursorIndex > 0 {
				n.cursorIndex--
			}
		case "down", "j":
			if n.cursorIndex < n.totalItems()-1 {
				n.cursorIndex++
			}
		case " ", "x":
			// Toggle selection
			n.toggleCurrentItem()
		case "enter":
			// Confirm selection or enter custom mode
			item, _, isAddCustom := n.getItemAtIndex(n.cursorIndex)
			if isAddCustom {
				n.inCustomMode = true
				n.customAddress = ""
				return n, nil
			}
			if item != nil {
				n.toggleCurrentItem()
			}
		case "a":
			// Select all local nodes
			n.selectAllLocal()
		case "n":
			// Deselect all
			n.deselectAll()
		case "d":
			// Set current as default
			n.setCurrentAsDefault()
		}
	}

	return n, nil
}

// updateCustomMode handles key input in custom address mode
func (n *NodeSelector) updateCustomMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Add custom address
		if n.customAddress != "" {
			n.addCustomNode(n.customAddress)
		}
		n.inCustomMode = false
	case "esc":
		n.inCustomMode = false
	case "backspace":
		if len(n.customAddress) > 0 {
			n.customAddress = n.customAddress[:len(n.customAddress)-1]
		}
	default:
		if len(msg.String()) == 1 {
			n.customAddress += msg.String()
		}
	}
	return n, nil
}

// toggleCurrentItem toggles the selection of the current item
func (n *NodeSelector) toggleCurrentItem() {
	item, isBibDev, isAddCustom := n.getItemAtIndex(n.cursorIndex)
	if isAddCustom {
		return
	}

	if isBibDev {
		n.bibDevSelected = !n.bibDevSelected
		n.bibDevItem.Selected = n.bibDevSelected
	} else if item != nil {
		if n.allowMultiple {
			item.Selected = !item.Selected
		} else {
			// Single select mode - deselect all others
			for i := range n.items {
				n.items[i].Selected = false
			}
			n.bibDevSelected = false
			n.bibDevItem.Selected = false
			item.Selected = true
		}
	}

	n.notifySelectionChange()
}

// selectAllLocal selects all local nodes
func (n *NodeSelector) selectAllLocal() {
	for i := range n.items {
		if n.items[i].Node.Method == discovery.MethodLocal {
			n.items[i].Selected = true
		}
	}
	n.notifySelectionChange()
}

// deselectAll deselects all nodes
func (n *NodeSelector) deselectAll() {
	for i := range n.items {
		n.items[i].Selected = false
	}
	n.bibDevSelected = false
	n.bibDevItem.Selected = false
	n.notifySelectionChange()
}

// setCurrentAsDefault sets the current item as the default
func (n *NodeSelector) setCurrentAsDefault() {
	// Clear existing default
	for i := range n.items {
		n.items[i].IsDefault = false
	}
	n.bibDevItem.IsDefault = false

	item, isBibDev, isAddCustom := n.getItemAtIndex(n.cursorIndex)
	if isAddCustom {
		return
	}

	if isBibDev {
		n.bibDevItem.IsDefault = true
		n.bibDevItem.Selected = true
		n.bibDevSelected = true
	} else if item != nil {
		item.IsDefault = true
		item.Selected = true
	}

	n.notifySelectionChange()
}

// addCustomNode adds a custom node address
func (n *NodeSelector) addCustomNode(address string) {
	// Normalize address
	if !strings.Contains(address, ":") {
		address += ":4000"
	}

	// Check if already exists
	for _, item := range n.items {
		if item.Node.Address == address {
			return
		}
	}

	n.items = append(n.items, NodeSelectorItem{
		Node: discovery.DiscoveredNode{
			Address:      address,
			Method:       discovery.MethodManual,
			DiscoveredAt: time.Now(),
		},
		Alias:    address,
		Selected: true,
		Status:   "unknown",
	})

	n.notifySelectionChange()
}

// notifySelectionChange calls the selection change callback
func (n *NodeSelector) notifySelectionChange() {
	if n.onSelectionChange != nil {
		n.onSelectionChange(n.SelectedItems())
	}
}

// SelectedItems returns all selected items
func (n *NodeSelector) SelectedItems() []NodeSelectorItem {
	var selected []NodeSelectorItem
	for _, item := range n.items {
		if item.Selected {
			selected = append(selected, item)
		}
	}
	if n.bibDevSelected {
		selected = append(selected, n.bibDevItem)
	}
	return selected
}

// SelectedNodes returns just the selected nodes
func (n *NodeSelector) SelectedNodes() []discovery.DiscoveredNode {
	var nodes []discovery.DiscoveredNode
	for _, item := range n.items {
		if item.Selected {
			nodes = append(nodes, item.Node)
		}
	}
	if n.bibDevSelected {
		nodes = append(nodes, n.bibDevItem.Node)
	}
	return nodes
}

// IsBibDevSelected returns whether bib.dev is selected
func (n *NodeSelector) IsBibDevSelected() bool {
	return n.bibDevSelected
}

// SetBibDevSelected sets the bib.dev selection
func (n *NodeSelector) SetBibDevSelected(selected bool) {
	n.bibDevSelected = selected
	n.bibDevItem.Selected = selected
}

// GetDefaultNode returns the default node, or nil if none is set
func (n *NodeSelector) GetDefaultNode() *NodeSelectorItem {
	for i := range n.items {
		if n.items[i].IsDefault {
			return &n.items[i]
		}
	}
	if n.bibDevItem.IsDefault {
		return &n.bibDevItem
	}
	// Return first selected if no default
	for i := range n.items {
		if n.items[i].Selected {
			return &n.items[i]
		}
	}
	if n.bibDevSelected {
		return &n.bibDevItem
	}
	return nil
}

// View implements tea.Model
func (n *NodeSelector) View() string {
	return n.ViewWidth(n.width)
}

// ViewWidth renders the component at the given width
func (n *NodeSelector) ViewWidth(width int) string {
	if width == 0 {
		width = 60
	}

	theme := n.Theme()
	if theme == nil {
		theme = themes.Global().Active()
	}

	var sb strings.Builder

	// Header
	selectedCount := len(n.SelectedItems())
	header := fmt.Sprintf("Select Nodes (%d selected)", selectedCount)
	sb.WriteString(theme.SectionTitle.Render(header))
	sb.WriteString("\n\n")

	// Custom mode input
	if n.inCustomMode {
		sb.WriteString("Enter address: ")
		sb.WriteString(n.customAddress)
		sb.WriteString("█\n")
		sb.WriteString(theme.Description.Render("Press Enter to add, Esc to cancel"))
		sb.WriteString("\n\n")
	}

	// Render each item
	for i := 0; i < n.totalItems(); i++ {
		item, isBibDev, isAddCustom := n.getItemAtIndex(i)
		isCursor := i == n.cursorIndex

		line := n.renderItem(item, isBibDev, isAddCustom, isCursor, width, theme)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Help text
	sb.WriteString("\n")
	helpStyle := theme.HelpDesc
	if n.allowMultiple {
		sb.WriteString(helpStyle.Render("↑/↓ navigate • Space toggle • a select local • n clear • d set default • Enter confirm"))
	} else {
		sb.WriteString(helpStyle.Render("↑/↓ navigate • Enter select"))
	}

	return sb.String()
}

// renderItem renders a single item
func (n *NodeSelector) renderItem(item *NodeSelectorItem, isBibDev, isAddCustom, isCursor bool, width int, theme *themes.Theme) string {
	var sb strings.Builder

	// Cursor indicator
	if isCursor {
		sb.WriteString("❯ ")
	} else {
		sb.WriteString("  ")
	}

	// Handle special items
	if isAddCustom {
		icon := "+"
		text := "Add custom address..."
		style := theme.Disabled
		if isCursor {
			style = theme.Focused
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s %s", icon, text)))
		return sb.String()
	}

	if item == nil {
		return sb.String()
	}

	// Checkbox
	if item.Selected {
		sb.WriteString("[✓] ")
	} else {
		sb.WriteString("[ ] ")
	}

	// Default indicator
	if item.IsDefault {
		sb.WriteString("★ ")
	}

	// Method icon
	methodIcon := n.getMethodIcon(item.Node.Method)
	sb.WriteString(methodIcon)
	sb.WriteString(" ")

	// Main content
	var contentStyle lipgloss.Style
	if isCursor {
		contentStyle = theme.Focused
	} else if item.Selected {
		contentStyle = theme.Success
	} else {
		contentStyle = theme.Base
	}

	// Alias or address
	alias := item.Alias
	if alias == "" {
		alias = item.Node.Address
	}
	sb.WriteString(contentStyle.Render(alias))

	// Latency
	if n.showLatency && item.Node.Latency > 0 {
		latencyStr := fmt.Sprintf(" (%s)", item.Node.Latency.Round(time.Millisecond))
		sb.WriteString(theme.Description.Render(latencyStr))
	}

	// Error
	if item.Error != "" {
		sb.WriteString(theme.Error.Render(fmt.Sprintf(" ⚠ %s", item.Error)))
	}

	// bib.dev warning
	if isBibDev && item.Selected {
		sb.WriteString("\n      ")
		sb.WriteString(theme.Warning.Render("⚠ Connects to public network"))
	}

	return sb.String()
}

// getMethodIcon returns an icon for the discovery method
func (n *NodeSelector) getMethodIcon(method discovery.DiscoveryMethod) string {
	switch method {
	case discovery.MethodLocal:
		return "🏠"
	case discovery.MethodMDNS:
		return "📡"
	case discovery.MethodP2P:
		return "🌐"
	case discovery.MethodPublic:
		return "☁️"
	case discovery.MethodManual:
		return "✎"
	default:
		return "•"
	}
}

// HasSelection returns true if at least one node is selected
func (n *NodeSelector) HasSelection() bool {
	for _, item := range n.items {
		if item.Selected {
			return true
		}
	}
	return n.bibDevSelected
}

// SelectFirst selects the first node if nothing is selected
func (n *NodeSelector) SelectFirst() {
	if n.HasSelection() {
		return
	}
	if len(n.items) > 0 {
		n.items[0].Selected = true
		n.items[0].IsDefault = true
	}
}
