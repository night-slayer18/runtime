package tree

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/runtime-sh/runtime/packages/theme"
)

// buildSampleTree returns a small multi-level tree:
//
//	root
//	├── a
//	│   ├── a1
//	│   └── a2
//	└── b
//	    └── b1
func buildSampleTree() *TreeNode {
	return &TreeNode{
		Key:   "root",
		Title: "Root",
		Children: []*TreeNode{
			{
				Key:   "a",
				Title: "A",
				Children: []*TreeNode{
					{Key: "a1", Title: "A1"},
					{Key: "a2", Title: "A2"},
				},
			},
			{
				Key:   "b",
				Title: "B",
				Children: []*TreeNode{
					{Key: "b1", Title: "B1"},
				},
			},
		},
	}
}

// buildDeepTree returns a single chain of the given depth: n0 -> n1 -> ... .
// Every node except the last has exactly one child.
func buildDeepTree(depth int) (*TreeNode, []string) {
	keys := make([]string, depth)
	nodes := make([]*TreeNode, depth)
	for i := 0; i < depth; i++ {
		keys[i] = "n" + string(rune('0'+i%10)) + "_" + itoa(i)
		nodes[i] = &TreeNode{Key: keys[i], Title: keys[i]}
	}
	for i := 0; i < depth-1; i++ {
		nodes[i].Children = []*TreeNode{nodes[i+1]}
	}
	return nodes[0], keys
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func newTestTree() *Tree {
	return New(theme.DefaultStyles)
}

func keysOf(nodes []*TreeNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Key
	}
	return out
}

func TestSetRootSelectsRootAndExpands(t *testing.T) {
	tr := newTestTree()
	root := buildSampleTree()
	tr.SetRoot(root)

	if tr.Selected() != root {
		t.Fatalf("expected root selected after SetRoot, got %v", tr.Selected())
	}
	if !tr.IsExpanded(root) {
		t.Fatalf("expected root expanded after SetRoot")
	}
}

func TestSetRootNilClearsSelection(t *testing.T) {
	tr := newTestTree()
	tr.SetRoot(buildSampleTree())
	tr.SetRoot(nil)
	if tr.Selected() != nil {
		t.Fatalf("expected nil selection after clearing tree, got %v", tr.Selected())
	}
	if tr.Root() != nil {
		t.Fatalf("expected nil root after clearing tree")
	}
}

func TestNavigationAcrossCollapsedNodes(t *testing.T) {
	tr := newTestTree()
	root := buildSampleTree()
	tr.SetRoot(root)

	// Only the root is expanded; its children a and b are visible but their
	// grandchildren are not. Visible order: root, a, b.
	visible := keysOf(tr.visibleNodes())
	want := []string{"root", "a", "b"}
	if strings.Join(visible, ",") != strings.Join(want, ",") {
		t.Fatalf("collapsed visible nodes = %v, want %v", visible, want)
	}

	// Moving down should step root -> a -> b and skip the hidden grandchildren.
	tr.MoveDown()
	if got := tr.Selected().Key; got != "a" {
		t.Fatalf("after first MoveDown selected = %q, want a", got)
	}
	tr.MoveDown()
	if got := tr.Selected().Key; got != "b" {
		t.Fatalf("after second MoveDown selected = %q, want b", got)
	}
	// At the end of the visible list MoveDown is a no-op.
	tr.MoveDown()
	if got := tr.Selected().Key; got != "b" {
		t.Fatalf("MoveDown past end changed selection to %q, want b", got)
	}
}

func TestNavigationAcrossExpandedNodes(t *testing.T) {
	tr := newTestTree()
	root := buildSampleTree()
	tr.SetRoot(root)

	// Expand both children so all grandchildren become visible.
	tr.Expand(root.Children[0]) // a
	tr.Expand(root.Children[1]) // b

	visible := keysOf(tr.visibleNodes())
	want := []string{"root", "a", "a1", "a2", "b", "b1"}
	if strings.Join(visible, ",") != strings.Join(want, ",") {
		t.Fatalf("expanded visible nodes = %v, want %v", visible, want)
	}

	// Walk top to bottom and confirm order.
	for _, key := range want[1:] {
		tr.MoveDown()
		if got := tr.Selected().Key; got != key {
			t.Fatalf("MoveDown selected = %q, want %q", got, key)
		}
	}

	// Walk back up to the root.
	for i := len(want) - 2; i >= 0; i-- {
		tr.MoveUp()
		if got := tr.Selected().Key; got != want[i] {
			t.Fatalf("MoveUp selected = %q, want %q", got, want[i])
		}
	}
	// MoveUp past the top is a no-op.
	tr.MoveUp()
	if got := tr.Selected().Key; got != "root" {
		t.Fatalf("MoveUp past top changed selection to %q, want root", got)
	}
}

func TestToggleExpandCollapse(t *testing.T) {
	tr := newTestTree()
	root := buildSampleTree()
	tr.SetRoot(root)
	a := root.Children[0]

	if tr.IsExpanded(a) {
		t.Fatalf("child a should start collapsed")
	}
	tr.Toggle(a)
	if !tr.IsExpanded(a) {
		t.Fatalf("Toggle should expand collapsed node a")
	}
	tr.Toggle(a)
	if tr.IsExpanded(a) {
		t.Fatalf("Toggle should collapse expanded node a")
	}
}

func TestLeafNeverExpands(t *testing.T) {
	tr := newTestTree()
	root := buildSampleTree()
	tr.SetRoot(root)
	leaf := root.Children[0].Children[0] // a1

	tr.Expand(leaf)
	if tr.IsExpanded(leaf) {
		t.Fatalf("leaf node should never report expanded")
	}
}

func TestCollapsePullsSelectionUp(t *testing.T) {
	tr := newTestTree()
	root := buildSampleTree()
	tr.SetRoot(root)
	a := root.Children[0]

	// Select a descendant of a.
	tr.Expand(a)
	tr.Navigate("a2")
	if tr.Selected().Key != "a2" {
		t.Fatalf("setup: expected a2 selected, got %q", tr.Selected().Key)
	}

	// Collapsing a hides a2, so selection should move up to a.
	tr.Collapse(a)
	if got := tr.Selected().Key; got != "a" {
		t.Fatalf("after collapse selection = %q, want a (pulled up from hidden a2)", got)
	}
}

func TestCollapseKeepsSelectionWhenNotDescendant(t *testing.T) {
	tr := newTestTree()
	root := buildSampleTree()
	tr.SetRoot(root)
	a := root.Children[0]
	b := root.Children[1]

	tr.Expand(b)
	tr.Navigate("b1")

	// Collapsing a (unrelated to selected b1) must not move the selection.
	tr.Collapse(a)
	if got := tr.Selected().Key; got != "b1" {
		t.Fatalf("collapse of unrelated node changed selection to %q, want b1", got)
	}
}

func TestNavigateExpandsAncestors(t *testing.T) {
	tr := newTestTree()
	root := buildSampleTree()
	tr.SetRoot(root)
	a := root.Children[0]

	// a is collapsed; navigating to a deep descendant must expand ancestors so
	// the target becomes visible.
	if tr.IsExpanded(a) {
		t.Fatalf("setup: expected a collapsed before Navigate")
	}
	tr.Navigate("a1")

	if tr.Selected().Key != "a1" {
		t.Fatalf("Navigate did not select target, got %q", tr.Selected().Key)
	}
	if !tr.IsExpanded(a) {
		t.Fatalf("Navigate should expand ancestor a so target is visible")
	}
	// The newly selected node must appear in the visible (flattened) view.
	if indexOf(tr.visibleNodes(), tr.Selected()) < 0 {
		t.Fatalf("Navigated node a1 is not visible after Navigate")
	}
}

func TestNavigateUnknownKeyIsNoop(t *testing.T) {
	tr := newTestTree()
	root := buildSampleTree()
	tr.SetRoot(root)
	tr.Navigate("a")
	before := tr.Selected()

	tr.Navigate("does-not-exist")
	if tr.Selected() != before {
		t.Fatalf("Navigate with unknown key changed selection")
	}
}

func TestDeepNestingTraversal(t *testing.T) {
	const depth = 50
	root, keys := buildDeepTree(depth)

	tr := newTestTree()
	tr.SetRoot(root)

	// Navigate to the deepest node; every ancestor must be expanded.
	deepest := keys[depth-1]
	tr.Navigate(deepest)
	if tr.Selected().Key != deepest {
		t.Fatalf("Navigate to deepest node failed, got %q", tr.Selected().Key)
	}

	visible := tr.visibleNodes()
	if len(visible) != depth {
		t.Fatalf("expected all %d nodes visible after deep Navigate, got %d", depth, len(visible))
	}
	if got := keysOf(visible); strings.Join(got, ",") != strings.Join(keys, ",") {
		t.Fatalf("deep traversal order mismatch:\n got  %v\n want %v", got, keys)
	}

	// MoveUp from the deepest node should land on its parent.
	tr.MoveUp()
	if got := tr.Selected().Key; got != keys[depth-2] {
		t.Fatalf("MoveUp from deepest selected = %q, want %q", got, keys[depth-2])
	}
}

func TestUpdateKeyboardNavigation(t *testing.T) {
	tr := newTestTree()
	root := buildSampleTree()
	tr.SetRoot(root)

	press := func(s string) {
		tr.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(s)}))
	}

	// down: root -> a
	press("j")
	if got := tr.Selected().Key; got != "a" {
		t.Fatalf("after 'j' selected = %q, want a", got)
	}

	// right: expand a
	press("l")
	if !tr.IsExpanded(root.Children[0]) {
		t.Fatalf("after 'l' node a should be expanded")
	}

	// down: a -> a1
	press("j")
	if got := tr.Selected().Key; got != "a1" {
		t.Fatalf("after 'j' selected = %q, want a1", got)
	}

	// left on a leaf moves to parent a
	press("h")
	if got := tr.Selected().Key; got != "a" {
		t.Fatalf("after 'h' on leaf selected = %q, want a", got)
	}

	// left on expanded a collapses it
	press("h")
	if tr.IsExpanded(root.Children[0]) {
		t.Fatalf("after 'h' on expanded node a should collapse")
	}

	// up: a -> root
	press("k")
	if got := tr.Selected().Key; got != "root" {
		t.Fatalf("after 'k' selected = %q, want root", got)
	}
}

func TestViewHighlightsSelectionAndHidesCollapsedChildren(t *testing.T) {
	tr := newTestTree()
	root := buildSampleTree()
	tr.SetRoot(root)

	out := tr.View()
	// Root is expanded so a and b appear; their children are hidden.
	if !strings.Contains(out, "Root") || !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Fatalf("View missing top-level labels:\n%s", out)
	}
	if strings.Contains(out, "A1") || strings.Contains(out, "B1") {
		t.Fatalf("View should hide collapsed children:\n%s", out)
	}

	// Expanding a should reveal A1 and A2.
	tr.Expand(root.Children[0])
	out = tr.View()
	if !strings.Contains(out, "A1") || !strings.Contains(out, "A2") {
		t.Fatalf("View should show children of expanded node:\n%s", out)
	}
}
