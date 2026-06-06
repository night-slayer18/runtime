# Configuration

Every Runtime application shares one configuration model, provided by the `packages/config` module. Configuration is per-app, stored as JSON in the platform's standard config directory, and applied on startup. The schema is compatible across all apps, so knowledge transfers between them.

## Config schema

The shared `BaseConfig` is what every app loads:

```jsonc
{
  "theme": "default",       // "default", "light", or a custom theme name
  "mouse": false,           // enable mouse support
  "log_level": "warn",      // "debug" | "info" | "warn" | "error"
  "plugin": {
    "paths": [],            // directories to search for plugins
    "enabled": {}           // per-plugin enable/disable, keyed by plugin name
  }
}
```

Defaults (used when no config file exists): `theme: "default"`, `mouse: false`, `log_level: "warn"`, and empty plugin paths/enablement.

A missing config file is **not** an error — apps fall back to defaults. A malformed config never prevents launch; the app logs nothing sensitive and falls back to the default theme/styles.

## File locations

Config is stored per app under a `runtime/<app>/config.json` path:

| Platform | Location |
|----------|----------|
| Linux / macOS | `$XDG_CONFIG_HOME/runtime/<app>/config.json` |
| Linux / macOS (fallback when `XDG_CONFIG_HOME` is unset) | `~/.config/runtime/<app>/config.json` |
| Windows | `%APPDATA%\runtime\<app>\config.json` |

`<app>` is one of `grid`, `prism`, `pulse`, `strata`, `vault`. For example, Grid on Linux reads `~/.config/runtime/grid/config.json`.

When an app saves config, the directory is created automatically with `0700` permissions and the file is written with `0600` permissions and indented JSON.

## Themes

The `theme` field selects a palette resolved by the `packages/theme` module:

- `default` — the dark-first canonical palette
- `light` — a light palette pinned to light tones
- a **custom theme name** — any palette registered from a JSON theme file

### Theme files

A theme file is a single JSON object whose keys are color tokens, each an object with `light` and `dark` hex strings:

```json
{
  "background":  { "light": "#FAFAFA", "dark": "#0D0D0D" },
  "text":        { "light": "#1A1A1A", "dark": "#E8E8E8" },
  "primary":     { "light": "#5B5EA6", "dark": "#7B7FC4" },
  "secondary":   { "light": "#3D7A6B", "dark": "#5AADA0" },
  "accent":      { "light": "#C04B3E", "dark": "#E06E62" },
  "success":     { "light": "#2E7D32", "dark": "#66BB6A" },
  "warning":     { "light": "#E65100", "dark": "#FFA726" },
  "error":       { "light": "#C62828", "dark": "#EF5350" },
  "info":        { "light": "#1565C0", "dark": "#42A5F5" },
  "border":      { "light": "#CCCCCC", "dark": "#2A2A2A" },
  "borderFocus": { "light": "#5B5EA6", "dark": "#7B7FC4" }
}
```

The full token set is: `background`, `surface`, `overlay`, `text`, `subtle`, `muted`, `primary`, `secondary`, `accent`, `success`, `warning`, `error`, `info`, `border`, `borderFocus`. Tokens omitted from a file default to their zero value, so a custom theme is best built by overriding tokens on top of an existing palette. lipgloss renders the appropriate `light`/`dark` variant based on the detected terminal background.

Styles (header, footer, title, selected, semantic colors, etc.) are derived from the palette in a single pass, so applying a theme is a bounded operation regardless of how many components are on screen.

## Live reload

The `config` package can watch an app's `config.json` and apply changes without a restart. It uses a cross-platform polling strategy (default cadence: 1 second) with no external dependencies:

- The watcher invokes a callback with the freshly loaded `BaseConfig` whenever the file is created, modified, or removed and recreated.
- The callback is never invoked with a config that failed to parse — malformed files are skipped until they become valid again.
- The watcher does not fire for the file's initial state, only for subsequent changes, so callers load the current config once before starting the watch.
- Starting a watch returns a `stop` function that cancels the watcher and waits for its goroutine to exit.

This satisfies the requirement that saved configuration changes apply without requiring a restart.
