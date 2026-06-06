package model

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/runtime-sh/runtime/packages/search"
	"github.com/runtime-sh/runtime/packages/theme"
	"github.com/runtime-sh/runtime/packages/tree"
)

// PrismModel binds the shared component packages (tree, search) into the
// application's state. It owns the parsed document tree, the active search over
// node labels, and the path/format of the loaded file, and it exposes the
// operations the UI layer drives: loading a file, navigating the tree, and
// searching for nodes by key or value.
type PrismModel struct {
	// document is the navigable tree view over the parsed document.
	document *tree.Tree
	// search performs case-insensitive matching over node labels.
	search *search.Searcher
	// path is the file path of the loaded document, if any.
	path string
	// format is the detected format of the loaded document.
	format Format
	// loaded reports whether a document is currently loaded.
	loaded bool
	// styles is the active theme style set used by the tree.
	styles theme.Styles
	// matches holds the keys of nodes matching the most recent search query, in
	// document order.
	matches []string
	// matchIdx is the index into matches of the currently focused match.
	matchIdx int
}

// New returns a PrismModel with the given theme styles and an empty tree ready
// to receive a document.
func New(styles theme.Styles) *PrismModel {
	return &PrismModel{
		document: tree.New(styles),
		search:   search.New(),
		styles:   styles,
	}
}

// Document exposes the underlying tree for the UI to render and navigate.
func (m *PrismModel) Document() *tree.Tree { return m.document }

// Path returns the path of the loaded file, or "" when nothing is loaded.
func (m *PrismModel) Path() string { return m.path }

// Format returns the format of the loaded document.
func (m *PrismModel) Format() Format { return m.format }

// Loaded reports whether a document is currently loaded.
func (m *PrismModel) Loaded() bool { return m.loaded }

// SetStyles updates the theme styles applied to the tree, supporting a live
// theme change as a single bounded operation.
func (m *PrismModel) SetStyles(styles theme.Styles) {
	m.styles = styles
	m.document.SetStyle(styles)
}

// LoadFile reads and parses the file at path, detecting the format from its
// extension, and installs the resulting tree. It replaces any previously loaded
// document and resets search state.
func (m *PrismModel) LoadFile(path string) error {
	format, ok := FormatFromPath(path)
	if !ok {
		return fmt.Errorf("unsupported file type %q (want .json, .yaml, .toml, or .xml)", filepath.Ext(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	root, err := ParseDocument(data, format)
	if err != nil {
		return fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	m.document.SetRoot(root)
	m.path = path
	m.format = format
	m.loaded = true
	m.search = search.New()
	m.matches = nil
	m.matchIdx = 0
	return nil
}

// LoadDocument installs a pre-parsed tree directly. It is primarily useful for
// tests and for callers that parse documents themselves.
func (m *PrismModel) LoadDocument(root *tree.TreeNode, format Format) {
	m.document.SetRoot(root)
	m.format = format
	m.loaded = root != nil
	m.search = search.New()
	m.matches = nil
	m.matchIdx = 0
}

// Search locates every node whose label (title or key) contains query
// (case-insensitive), records the matching keys in document order, and moves
// the tree selection to the first match. An empty query clears the search.
// It returns the number of matching nodes.
func (m *PrismModel) Search(query string) int {
	m.matches = nil
	m.matchIdx = 0
	if query == "" {
		return 0
	}
	root := m.document.Root()
	if root == nil {
		return 0
	}
	var walk func(n *tree.TreeNode)
	walk = func(n *tree.TreeNode) {
		if n == nil {
			return
		}
		label := n.Title
		if label == "" {
			label = n.Key
		}
		if len(m.search.Search(label, query)) > 0 {
			m.matches = append(m.matches, n.Key)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)

	if len(m.matches) > 0 {
		m.document.Navigate(m.matches[0])
	}
	return len(m.matches)
}

// Matches returns the keys of nodes matching the most recent search, in
// document order.
func (m *PrismModel) Matches() []string {
	out := make([]string, len(m.matches))
	copy(out, m.matches)
	return out
}

// NextMatch advances the selection to the next search match, wrapping around to
// the first. It is a no-op when there are no matches.
func (m *PrismModel) NextMatch() {
	if len(m.matches) == 0 {
		return
	}
	m.matchIdx = (m.matchIdx + 1) % len(m.matches)
	m.document.Navigate(m.matches[m.matchIdx])
}

// PrevMatch moves the selection to the previous search match, wrapping around
// to the last. It is a no-op when there are no matches.
func (m *PrismModel) PrevMatch() {
	if len(m.matches) == 0 {
		return
	}
	m.matchIdx = (m.matchIdx - 1 + len(m.matches)) % len(m.matches)
	m.document.Navigate(m.matches[m.matchIdx])
}
