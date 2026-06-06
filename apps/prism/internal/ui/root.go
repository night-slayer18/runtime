// Package ui contains the top-level Bubble Tea model for runtime-prism.
//
// Root wires the shared packages into an interactive document explorer: it
// loads configuration and theme on startup, binds the PrismModel (which owns
// the document tree and search), and translates key presses into tree
// navigation and search actions.
package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/runtime-sh/runtime/apps/prism/internal/keymap"
	"github.com/runtime-sh/runtime/apps/prism/internal/model"
	"github.com/runtime-sh/runtime/packages/config"
	"github.com/runtime-sh/runtime/packages/theme"
	"github.com/runtime-sh/runtime/packages/tui"
)

// mode enumerates the top-level interaction modes of the UI.
type mode int

const (
	modeNormal mode = iota // navigating the tree
	modeSearch             // typing a search query
)

// Root is the top-level Bubble Tea model for runtime-prism.
type Root struct {
	prism    *model.PrismModel
	keys     keymap.KeyMap
	help     help.Model
	styles   theme.Styles
	size     tui.Size
	showHelp bool

	mode   mode
	search textinput.Model

	// path is the file to load on Init, if any.
	path string
	// status is a transient status/error line shown in the footer.
	status string
}

// New constructs a Root, loading configuration and theme. An optional file path
// may be supplied; when non-empty it is loaded on Init.
func New(path string) Root {
	styles := loadStyles()

	si := textinput.New()
	si.Prompt = "/"
	si.Placeholder = "search"

	return Root{
		prism:  model.New(styles),
		keys:   keymap.Default(),
		help:   help.New(),
		styles: styles,
		search: si,
		path:   path,
	}
}

// loadStyles loads the persisted Prism configuration and resolves its theme,
// falling back to the default styles when no config exists or the theme is
// unknown. Configuration is loaded from the standard per-user location
// (Requirement 10.1).
func loadStyles() theme.Styles {
	cfg := config.DefaultBase()
	if err := config.Load("prism", &cfg); err != nil && !errors.Is(err, config.ErrNotFound) {
		// A malformed config should not prevent launch; fall back to defaults.
		return theme.DefaultStyles
	}
	if styles, err := theme.Apply(cfg.Theme); err == nil {
		return styles
	}
	return theme.DefaultStyles
}

// Init loads the configured file (if any) once the program starts.
func (r Root) Init() tea.Cmd {
	if r.path == "" {
		return nil
	}
	path := r.path
	return func() tea.Msg { return loadFileMsg{path: path} }
}

// loadFileMsg requests that a document be loaded.
type loadFileMsg struct{ path string }

// Update handles window sizing, key input, and file-load messages.
func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.size = tui.WindowSizeMsg(msg)
		r.help.Width = msg.Width
		return r, nil

	case loadFileMsg:
		if err := r.prism.LoadFile(msg.path); err != nil {
			r.status = err.Error()
		} else {
			r.status = fmt.Sprintf("loaded %s (%s)", r.prism.Path(), r.prism.Format())
		}
		return r, nil

	case tea.KeyMsg:
		return r.handleKey(msg)
	}
	return r, nil
}

// handleKey routes a key press based on the current mode.
func (r Root) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if r.mode == modeSearch {
		return r.handleSearchKey(msg)
	}

	switch {
	case key.Matches(msg, r.keys.Quit):
		return r, tea.Quit
	case key.Matches(msg, r.keys.Help):
		r.showHelp = !r.showHelp
		return r, nil
	case key.Matches(msg, r.keys.Search):
		r.mode = modeSearch
		r.search.SetValue("")
		r.search.Focus()
		return r, textinput.Blink
	}

	// "n" / "N" cycle through search matches when a search is active.
	switch msg.String() {
	case "n":
		r.prism.NextMatch()
		return r, nil
	case "N":
		r.prism.PrevMatch()
		return r, nil
	}

	// Delegate navigation and expand/collapse to the tree component.
	r.prism.Document().Update(msg)
	return r, nil
}

// handleSearchKey handles input while the search field is focused.
func (r Root) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		n := r.prism.Search(r.search.Value())
		r.mode = modeNormal
		r.search.Blur()
		r.status = fmt.Sprintf("%d nodes match %q", n, r.search.Value())
		return r, nil
	case tea.KeyEsc:
		r.mode = modeNormal
		r.search.Blur()
		r.prism.Search("")
		return r, nil
	}
	var cmd tea.Cmd
	r.search, cmd = r.search.Update(msg)
	// Live search as the query changes.
	r.prism.Search(r.search.Value())
	return r, cmd
}

// View renders the header, tree body, and footer.
func (r Root) View() string {
	header := r.styles.Header.Width(r.size.Width).Render(
		"runtime-prism  •  Structured document explorer — JSON · YAML · TOML · XML")

	var body string
	if r.prism.Loaded() {
		body = r.prism.Document().View()
	} else {
		body = lipgloss.Place(r.size.Width, max(r.size.Height-2, 1), lipgloss.Center, lipgloss.Center,
			r.styles.Muted.Render("No file loaded. Pass a file as an argument."))
	}

	footer := r.renderFooter()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// renderFooter renders the search field, status line, or help, depending on the
// current mode.
func (r Root) renderFooter() string {
	if r.mode == modeSearch {
		return r.styles.Footer.Width(r.size.Width).Render(r.search.View())
	}
	if r.showHelp {
		return r.styles.Footer.Width(r.size.Width).Render(r.help.View(r.keys))
	}

	hints := r.help.ShortHelpView(r.keys.ShortHelp())
	if r.status != "" {
		hints = r.status + "  •  " + hints
	}
	return r.styles.Footer.Width(r.size.Width).Render(strings.TrimSpace(hints))
}

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
