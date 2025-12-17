package component

import (
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TreeNode represents a node in the tree
type TreeNode struct {
	ID       string
	Label    string
	Icon     string
	Children []*TreeNode
	Data     interface{}
	Expanded bool
}

// AddChild adds a child node
func (n *TreeNode) AddChild(child *TreeNode) *TreeNode {
	n.Children = append(n.Children, child)
	return n
}

// Expand expands this node
func (n *TreeNode) Expand() *TreeNode {
	n.Expanded = true
	return n
}

// Collapse collapses this node
func (n *TreeNode) Collapse() *TreeNode {
	n.Expanded = false
	return n
}

// IsLeaf returns true if the node has no children
func (n *TreeNode) IsLeaf() bool {
	return len(n.Children) == 0
}

// Tree is an interactive tree view component
type Tree struct {
	BaseComponent
	FocusState
	ScrollState

	root          *TreeNode
	flatNodes     []*flatNode // Flattened visible nodes
	selectedIndex int
	width         int
	height        int
	showRoot      bool
	indent        int
}

type flatNode struct {
	node  *TreeNode
	depth int
}

// NewTree creates a new tree
func NewTree() *Tree {
	return &Tree{
		BaseComponent: NewBaseComponent(),
		showRoot:      true,
		indent:        2,
	}
}

// WithRoot sets the root node
func (t *Tree) WithRoot(root *TreeNode) *Tree {
	t.root = root
	t.rebuild()
	return t
}

// WithSize sets the tree dimensions
func (t *Tree) WithSize(width, height int) *Tree {
	t.width = width
	t.height = height
	t.ScrollState.SetMaxOffset(max(0, len(t.flatNodes)-t.visibleNodes()))
	return t
}

// WithShowRoot shows/hides the root node
func (t *Tree) WithShowRoot(show bool) *Tree {
	t.showRoot = show
	t.rebuild()
	return t
}

// WithIndent sets the indentation per level
func (t *Tree) WithIndent(indent int) *Tree {
	t.indent = indent
	return t
}

// WithTheme sets the theme
func (t *Tree) WithTheme(theme *themes.Theme) *Tree {
	t.SetTheme(theme)
	return t
}

func (t *Tree) rebuild() {
	t.flatNodes = make([]*flatNode, 0)
	if t.root == nil {
		return
	}

	startDepth := 0
	if t.showRoot {
		t.flatNodes = append(t.flatNodes, &flatNode{node: t.root, depth: 0})
		if t.root.Expanded {
			t.flattenChildren(t.root, 1)
		}
	} else {
		startDepth = -1
		for _, child := range t.root.Children {
			t.flatNodes = append(t.flatNodes, &flatNode{node: child, depth: 0})
			if child.Expanded {
				t.flattenChildren(child, 1)
			}
		}
	}
	_ = startDepth

	t.ScrollState.SetMaxOffset(max(0, len(t.flatNodes)-t.visibleNodes()))
}

func (t *Tree) flattenChildren(parent *TreeNode, depth int) {
	for _, child := range parent.Children {
		t.flatNodes = append(t.flatNodes, &flatNode{node: child, depth: depth})
		if child.Expanded && len(child.Children) > 0 {
			t.flattenChildren(child, depth+1)
		}
	}
}

func (t *Tree) visibleNodes() int {
	if t.height <= 0 {
		return len(t.flatNodes)
	}
	return max(1, t.height)
}

// Init implements tea.Model
func (t *Tree) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (t *Tree) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !t.Focused() {
		return t, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if t.selectedIndex > 0 {
				t.selectedIndex--
				if t.selectedIndex < t.Offset() {
					t.SetOffset(t.selectedIndex)
				}
			}

		case "down", "j":
			if t.selectedIndex < len(t.flatNodes)-1 {
				t.selectedIndex++
				visibleEnd := t.Offset() + t.visibleNodes()
				if t.selectedIndex >= visibleEnd {
					t.SetOffset(t.selectedIndex - t.visibleNodes() + 1)
				}
			}

		case "enter", "right", "l":
			if t.selectedIndex >= 0 && t.selectedIndex < len(t.flatNodes) {
				node := t.flatNodes[t.selectedIndex].node
				if !node.IsLeaf() {
					node.Expanded = !node.Expanded
					t.rebuild()
				}
			}

		case "left", "h":
			if t.selectedIndex >= 0 && t.selectedIndex < len(t.flatNodes) {
				node := t.flatNodes[t.selectedIndex].node
				if node.Expanded && !node.IsLeaf() {
					node.Expanded = false
					t.rebuild()
				} else {
					// Move to parent
					t.moveToParent()
				}
			}

		case "o":
			// Expand all children of current node
			if t.selectedIndex >= 0 && t.selectedIndex < len(t.flatNodes) {
				t.expandAll(t.flatNodes[t.selectedIndex].node)
				t.rebuild()
			}

		case "O":
			// Collapse all children of current node
			if t.selectedIndex >= 0 && t.selectedIndex < len(t.flatNodes) {
				t.collapseAll(t.flatNodes[t.selectedIndex].node)
				t.rebuild()
			}

		case "home", "g":
			t.selectedIndex = 0
			t.SetOffset(0)

		case "end", "G":
			t.selectedIndex = len(t.flatNodes) - 1
			t.SetOffset(t.MaxOffset())
		}

	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		t.ScrollState.SetMaxOffset(max(0, len(t.flatNodes)-t.visibleNodes()))
	}

	return t, nil
}

func (t *Tree) moveToParent() {
	if t.selectedIndex <= 0 || t.selectedIndex >= len(t.flatNodes) {
		return
	}

	currentDepth := t.flatNodes[t.selectedIndex].depth
	if currentDepth == 0 {
		return
	}

	// Search backwards for parent
	for i := t.selectedIndex - 1; i >= 0; i-- {
		if t.flatNodes[i].depth < currentDepth {
			t.selectedIndex = i
			if t.selectedIndex < t.Offset() {
				t.SetOffset(t.selectedIndex)
			}
			return
		}
	}
}

func (t *Tree) expandAll(node *TreeNode) {
	node.Expanded = true
	for _, child := range node.Children {
		t.expandAll(child)
	}
}

func (t *Tree) collapseAll(node *TreeNode) {
	node.Expanded = false
	for _, child := range node.Children {
		t.collapseAll(child)
	}
}

// View implements tea.Model
func (t *Tree) View() string {
	return t.ViewWidth(t.width)
}

// ViewWidth renders the tree at a specific width (implements Component)
func (t *Tree) ViewWidth(width int) string {
	if len(t.flatNodes) == 0 {
		return ""
	}

	if width == 0 {
		width = t.width
	}
	if width == 0 {
		width = 60
	}

	theme := t.Theme()
	var lines []string

	startNode := t.Offset()
	endNode := min(startNode+t.visibleNodes(), len(t.flatNodes))

	for i := startNode; i < endNode; i++ {
		fn := t.flatNodes[i]
		node := fn.node

		// Build prefix
		prefix := strings.Repeat(" ", fn.depth*t.indent)

		// Add tree branch characters
		var icon string
		if node.IsLeaf() {
			if node.Icon != "" {
				icon = node.Icon
			} else {
				icon = themes.IconTreeLeaf
			}
		} else if node.Expanded {
			icon = themes.IconTreeExpanded
		} else {
			icon = themes.IconTreeCollapsed
		}

		// Style based on selection
		var lineStyle lipgloss.Style
		var iconStyle lipgloss.Style

		if i == t.selectedIndex {
			lineStyle = theme.Selected
			iconStyle = theme.TreeExpanded
		} else {
			lineStyle = theme.TreeNode
			iconStyle = theme.TreeBranch
		}

		line := prefix + iconStyle.Render(icon) + " " + lineStyle.Render(node.Label)

		// Truncate if needed
		if lipgloss.Width(line) > width {
			line = Truncate(line, width)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// SelectedNode returns the currently selected node
func (t *Tree) SelectedNode() *TreeNode {
	if t.selectedIndex >= 0 && t.selectedIndex < len(t.flatNodes) {
		return t.flatNodes[t.selectedIndex].node
	}
	return nil
}

// SelectedValue returns the data of the selected node
func (t *Tree) SelectedValue() interface{} {
	if node := t.SelectedNode(); node != nil {
		return node.Data
	}
	return nil
}

// SelectedPath returns the path to the selected node
func (t *Tree) SelectedPath() []string {
	if t.selectedIndex < 0 || t.selectedIndex >= len(t.flatNodes) {
		return nil
	}

	// Build path by traversing up
	var path []string
	fn := t.flatNodes[t.selectedIndex]
	path = append(path, fn.node.Label)

	for i := t.selectedIndex - 1; i >= 0; i-- {
		if t.flatNodes[i].depth < fn.depth {
			fn = t.flatNodes[i]
			path = append([]string{fn.node.Label}, path...)
		}
		if fn.depth == 0 {
			break
		}
	}

	return path
}

// ExpandNode expands a node by ID
func (t *Tree) ExpandNode(id string) bool {
	node := t.findNode(t.root, id)
	if node != nil {
		node.Expanded = true
		t.rebuild()
		return true
	}
	return false
}

// CollapseNode collapses a node by ID
func (t *Tree) CollapseNode(id string) bool {
	node := t.findNode(t.root, id)
	if node != nil {
		node.Expanded = false
		t.rebuild()
		return true
	}
	return false
}

func (t *Tree) findNode(root *TreeNode, id string) *TreeNode {
	if root == nil {
		return nil
	}
	if root.ID == id {
		return root
	}
	for _, child := range root.Children {
		if found := t.findNode(child, id); found != nil {
			return found
		}
	}
	return nil
}

// Focus implements FocusableComponent
func (t *Tree) Focus() tea.Cmd {
	t.FocusState.Focus()
	return nil
}

// NodeCount returns the total number of nodes
func (t *Tree) NodeCount() int {
	return t.countNodes(t.root)
}

func (t *Tree) countNodes(node *TreeNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.Children {
		count += t.countNodes(child)
	}
	return count
}

// VisibleNodeCount returns the number of visible nodes
func (t *Tree) VisibleNodeCount() int {
	return len(t.flatNodes)
}
