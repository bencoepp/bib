package component

import (
	"testing"
	"time"

	"bib/internal/discovery"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewNodeSelector(t *testing.T) {
	ns := NewNodeSelector()

	if ns == nil {
		t.Fatal("node selector is nil")
	}

	if !ns.showLatency {
		t.Error("expected showLatency to be true by default")
	}

	if !ns.allowMultiple {
		t.Error("expected allowMultiple to be true by default")
	}

	if !ns.showBibDev {
		t.Error("expected showBibDev to be true by default")
	}

	if !ns.showAddCustom {
		t.Error("expected showAddCustom to be true by default")
	}
}

func TestNodeSelectorWithNodes(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal, Latency: 5 * time.Millisecond},
		{Address: "192.168.1.50:4000", Method: discovery.MethodMDNS, Latency: 10 * time.Millisecond},
	}

	ns := NewNodeSelector().WithNodes(nodes)

	if len(ns.items) != 2 {
		t.Errorf("expected 2 items, got %d", len(ns.items))
	}

	if ns.items[0].Node.Address != "localhost:4000" {
		t.Error("first item address mismatch")
	}

	if ns.items[0].Alias == "" {
		t.Error("alias should be generated")
	}
}

func TestNodeSelectorToggleSelection(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal},
	}

	ns := NewNodeSelector().WithNodes(nodes)

	// Initially not selected
	if ns.items[0].Selected {
		t.Error("expected item to not be selected initially")
	}

	// Toggle selection
	ns.toggleCurrentItem()

	if !ns.items[0].Selected {
		t.Error("expected item to be selected after toggle")
	}

	// Toggle again
	ns.toggleCurrentItem()

	if ns.items[0].Selected {
		t.Error("expected item to not be selected after second toggle")
	}
}

func TestNodeSelectorSingleSelect(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal},
		{Address: "localhost:8080", Method: discovery.MethodLocal},
	}

	ns := NewNodeSelector().WithNodes(nodes).WithMultiSelect(false)

	// Select first item
	ns.cursorIndex = 0
	ns.toggleCurrentItem()

	if !ns.items[0].Selected {
		t.Error("expected first item to be selected")
	}

	// Select second item (should deselect first)
	ns.cursorIndex = 1
	ns.toggleCurrentItem()

	if ns.items[0].Selected {
		t.Error("expected first item to be deselected in single select mode")
	}

	if !ns.items[1].Selected {
		t.Error("expected second item to be selected")
	}
}

func TestNodeSelectorSelectedItems(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal},
		{Address: "localhost:8080", Method: discovery.MethodLocal},
		{Address: "192.168.1.50:4000", Method: discovery.MethodMDNS},
	}

	ns := NewNodeSelector().WithNodes(nodes)

	// Select first and third
	ns.items[0].Selected = true
	ns.items[2].Selected = true

	selected := ns.SelectedItems()

	if len(selected) != 2 {
		t.Errorf("expected 2 selected items, got %d", len(selected))
	}
}

func TestNodeSelectorBibDev(t *testing.T) {
	ns := NewNodeSelector()

	if ns.IsBibDevSelected() {
		t.Error("bib.dev should not be selected initially")
	}

	ns.SetBibDevSelected(true)

	if !ns.IsBibDevSelected() {
		t.Error("bib.dev should be selected")
	}

	selected := ns.SelectedItems()
	found := false
	for _, item := range selected {
		if item.Node.Method == discovery.MethodPublic {
			found = true
			break
		}
	}

	if !found {
		t.Error("bib.dev should be in selected items")
	}
}

func TestNodeSelectorSelectAllLocal(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal},
		{Address: "localhost:8080", Method: discovery.MethodLocal},
		{Address: "192.168.1.50:4000", Method: discovery.MethodMDNS},
	}

	ns := NewNodeSelector().WithNodes(nodes)
	ns.selectAllLocal()

	if !ns.items[0].Selected {
		t.Error("first local node should be selected")
	}

	if !ns.items[1].Selected {
		t.Error("second local node should be selected")
	}

	if ns.items[2].Selected {
		t.Error("mDNS node should not be selected by selectAllLocal")
	}
}

func TestNodeSelectorDeselectAll(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal},
		{Address: "localhost:8080", Method: discovery.MethodLocal},
	}

	ns := NewNodeSelector().WithNodes(nodes)

	// Select all
	ns.items[0].Selected = true
	ns.items[1].Selected = true
	ns.bibDevSelected = true

	ns.deselectAll()

	if ns.items[0].Selected || ns.items[1].Selected {
		t.Error("all items should be deselected")
	}

	if ns.bibDevSelected {
		t.Error("bib.dev should be deselected")
	}
}

func TestNodeSelectorSetDefault(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal},
		{Address: "localhost:8080", Method: discovery.MethodLocal},
	}

	ns := NewNodeSelector().WithNodes(nodes)

	// Set second as default
	ns.cursorIndex = 1
	ns.setCurrentAsDefault()

	if ns.items[0].IsDefault {
		t.Error("first item should not be default")
	}

	if !ns.items[1].IsDefault {
		t.Error("second item should be default")
	}

	if !ns.items[1].Selected {
		t.Error("default item should also be selected")
	}
}

func TestNodeSelectorGetDefaultNode(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal},
		{Address: "localhost:8080", Method: discovery.MethodLocal},
	}

	ns := NewNodeSelector().WithNodes(nodes)

	// No default set
	def := ns.GetDefaultNode()
	if def != nil {
		t.Error("expected nil when no default or selection")
	}

	// Set default
	ns.items[1].IsDefault = true
	def = ns.GetDefaultNode()

	if def == nil {
		t.Fatal("expected default node")
	}

	if def.Node.Address != "localhost:8080" {
		t.Error("wrong default node returned")
	}
}

func TestNodeSelectorAddCustomNode(t *testing.T) {
	ns := NewNodeSelector()

	initialCount := len(ns.items)

	ns.addCustomNode("custom.example.com:4000")

	if len(ns.items) != initialCount+1 {
		t.Errorf("expected %d items, got %d", initialCount+1, len(ns.items))
	}

	lastItem := ns.items[len(ns.items)-1]
	if lastItem.Node.Address != "custom.example.com:4000" {
		t.Error("custom node address mismatch")
	}

	if lastItem.Node.Method != discovery.MethodManual {
		t.Error("custom node should have manual method")
	}

	if !lastItem.Selected {
		t.Error("custom node should be selected")
	}
}

func TestNodeSelectorAddCustomNodeWithoutPort(t *testing.T) {
	ns := NewNodeSelector()

	ns.addCustomNode("custom.example.com")

	lastItem := ns.items[len(ns.items)-1]
	if lastItem.Node.Address != "custom.example.com:4000" {
		t.Errorf("expected port 4000 to be added, got %s", lastItem.Node.Address)
	}
}

func TestNodeSelectorAddDuplicateCustomNode(t *testing.T) {
	ns := NewNodeSelector()

	ns.addCustomNode("custom.example.com:4000")
	initialCount := len(ns.items)

	ns.addCustomNode("custom.example.com:4000")

	if len(ns.items) != initialCount {
		t.Error("duplicate node should not be added")
	}
}

func TestNodeSelectorTotalItems(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal},
	}

	ns := NewNodeSelector().WithNodes(nodes)

	// 1 node + bib.dev + add custom = 3
	if ns.totalItems() != 3 {
		t.Errorf("expected 3 total items, got %d", ns.totalItems())
	}

	ns.WithBibDev(false)
	// 1 node + add custom = 2
	if ns.totalItems() != 2 {
		t.Errorf("expected 2 total items, got %d", ns.totalItems())
	}

	ns.WithAddCustom(false)
	// 1 node = 1
	if ns.totalItems() != 1 {
		t.Errorf("expected 1 total item, got %d", ns.totalItems())
	}
}

func TestNodeSelectorNavigation(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal},
		{Address: "localhost:8080", Method: discovery.MethodLocal},
	}

	ns := NewNodeSelector().WithNodes(nodes)

	// Initial position
	if ns.cursorIndex != 0 {
		t.Error("cursor should start at 0")
	}

	// Move down
	ns.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if ns.cursorIndex != 1 {
		t.Error("cursor should move down")
	}

	// Move down again (to bib.dev)
	ns.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if ns.cursorIndex != 2 {
		t.Error("cursor should move to bib.dev")
	}

	// Move up
	ns.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if ns.cursorIndex != 1 {
		t.Error("cursor should move up")
	}
}

func TestNodeSelectorView(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal, Latency: 5 * time.Millisecond},
	}

	ns := NewNodeSelector().WithNodes(nodes).WithSize(60, 20)

	view := ns.View()

	if view == "" {
		t.Error("view should not be empty")
	}

	if !containsStr(view, "Select Nodes") {
		t.Error("view should contain header")
	}

	if !containsStr(view, "localhost:4000") {
		t.Error("view should contain node address")
	}
}

func TestNodeSelectorHasSelection(t *testing.T) {
	ns := NewNodeSelector()

	if ns.HasSelection() {
		t.Error("should not have selection initially")
	}

	ns.addCustomNode("test:4000")

	if !ns.HasSelection() {
		t.Error("should have selection after adding node")
	}
}

func TestNodeSelectorSelectFirst(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal},
	}

	ns := NewNodeSelector().WithNodes(nodes)

	if ns.HasSelection() {
		t.Error("should not have selection initially")
	}

	ns.SelectFirst()

	if !ns.HasSelection() {
		t.Error("should have selection after SelectFirst")
	}

	if !ns.items[0].Selected {
		t.Error("first item should be selected")
	}

	if !ns.items[0].IsDefault {
		t.Error("first item should be default")
	}
}

func TestNodeSelectorOnSelectionChange(t *testing.T) {
	nodes := []discovery.DiscoveredNode{
		{Address: "localhost:4000", Method: discovery.MethodLocal},
	}

	callbackCalled := false
	var callbackItems []NodeSelectorItem

	ns := NewNodeSelector().
		WithNodes(nodes).
		OnSelectionChange(func(items []NodeSelectorItem) {
			callbackCalled = true
			callbackItems = items
		})

	ns.toggleCurrentItem()

	if !callbackCalled {
		t.Error("callback should be called")
	}

	if len(callbackItems) != 1 {
		t.Errorf("expected 1 item in callback, got %d", len(callbackItems))
	}
}

func TestNodeSelectorGenerateAlias(t *testing.T) {
	ns := NewNodeSelector()

	tests := []struct {
		node     discovery.DiscoveredNode
		expected string
	}{
		{
			node:     discovery.DiscoveredNode{Address: "localhost:4000", Method: discovery.MethodLocal},
			expected: "Local (localhost:4000)",
		},
		{
			node:     discovery.DiscoveredNode{Address: "192.168.1.50:4000", Method: discovery.MethodMDNS},
			expected: "Network (192.168.1.50:4000)",
		},
		{
			node: discovery.DiscoveredNode{
				Address: "192.168.1.50:4000",
				Method:  discovery.MethodMDNS,
				NodeInfo: &discovery.NodeInfo{
					Name: "My Node",
				},
			},
			expected: "My Node",
		},
		{
			node:     discovery.DiscoveredNode{Address: "bib.dev:4000", Method: discovery.MethodPublic},
			expected: "bib.dev (Public Network)",
		},
	}

	for _, tt := range tests {
		alias := ns.generateAlias(tt.node)
		if alias != tt.expected {
			t.Errorf("for %s expected alias %q, got %q", tt.node.Address, tt.expected, alias)
		}
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
