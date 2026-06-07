// Package ui contains the top-level Bubble Tea model for runtime-vault.
package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/runtime-sh/runtime/apps/vault/internal/keymap"
	"github.com/runtime-sh/runtime/apps/vault/internal/model"
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
}

// New constructs the root model. When path is non-empty the referenced artifact
// is loaded and inspected immediately. Secret values are never retained for
// display — only masked metadata is shown. Configuration and theme are loaded
// from the standard per-user location on startup.
func New(path string) Root {
	state := model.New()
	if path != "" {
		state = model.LoadFile(path)
	}
	return Root{
		state:  state,
		keys:   keymap.Default(),
		help:   help.New(),
		styles: loadStyles(),
	}
}

// loadStyles loads the persisted Vault configuration and resolves its theme,
// falling back to the default styles when no config exists or the theme is
// unknown. A malformed config never prevents launch (Requirement 10.1, 4.2).
func loadStyles() theme.Styles {
	cfg := config.DefaultBase()
	if err := config.Load("vault", &cfg); err != nil && !errors.Is(err, config.ErrNotFound) {
		return theme.DefaultStyles
	}
	if styles, err := theme.Apply(cfg.Theme); err == nil {
		return styles
	}
	return theme.DefaultStyles
}

func (r Root) Init() tea.Cmd { return nil }

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.size = tui.WindowSizeMsg(msg)
		r.help.Width = msg.Width
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, r.keys.Quit):
			return r, tea.Quit
		case key.Matches(msg, r.keys.Help):
			r.showHelp = !r.showHelp
		}
	}
	return r, nil
}

func (r Root) View() string {
	header := r.styles.Header.Width(r.size.Width).Render("runtime-vault  •  Secrets and configuration explorer")
	body := lipgloss.Place(r.size.Width, r.size.Height-2, lipgloss.Center, lipgloss.Center, r.bodyContent())

	var footer string
	if r.showHelp {
		footer = r.styles.Footer.Width(r.size.Width).Render(r.help.View(r.keys))
	} else {
		footer = r.styles.Footer.Width(r.size.Width).Render(r.help.ShortHelpView(r.keys.ShortHelp()))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// bodyContent renders the current inspection result or a prompt. It only ever
// renders masked, non-sensitive metadata supplied by the model.
func (r Root) bodyContent() string {
	if r.state.Err != nil {
		return r.styles.Error.Render(fmt.Sprintf("error: %v", r.state.Err))
	}
	insp := r.state.Inspection
	if insp == nil {
		return r.styles.Muted.Render("No file loaded. Pass a file as an argument.")
	}

	var b strings.Builder
	b.WriteString(r.styles.Title.Render(fmt.Sprintf("%s — %s", insp.Source, insp.Kind)))
	b.WriteString("\n\n")

	if len(insp.Fields) == 0 {
		b.WriteString(r.styles.Muted.Render("(no fields)"))
	}
	for _, f := range insp.Fields {
		label := r.styles.Subtitle.Render(f.Key)
		value := f.Value
		if f.Sensitive {
			value = r.styles.Muted.Render(value)
		} else {
			value = r.styles.Body.Render(value)
		}
		fmt.Fprintf(&b, "%s: %s\n", label, value)
	}

	if len(insp.Issues) > 0 {
		b.WriteString("\n")
		for _, issue := range insp.Issues {
			b.WriteString(r.styles.Warning.Render("⚠ "+issue) + "\n")
		}
	} else {
		b.WriteString("\n" + r.styles.Success.Render("✓ valid"))
	}

	return b.String()
}
