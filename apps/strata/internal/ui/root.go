// Package ui contains the top-level Bubble Tea model for runtime-strata.
package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/runtime-sh/runtime/apps/strata/internal/keymap"
	"github.com/runtime-sh/runtime/apps/strata/internal/model"
	appds "github.com/runtime-sh/runtime/apps/strata/internal/model/datasource"
	"github.com/runtime-sh/runtime/packages/config"
	"github.com/runtime-sh/runtime/packages/theme"
	"github.com/runtime-sh/runtime/packages/tui"
)

type Root struct {
	state    *model.State
	keys     keymap.KeyMap
	help     help.Model
	styles   theme.Styles
	size     tui.Size
	showHelp bool

	// searching reports whether the query input line is active.
	searching bool
	// queryInput holds the in-progress query while searching.
	queryInput string
	// conn is the connection string supplied on the command line, if any. It is
	// resolved to a backend + DSN and connected on Init.
	conn string
	// status is a transient status/error line shown in the footer.
	status string
}

// New constructs a Root, loading configuration and theme on startup. An
// optional connection string (e.g. "sqlite:file:data.db" or
// "postgres://user@host/db") may be supplied; when non-empty Strata connects to
// it on Init.
func New(conn string) Root {
	styles := loadStyles()
	return Root{
		state:  model.NewWithStyles(styles),
		keys:   keymap.Default(),
		help:   help.New(),
		styles: styles,
		conn:   conn,
	}
}

// loadStyles loads the persisted Strata configuration and resolves its theme,
// falling back to the default styles when no config exists or the theme is
// unknown. Configuration is loaded from the standard per-user location
// (Requirement 10.1).
func loadStyles() theme.Styles {
	cfg := config.DefaultBase()
	if err := config.Load("strata", &cfg); err != nil && !errors.Is(err, config.ErrNotFound) {
		// A malformed config should not prevent launch; fall back to defaults.
		return theme.DefaultStyles
	}
	if styles, err := theme.Apply(cfg.Theme); err == nil {
		return styles
	}
	return theme.DefaultStyles
}

// connectMsg requests that Strata connect to the supplied connection string.
type connectMsg struct{ conn string }

// Init connects to the configured connection string (if any) once the program
// starts.
func (r Root) Init() tea.Cmd {
	if r.conn == "" {
		return nil
	}
	conn := r.conn
	return func() tea.Msg { return connectMsg{conn: conn} }
}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.size = tui.WindowSizeMsg(msg)
		r.help.Width = msg.Width
		// Reserve space for header, schema summary, and footer.
		r.state.Table.SetSize(msg.Width, max(msg.Height-5, 1))
	case connectMsg:
		r.connect(msg.conn)
		return r, nil
	case tea.KeyMsg:
		return r.handleKey(msg)
	}
	return r, nil
}

// handleKey routes key presses either to the query input line or to standard navigation.
func (r Root) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if r.searching {
		return r.handleSearchKey(msg)
	}

	switch r.keys.Dispatch(msg) {
	case tui.ActionQuit:
		return r, tea.Quit
	case tui.ActionHelp:
		r.showHelp = !r.showHelp
	case tui.ActionSearch:
		r.searching = true
		// Pre-populate input with current query if any
		r.queryInput = ""
	case tui.ActionUp, tui.ActionDown, tui.ActionLeft, tui.ActionRight,
		tui.ActionPageUp, tui.ActionPageDown, tui.ActionTop, tui.ActionBottom:
		r.state.Table.Navigate(msg.String())
	}
	return r, nil
}

// handleSearchKey handles character input and executes or cancels queries.
func (r Root) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		r.searching = false
		if r.queryInput != "" {
			r.state.SetSchemaQuery(r.queryInput)
			if _, err := r.state.ReadSchema(); err != nil {
				r.status = fmt.Sprintf("schema error: %v", err)
				return r, nil
			}
			n, err := r.state.RunQuery(r.queryInput)
			if err != nil {
				r.status = fmt.Sprintf("query error: %v", err)
				return r, nil
			}
			r.status = fmt.Sprintf("loaded %d rows", n)
		}
	case tea.KeyEsc:
		r.searching = false
		r.queryInput = ""
	case tea.KeyBackspace:
		if len(r.queryInput) > 0 {
			r.queryInput = r.queryInput[:len(r.queryInput)-1]
		}
	case tea.KeyRunes, tea.KeySpace:
		r.queryInput += string(msg.Runes)
	}
	return r, nil
}

func (r Root) View() string {
	header := r.styles.Header.Width(r.size.Width).Render("runtime-strata  •  Database exploration and administration")

	var body string
	if r.state.Connected && r.state.Table.RowCount() > 0 {
		body = lipgloss.JoinVertical(lipgloss.Left, r.schemaSummary(), r.state.Table.View())
	} else if r.state.Connected {
		body = lipgloss.JoinVertical(lipgloss.Left, r.schemaSummary(),
			r.styles.Muted.Render("Connected. Run a query to view results."))
	} else {
		body = lipgloss.Place(r.size.Width, max(r.size.Height-2, 1), lipgloss.Center, lipgloss.Center,
			r.styles.Muted.Render("Not connected. Connect to a database to explore its schema and run queries."))
	}

	var footer string
	if r.searching {
		footer = r.styles.Footer.Width(r.size.Width).Render("Query: " + r.queryInput + "▏")
	} else if r.showHelp {
		footer = r.styles.Footer.Width(r.size.Width).Render(r.help.View(r.keys))
	} else {
		hints := r.help.ShortHelpView(r.keys.ShortHelp())
		if r.status != "" {
			hints = r.status + "  •  " + hints
		}
		footer = r.styles.Footer.Width(r.size.Width).Render(strings.TrimSpace(hints))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// connect resolves the connection string to a backend + DSN, opens the
// connection through the datasource registry, and loads the source's schema so
// the UI can describe it. Any failure (a bad DSN, an unreachable server, or a
// backend whose driver was excluded from the build, which wraps
// ErrDriverUnavailable) is surfaced in the status line rather than aborting the
// program.
func (r *Root) connect(conn string) {
	backend, dsn, err := appds.ParseConnectionString(conn)
	if err != nil {
		r.status = err.Error()
		return
	}
	if err := r.state.Connect(backend, dsn); err != nil {
		r.status = fmt.Sprintf("connect %s: %v", backend, err)
		return
	}
	// For SQL/NoSQL backends, derive the schema from the connected database/table the
	// user is exploring. Ignore initial lack of query/collection errors.
	if _, err := r.state.ReadSchema(); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "no schema query configured") ||
			strings.Contains(errMsg, "no collection configured") ||
			strings.Contains(errMsg, "no table configured") {
			r.status = fmt.Sprintf("connected to %s", backend)
			return
		}
		r.status = errMsg
		return
	}
	r.status = fmt.Sprintf("connected to %s", backend)
}

// schemaSummary renders a one-line description of the connected source's schema
// using the shared schema package's field types.
func (r Root) schemaSummary() string {
	if len(r.state.Columns) == 0 {
		return r.styles.Subtitle.Render(fmt.Sprintf("%s — no schema loaded", r.state.Backend))
	}
	fields := r.state.SchemaDefinition().Fields()
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, fmt.Sprintf("%s:%s", f.Name, f.Type))
	}
	label := fmt.Sprintf("%s — %s", r.state.Backend, strings.Join(parts, "  "))
	return r.styles.Subtitle.Render(label)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
