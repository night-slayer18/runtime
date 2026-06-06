// Package ui contains the top-level Bubble Tea model for runtime-pulse.
package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/runtime-sh/runtime/apps/pulse/internal/keymap"
	"github.com/runtime-sh/runtime/apps/pulse/internal/model"
	"github.com/runtime-sh/runtime/packages/config"
	"github.com/runtime-sh/runtime/packages/theme"
	"github.com/runtime-sh/runtime/packages/tui"
)

// tailInterval is the polling cadence used to pick up newly appended log lines
// for live tailing.
const tailInterval = 500 * time.Millisecond

// tailMsg drives the live-tail polling loop.
type tailMsg time.Time

// errMsg reports a failure opening or reading the log source.
type errMsg struct{ err error }

type Root struct {
	state    *model.State
	keys     keymap.KeyMap
	help     help.Model
	styles   theme.Styles
	size     tui.Size
	showHelp bool

	// searching reports whether the search input line is active.
	searching bool
	// queryInput holds the in-progress filter query while searching.
	queryInput string
	path       string
}

// New builds the root model. When path is non-empty the named log file is
// opened on Init and tailed live. Configuration and theme are loaded from the
// standard per-user location on startup (Requirement 10.1).
func New(path string) Root {
	return Root{
		state:  model.New(),
		keys:   keymap.Default(),
		help:   help.New(),
		styles: loadStyles(),
		path:   path,
	}
}

// loadStyles loads the persisted Pulse configuration and resolves its theme,
// falling back to the default styles when no config exists or the theme is
// unknown. A malformed config never prevents launch.
func loadStyles() theme.Styles {
	cfg := config.DefaultBase()
	if err := config.Load("pulse", &cfg); err != nil && !errors.Is(err, config.ErrNotFound) {
		return theme.DefaultStyles
	}
	if styles, err := theme.Apply(cfg.Theme); err == nil {
		return styles
	}
	return theme.DefaultStyles
}

func (r Root) Init() tea.Cmd {
	if r.path == "" {
		return nil
	}
	if err := r.state.Open(r.path); err != nil {
		return func() tea.Msg { return errMsg{err} }
	}
	return tail()
}

// tail schedules the next live-tail poll.
func tail() tea.Cmd {
	return tea.Tick(tailInterval, func(t time.Time) tea.Msg { return tailMsg(t) })
}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.size = tui.WindowSizeMsg(msg)
		r.help.Width = msg.Width
	case errMsg:
		// Surface the error through the model's state for display.
		return r, nil
	case tailMsg:
		if r.path != "" {
			_ = r.state.Refresh()
			return r, tail()
		}
		return r, nil
	case tea.KeyMsg:
		return r.handleKey(msg)
	}
	return r, nil
}

// handleKey routes a key press either to the search input (when active) or to
// the navigation/command handler.
func (r Root) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if r.searching {
		return r.handleSearchKey(msg)
	}

	switch {
	case key.Matches(msg, r.keys.Quit):
		return r, tea.Quit
	case key.Matches(msg, r.keys.Help):
		r.showHelp = !r.showHelp
	case key.Matches(msg, r.keys.Search):
		r.searching = true
		r.queryInput = r.state.FilterQuery()
	case key.Matches(msg, r.keys.Up):
		r.state.MoveCursor(-1)
	case key.Matches(msg, r.keys.Down):
		r.state.MoveCursor(1)
	case key.Matches(msg, r.keys.PageUp):
		r.state.MoveCursor(-(r.bodyHeight() - 1))
	case key.Matches(msg, r.keys.PageDown):
		r.state.MoveCursor(r.bodyHeight() - 1)
	case key.Matches(msg, r.keys.Top):
		r.state.Top()
	case key.Matches(msg, r.keys.Bottom):
		r.state.Bottom()
	case msg.String() == "tab":
		r.state.ToggleView()
	}
	return r, nil
}

// handleSearchKey edits the in-progress filter query and commits or cancels it.
func (r Root) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		r.state.SetFilter(r.queryInput)
		r.searching = false
	case tea.KeyEsc:
		r.searching = false
		r.queryInput = ""
	case tea.KeyBackspace:
		if len(r.queryInput) > 0 {
			r.queryInput = r.queryInput[:len(r.queryInput)-1]
			r.state.SetFilter(r.queryInput)
		}
	case tea.KeyRunes, tea.KeySpace:
		r.queryInput += string(msg.Runes)
		r.state.SetFilter(r.queryInput)
	}
	return r, nil
}

// bodyHeight is the number of rows available for the log/group body.
func (r Root) bodyHeight() int {
	h := r.size.Height - 2
	if h < 1 {
		h = 1
	}
	return h
}

func (r Root) View() string {
	header := r.styles.Header.Width(r.size.Width).Render("runtime-pulse  •  Log analysis and exploration")

	var body string
	switch {
	case r.state.Err() != nil:
		body = r.centered(r.styles.Error.Render(fmt.Sprintf("runtime-pulse: %v", r.state.Err())))
	case r.path == "":
		body = r.centered(r.styles.Muted.Render("No file loaded. Pass a log file as an argument."))
	case r.state.View() == model.ViewGroups:
		body = r.renderGroups()
	default:
		body = r.renderLog()
	}

	footer := r.renderFooter()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// centered places content in the middle of the body region.
func (r Root) centered(content string) string {
	return lipgloss.Place(r.size.Width, r.bodyHeight(), lipgloss.Center, lipgloss.Center, content)
}

// renderLog renders the filtered log lines around the current cursor.
func (r Root) renderLog() string {
	entries := r.state.Visible()
	height := r.bodyHeight()
	if len(entries) == 0 {
		return r.centered(r.styles.Muted.Render("No matching log lines."))
	}

	start := r.windowStart(r.state.Cursor(), len(entries), height)
	var b strings.Builder
	for i := start; i < len(entries) && i < start+height; i++ {
		line := fmt.Sprintf("%6d  %s", entries[i].Line, entries[i].Text)
		line = fit(line, r.size.Width)
		if i == r.state.Cursor() {
			b.WriteString(r.styles.Selected.Render(line))
		} else {
			b.WriteString(r.styles.Body.Render(line))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// renderGroups renders the similar-error groups ordered by frequency.
func (r Root) renderGroups() string {
	groups := r.state.Groups()
	height := r.bodyHeight()
	if len(groups) == 0 {
		return r.centered(r.styles.Muted.Render("No log lines to group."))
	}

	start := r.windowStart(r.state.Cursor(), len(groups), height)
	var b strings.Builder
	for i := start; i < len(groups) && i < start+height; i++ {
		line := fmt.Sprintf("%6d×  %s", groups[i].Count, groups[i].Template)
		line = fit(line, r.size.Width)
		if i == r.state.Cursor() {
			b.WriteString(r.styles.Selected.Render(line))
		} else {
			b.WriteString(r.styles.Body.Render(line))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// windowStart computes the first visible index so the cursor stays on screen.
func (r Root) windowStart(cursor, total, height int) int {
	if total <= height {
		return 0
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	if start > total-height {
		start = total - height
	}
	return start
}

// renderFooter renders the search prompt, help, or status line.
func (r Root) renderFooter() string {
	if r.searching {
		return r.styles.Footer.Width(r.size.Width).Render("/" + r.queryInput + "▏")
	}
	if r.showHelp {
		return r.styles.Footer.Width(r.size.Width).Render(r.help.View(r.keys))
	}

	mode := "log"
	if r.state.View() == model.ViewGroups {
		mode = "groups"
	}
	status := fmt.Sprintf("%s  •  tab: toggle view", mode)
	if q := r.state.FilterQuery(); q != "" {
		status = fmt.Sprintf("filter:%q  •  %s", q, status)
	}
	help := r.help.ShortHelpView(r.keys.ShortHelp())
	return r.styles.Footer.Width(r.size.Width).Render(help + "  •  " + status)
}

// fit truncates s to at most w terminal cells (measured in runes).
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(rs[:w-1]) + "…"
}
