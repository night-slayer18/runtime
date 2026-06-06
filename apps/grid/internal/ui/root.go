// Package ui contains the top-level Bubble Tea model for runtime-grid.
//
// Root wires the shared packages into an interactive workbench: it loads
// configuration and theme on startup, binds the GridModel (which owns the
// table, search, datasource, and export integration), and translates key
// presses into navigation, search, and export actions.
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
	"github.com/runtime-sh/runtime/apps/grid/internal/keymap"
	"github.com/runtime-sh/runtime/apps/grid/internal/model"
	"github.com/runtime-sh/runtime/packages/config"
	"github.com/runtime-sh/runtime/packages/theme"
	"github.com/runtime-sh/runtime/packages/tui"
)

// mode enumerates the top-level interaction modes of the UI.
type mode int

const (
	modeNormal mode = iota // navigating the table
	modeSearch             // typing a search query
)

// Root is the top-level Bubble Tea model for runtime-grid.
type Root struct {
	grid     *model.GridModel
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
// may be supplied; when non-empty it is imported on Init.
func New(path string) Root {
	styles := loadStyles()

	si := textinput.New()
	si.Prompt = "/"
	si.Placeholder = "search"

	return Root{
		grid:   model.New(styles),
		keys:   keymap.Default(),
		help:   help.New(),
		styles: styles,
		search: si,
		path:   path,
	}
}

// loadStyles loads the persisted Grid configuration and resolves its theme,
// falling back to the default styles when no config exists or the theme is
// unknown. Configuration is loaded from the standard per-user location
// (Requirement 10.1).
func loadStyles() theme.Styles {
	cfg := config.DefaultBase()
	if err := config.Load("grid", &cfg); err != nil && !errors.Is(err, config.ErrNotFound) {
		// A malformed config should not prevent launch; fall back to defaults.
		return theme.DefaultStyles
	}
	if styles, err := theme.Apply(cfg.Theme); err == nil {
		return styles
	}
	return theme.DefaultStyles
}

// Init imports the configured file (if any) once the program starts.
func (r Root) Init() tea.Cmd {
	if r.path == "" {
		return nil
	}
	path := r.path
	return func() tea.Msg { return loadFileMsg{path: path} }
}

// loadFileMsg requests that a file be imported.
type loadFileMsg struct{ path string }

// Update handles window sizing, key input, and file-load messages.
func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.size = tui.WindowSizeMsg(msg)
		r.help.Width = msg.Width
		// Reserve two lines for header and footer.
		r.grid.SetSize(msg.Width, msg.Height-2)
		return r, nil

	case loadFileMsg:
		if err := r.grid.LoadFile(msg.path); err != nil {
			r.status = err.Error()
		} else {
			r.status = fmt.Sprintf("loaded %s (%d rows)", r.grid.Path(), r.grid.Table().RowCount())
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

	// Delegate navigation to the grid/table using the shared dispatch action.
	switch r.keys.Dispatch(msg) {
	case tui.ActionUp:
		r.grid.Navigate("up")
	case tui.ActionDown:
		r.grid.Navigate("down")
	case tui.ActionLeft:
		r.grid.Navigate("left")
	case tui.ActionRight:
		r.grid.Navigate("right")
	case tui.ActionPageUp:
		r.grid.Navigate("pgup")
	case tui.ActionPageDown:
		r.grid.Navigate("pgdn")
	case tui.ActionTop:
		r.grid.Navigate("g")
	case tui.ActionBottom:
		r.grid.Navigate("G")
	}
	return r, nil
}

// handleSearchKey handles input while the search field is focused.
func (r Root) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		r.grid.Search(r.search.Value())
		r.mode = modeNormal
		r.search.Blur()
		r.status = fmt.Sprintf("%d rows match %q", r.grid.Table().RowCount(), r.search.Value())
		return r, nil
	case tea.KeyEsc:
		r.mode = modeNormal
		r.search.Blur()
		r.grid.Search("")
		return r, nil
	}
	var cmd tea.Cmd
	r.search, cmd = r.search.Update(msg)
	// Live filtering as the query changes.
	r.grid.Search(r.search.Value())
	return r, cmd
}

// View renders the header, table body, and footer.
func (r Root) View() string {
	header := r.styles.Header.Width(r.size.Width).Render(
		"runtime-grid  •  Tabular data workbench — CSV · TSV · XLSX · Parquet · Arrow")

	var body string
	if r.grid.Loaded() {
		body = r.grid.Table().View()
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
