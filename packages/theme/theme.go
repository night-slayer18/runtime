// Package theme provides the shared design language for all Runtime applications.
package theme

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Palette defines the color tokens used across the ecosystem.
//
// The JSON tags describe the on-disk format for custom theme files. Each token
// is an object with "light" and "dark" hex strings, for example:
//
//	{ "primary": { "light": "#5B5EA6", "dark": "#7B7FC4" } }
//
// Token keys map onto lipgloss.AdaptiveColor's Light/Dark fields via
// encoding/json's case-insensitive matching.
type Palette struct {
	// Base
	Background lipgloss.AdaptiveColor `json:"background"`
	Surface    lipgloss.AdaptiveColor `json:"surface"`
	Overlay    lipgloss.AdaptiveColor `json:"overlay"`

	// Text
	Text   lipgloss.AdaptiveColor `json:"text"`
	Subtle lipgloss.AdaptiveColor `json:"subtle"`
	Muted  lipgloss.AdaptiveColor `json:"muted"`

	// Accent
	Primary   lipgloss.AdaptiveColor `json:"primary"`
	Secondary lipgloss.AdaptiveColor `json:"secondary"`
	Accent    lipgloss.AdaptiveColor `json:"accent"`

	// Semantic
	Success lipgloss.AdaptiveColor `json:"success"`
	Warning lipgloss.AdaptiveColor `json:"warning"`
	Error   lipgloss.AdaptiveColor `json:"error"`
	Info    lipgloss.AdaptiveColor `json:"info"`

	// Border
	Border      lipgloss.AdaptiveColor `json:"border"`
	BorderFocus lipgloss.AdaptiveColor `json:"borderFocus"`
}

// Default is the canonical Runtime color palette.
var Default = Palette{
	Background: lipgloss.AdaptiveColor{Light: "#FAFAFA", Dark: "#0D0D0D"},
	Surface:    lipgloss.AdaptiveColor{Light: "#F0F0F0", Dark: "#141414"},
	Overlay:    lipgloss.AdaptiveColor{Light: "#E4E4E4", Dark: "#1C1C1C"},

	Text:   lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#E8E8E8"},
	Subtle: lipgloss.AdaptiveColor{Light: "#4A4A4A", Dark: "#A0A0A0"},
	Muted:  lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#5A5A5A"},

	Primary:   lipgloss.AdaptiveColor{Light: "#5B5EA6", Dark: "#7B7FC4"},
	Secondary: lipgloss.AdaptiveColor{Light: "#3D7A6B", Dark: "#5AADA0"},
	Accent:    lipgloss.AdaptiveColor{Light: "#C04B3E", Dark: "#E06E62"},

	Success: lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#66BB6A"},
	Warning: lipgloss.AdaptiveColor{Light: "#E65100", Dark: "#FFA726"},
	Error:   lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#EF5350"},
	Info:    lipgloss.AdaptiveColor{Light: "#1565C0", Dark: "#42A5F5"},

	Border:      lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#2A2A2A"},
	BorderFocus: lipgloss.AdaptiveColor{Light: "#5B5EA6", Dark: "#7B7FC4"},
}

// Styles provides pre-built lipgloss styles derived from a Palette.
type Styles struct {
	// Layout
	App    lipgloss.Style
	Header lipgloss.Style
	Footer lipgloss.Style
	Pane   lipgloss.Style

	// Text
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Body     lipgloss.Style
	Muted    lipgloss.Style

	// Interactive
	Selected  lipgloss.Style
	Focused   lipgloss.Style
	Unfocused lipgloss.Style

	// Semantic
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
	Info    lipgloss.Style

	// Misc
	KeyHint lipgloss.Style
	Badge   lipgloss.Style
}

// NewStyles derives a Styles from a Palette.
func NewStyles(p Palette) Styles {
	return Styles{
		App:    lipgloss.NewStyle().Background(p.Background),
		Header: lipgloss.NewStyle().Background(p.Surface).Foreground(p.Text).Bold(true).Padding(0, 1),
		Footer: lipgloss.NewStyle().Background(p.Surface).Foreground(p.Muted).Padding(0, 1),
		Pane:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Border),

		Title:    lipgloss.NewStyle().Foreground(p.Primary).Bold(true),
		Subtitle: lipgloss.NewStyle().Foreground(p.Secondary),
		Body:     lipgloss.NewStyle().Foreground(p.Text),
		Muted:    lipgloss.NewStyle().Foreground(p.Muted),

		Selected:  lipgloss.NewStyle().Background(p.Primary).Foreground(p.Background).Bold(true),
		Focused:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.BorderFocus),
		Unfocused: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Border),

		Success: lipgloss.NewStyle().Foreground(p.Success),
		Warning: lipgloss.NewStyle().Foreground(p.Warning),
		Error:   lipgloss.NewStyle().Foreground(p.Error),
		Info:    lipgloss.NewStyle().Foreground(p.Info),

		KeyHint: lipgloss.NewStyle().Foreground(p.Muted).Background(p.Overlay).Padding(0, 1),
		Badge:   lipgloss.NewStyle().Background(p.Accent).Foreground(p.Background).Padding(0, 1),
	}
}

// DefaultStyles is the pre-built style set for the default palette.
var DefaultStyles = NewStyles(Default)

// Light is the light-mode palette. Because lipgloss renders the appropriate
// variant of an AdaptiveColor based on the detected terminal background, the
// Light palette pins both the Light and Dark variants to the light tones so it
// renders consistently regardless of the terminal's reported background.
var Light = Palette{
	Background: lipgloss.AdaptiveColor{Light: "#FAFAFA", Dark: "#FAFAFA"},
	Surface:    lipgloss.AdaptiveColor{Light: "#F0F0F0", Dark: "#F0F0F0"},
	Overlay:    lipgloss.AdaptiveColor{Light: "#E4E4E4", Dark: "#E4E4E4"},

	Text:   lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#1A1A1A"},
	Subtle: lipgloss.AdaptiveColor{Light: "#4A4A4A", Dark: "#4A4A4A"},
	Muted:  lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#8A8A8A"},

	Primary:   lipgloss.AdaptiveColor{Light: "#5B5EA6", Dark: "#5B5EA6"},
	Secondary: lipgloss.AdaptiveColor{Light: "#3D7A6B", Dark: "#3D7A6B"},
	Accent:    lipgloss.AdaptiveColor{Light: "#C04B3E", Dark: "#C04B3E"},

	Success: lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#2E7D32"},
	Warning: lipgloss.AdaptiveColor{Light: "#E65100", Dark: "#E65100"},
	Error:   lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#C62828"},
	Info:    lipgloss.AdaptiveColor{Light: "#1565C0", Dark: "#1565C0"},

	Border:      lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#CCCCCC"},
	BorderFocus: lipgloss.AdaptiveColor{Light: "#5B5EA6", Dark: "#5B5EA6"},
}

// LightStyles is the pre-built style set for the light palette.
var LightStyles = NewStyles(Light)

// registry maps theme names to their palettes. It holds the built-in themes
// and is the lookup table used by Get and Resolve.
var registry = map[string]Palette{
	"default": Default,
	"light":   Light,
}

// Names returns the names of all registered themes in no particular order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// Register adds or replaces a named theme in the registry. It allows custom
// palettes (for example, those loaded from theme files) to be resolved by name.
func Register(name string, p Palette) {
	registry[name] = p
}

// Get resolves a palette by name. The boolean return value reports whether a
// theme with the given name exists in the registry.
func Get(name string) (Palette, bool) {
	p, ok := registry[name]
	return p, ok
}

// Resolve returns the palette registered under name, falling back to Default
// when the name is empty or unknown. Use this when an application always needs
// a usable palette and an unknown name should degrade gracefully.
func Resolve(name string) Palette {
	if p, ok := registry[name]; ok {
		return p
	}
	return Default
}

// LoadFile reads a JSON theme file from path and parses it into a Palette.
//
// The file format mirrors the Palette JSON tags: a single object whose keys are
// the color tokens, each an object with "light" and "dark" hex strings. Keys
// that are omitted from the file default to their zero value, so callers that
// want to override only part of a palette should start from an existing palette
// (for example, Default) and decode onto it via the returned Palette.
//
// LoadFile returns an error if the file cannot be read or does not contain
// valid JSON. It does not register the palette; use Register (or Apply after
// registering) to make it resolvable by name.
func LoadFile(path string) (Palette, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Palette{}, fmt.Errorf("read theme file: %w", err)
	}
	var p Palette
	if err := json.Unmarshal(data, &p); err != nil {
		return Palette{}, fmt.Errorf("parse theme file: %w", err)
	}
	return p, nil
}

// Apply resolves the named theme and returns its derived Styles.
//
// The name is matched against the registry, which holds both the built-in
// themes ("default", "light") and any custom palettes added via Register (for
// example, those loaded with LoadFile). Apply returns an error when the name is
// empty or does not match a registered theme, so callers can surface a clear
// failure rather than silently falling back; use Resolve plus NewStyles when a
// graceful fallback to Default is desired instead.
//
// Styles are recomputed in a single NewStyles pass, so the cost of applying a
// theme is bounded by the fixed number of style tokens regardless of how many
// times Apply is called.
func Apply(name string) (Styles, error) {
	p, ok := Get(name)
	if !ok {
		return Styles{}, fmt.Errorf("theme %q not found", name)
	}
	return NewStyles(p), nil
}
