// Package tree provides a hierarchical navigation component shared across
// Runtime applications. It renders document structures (JSON/YAML/TOML/XML and
// similar trees) with expand/collapse state, keyboard navigation, and selection
// tracking on top of the shared theme styles.
package tree

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/runtime-sh/runtime/packages/theme"
)

// TreeNode is a single node in a hierarchical structure. A node may carry an
// arbitrary payload in Data and any number of Children, supporting trees of
// arbitrary depth.
type TreeNode struct {
	Key      string
	Title    string
	Children []*TreeNode
	Data     interface{}
}

// IsLeaf reports whether the node has no children.
func (n *TreeNode) IsLeaf() bool {
	return n == nil || len(n.Children) == 0
}

// Tree is a navigable, collapsible view over a TreeNode hierarchy. The zero
// value is not ready for use; construct a Tree with New.
type Tree struct {
	root     *TreeNode
	selected *TreeNode
	expanded map[string]bool
	style    theme.Styles
}

// New creates a Tree that renders with the supplied styles.
func New(style theme.Styles) *Tree {
	return &Tree{
		expanded: make(map[string]bool),
		style:    style,
	}
}

// SetStyle replaces the styles used for rendering. It is safe to call at any
// time, for example after a live theme change.
func (t *Tree) SetStyle(style theme.Styles) {
	t.style = style
}

// SetRoot installs root as the top of the tree. The root is expanded by
// default and selection moves to the root (or to its first visible descendant
// when the root has no key). Passing nil clears the tree.
func (t *Tree) SetRoot(root *TreeNode) {
	if t.expanded == nil {
		t.expanded = make(map[string]bool)
	}
	t.root = root
	t.selected = root
	if root != nil {
		t.expanded[root.Key] = true
	}
}

// Root returns the current root node, or nil when the tree is empty.
func (t *Tree) Root() *TreeNode {
	return t.root
}

// Selected returns the currently selected node, or nil when nothing is
// selected (for example, when the tree is empty).
func (t *Tree) Selected() *TreeNode {
	return t.selected
}

// IsExpanded reports whether node is currently expanded. Leaf nodes are never
// considered expanded.
func (t *Tree) IsExpanded(node *TreeNode) bool {
	if node == nil || node.IsLeaf() {
		return false
	}
	return t.expanded[node.Key]
}

// Expand marks node as expanded so its children become visible. Expanding a
// leaf node or nil has no effect.
func (t *Tree) Expand(node *TreeNode) {
	if node == nil || node.IsLeaf() {
		return
	}
	t.expanded[node.Key] = true
}

// Collapse marks node as collapsed so its children are hidden. If the current
// selection is a descendant of the collapsed node, selection moves up to node
// so it never points at a hidden node.
func (t *Tree) Collapse(node *TreeNode) {
	if node == nil || node.IsLeaf() {
		return
	}
	t.expanded[node.Key] = false
	if t.selected != nil && t.selected != node && isDescendant(node, t.selected) {
		t.selected = node
	}
}

// Toggle flips the expand/collapse state of node.
func (t *Tree) Toggle(node *TreeNode) {
	if node == nil || node.IsLeaf() {
		return
	}
	if t.IsExpanded(node) {
		t.Collapse(node)
	} else {
		t.Expand(node)
	}
}

// Navigate moves the selection to the node with the given key, searching the
// entire tree regardless of expand/collapse state. Ancestors of the target are
// expanded so the newly selected node is visible. Navigate is a no-op when no
// node matches key.
func (t *Tree) Navigate(key string) {
	if t.root == nil {
		return
	}
	path := findPath(t.root, key)
	if path == nil {
		return
	}
	// Expand every ancestor (all but the matched node itself) so the target is
	// visible in the flattened view.
	for i := 0; i < len(path)-1; i++ {
		t.expanded[path[i].Key] = true
	}
	t.selected = path[len(path)-1]
}

// visibleNodes returns the nodes in top-to-bottom display order: the root
// followed by the children of every expanded node, recursively.
func (t *Tree) visibleNodes() []*TreeNode {
	if t.root == nil {
		return nil
	}
	var out []*TreeNode
	var walk func(n *TreeNode)
	walk = func(n *TreeNode) {
		out = append(out, n)
		if t.expanded[n.Key] {
			for _, c := range n.Children {
				walk(c)
			}
		}
	}
	walk(t.root)
	return out
}

// MoveUp selects the previous node in display order.
func (t *Tree) MoveUp() {
	t.moveBy(-1)
}

// MoveDown selects the next node in display order.
func (t *Tree) MoveDown() {
	t.moveBy(1)
}

func (t *Tree) moveBy(delta int) {
	nodes := t.visibleNodes()
	if len(nodes) == 0 {
		return
	}
	idx := indexOf(nodes, t.selected)
	if idx < 0 {
		t.selected = nodes[0]
		return
	}
	next := idx + delta
	if next < 0 || next >= len(nodes) {
		return
	}
	t.selected = nodes[next]
}

// Update handles keyboard navigation for the tree:
//
//	up / k        move selection up
//	down / j      move selection down
//	right / l     expand the selected node
//	left / h      collapse the selected node (or move to parent when a leaf)
//	enter / space toggle the selected node
func (t *Tree) Update(msg tea.Msg) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return
	}
	switch key.String() {
	case "up", "k":
		t.MoveUp()
	case "down", "j":
		t.MoveDown()
	case "right", "l":
		t.Expand(t.selected)
	case "left", "h":
		if t.selected != nil && !t.selected.IsLeaf() && t.IsExpanded(t.selected) {
			t.Collapse(t.selected)
		} else if parent := t.parentOf(t.selected); parent != nil {
			t.selected = parent
		}
	case "enter", " ":
		t.Toggle(t.selected)
	}
}

func (t *Tree) parentOf(node *TreeNode) *TreeNode {
	if t.root == nil || node == nil || node == t.root {
		return nil
	}
	path := findPath(t.root, node.Key)
	if len(path) < 2 {
		return nil
	}
	return path[len(path)-2]
}

// View renders the visible portion of the tree using the configured styles.
// Each level is indented; expandable nodes show a disclosure indicator and the
// selected node is highlighted.
func (t *Tree) View() string {
	if t.root == nil {
		return ""
	}
	var b strings.Builder
	var walk func(n *TreeNode, depth int)
	walk = func(n *TreeNode, depth int) {
		b.WriteString(t.renderNode(n, depth))
		b.WriteByte('\n')
		if t.expanded[n.Key] {
			for _, c := range n.Children {
				walk(c, depth+1)
			}
		}
	}
	walk(t.root, 0)
	return strings.TrimRight(b.String(), "\n")
}

func (t *Tree) renderNode(n *TreeNode, depth int) string {
	indent := strings.Repeat("  ", depth)

	var indicator string
	switch {
	case n.IsLeaf():
		indicator = "  "
	case t.expanded[n.Key]:
		indicator = "▼ "
	default:
		indicator = "▶ "
	}

	label := n.Title
	if label == "" {
		label = n.Key
	}

	line := indent + indicator + label
	if n == t.selected {
		return t.style.Selected.Render(line)
	}
	return t.style.Body.Render(line)
}

// --- helpers ---

func indexOf(nodes []*TreeNode, target *TreeNode) int {
	for i, n := range nodes {
		if n == target {
			return i
		}
	}
	return -1
}

// findPath returns the chain of nodes from root to the node matching key
// (inclusive), or nil when no node matches.
func findPath(root *TreeNode, key string) []*TreeNode {
	if root == nil {
		return nil
	}
	if root.Key == key {
		return []*TreeNode{root}
	}
	for _, c := range root.Children {
		if sub := findPath(c, key); sub != nil {
			return append([]*TreeNode{root}, sub...)
		}
	}
	return nil
}

// isDescendant reports whether target is anywhere beneath ancestor.
func isDescendant(ancestor, target *TreeNode) bool {
	if ancestor == nil || target == nil {
		return false
	}
	for _, c := range ancestor.Children {
		if c == target || isDescendant(c, target) {
			return true
		}
	}
	return false
}
