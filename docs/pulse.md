# runtime-pulse

Log exploration and analysis. Pulse tails a log file live, filters lines by pattern, and groups similar messages so you can spot recurring errors.

## Supported inputs

Pulse reads any plain-text log file line by line. The sample data includes a plain log, a structured log, and JSON lines:

```sh
runtime-pulse examples/pulse/app.log
runtime-pulse examples/pulse/structured.log
runtime-pulse examples/pulse/app.jsonl
```

While the file is open, Pulse polls it roughly every 500ms and picks up newly appended lines, so it works as a live tail.

## Filtering

The filter query is interpreted as a **regular expression** (via the stdlib `regexp` package) when it compiles successfully. If the query is not a valid regular expression, Pulse falls back to case-insensitive substring matching via the shared `search` package, so the filter never rejects input. An empty query matches every line.

## Views

Pulse has two presentations, toggled with `tab`:

- **Log view** — the (optionally filtered) stream of log lines with their line numbers.
- **Groups view** — lines collapsed into templated groups ordered by frequency (a count followed by the group template), which surfaces the most common error shapes.

## Launch

```sh
runtime-pulse [file]
```

## Keymap

Universal bindings (see the [main keymap](../README.md#universal-keymap)) plus:

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | Move the cursor one line |
| `pgup`/`ctrl+u`, `pgdown`/`ctrl+d` | Page up / down |
| `g` / `G` | Jump to top / bottom |
| `/` | Start a filter query |
| `enter` (in filter) | Apply the filter |
| `esc` (in filter) | Cancel and clear the filter |
| `tab` | Toggle between log view and groups view |
| `?` | Toggle help |
| `q` / `ctrl+c` | Quit |

## Features

- Live log tailing with incremental polling
- Regex filtering with graceful substring fallback, case-insensitive by default
- Similar-error grouping ordered by frequency

## Configuration

Pulse loads its config on startup from the standard per-user location:

- Linux/macOS: `$XDG_CONFIG_HOME/runtime/pulse/config.json` (fallback `~/.config/runtime/pulse/config.json`)
- Windows: `%APPDATA%\runtime\pulse\config.json`

The `theme` field selects the palette. A missing config is fine; a malformed config never prevents launch. See [docs/configuration.md](configuration.md).

## Limitations

- Pulse keeps a bounded in-memory window of recent entries rather than the entire file; filtering and grouping operate on that window.
- Live tailing uses polling (~500ms), so newly appended lines appear with a short delay rather than instantly.
