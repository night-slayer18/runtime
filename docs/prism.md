# runtime-prism

Structured document explorer. Prism parses a document into a navigable tree you can expand, collapse, and search by key or value.

## Supported inputs

The format is detected from the file extension. Each format is backed by a real parser.

| Format | Extensions | Backing library |
|--------|-----------|-----------------|
| JSON | `.json` | `encoding/json` (stdlib) |
| YAML | `.yaml`, `.yml` | [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3) |
| TOML | `.toml` | [`github.com/pelletier/go-toml/v2`](https://github.com/pelletier/go-toml) (full spec, incl. offset datetimes) |
| XML | `.xml` | `encoding/xml` (stdlib) |

Maps, arrays, and scalars are converted into a unified tree: container nodes hold children; leaf nodes render their scalar value inline so you can read values without descending. Map keys are sorted for deterministic output.

## Launch

```sh
runtime-prism [file]
```

The first argument, if present, is the document loaded on startup. Examples:

```sh
runtime-prism examples/prism/config.json
runtime-prism examples/prism/config.yaml
runtime-prism examples/prism/config.toml
runtime-prism examples/prism/data.xml
```

## Keymap

Universal bindings (see the [main keymap](../README.md#universal-keymap)) plus:

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | Move through tree nodes |
| `←`/`h`, `→`/`l` | Collapse / expand (delegated to the tree component) |
| `g` / `G` | Jump to top / bottom |
| `/` | Search — matches node keys and titles live as you type |
| `enter` (in search) | Commit the search and report the match count |
| `esc` (in search) | Cancel search and clear matches |
| `n` / `N` | Jump to the next / previous search match (wraps around) |
| `?` | Toggle help |
| `q` / `ctrl+c` | Quit |

## Features

- Tree navigation over arbitrarily nested documents using the shared `tree` package
- Case-insensitive search across node labels with next/previous match cycling
- Format auto-detection from the file extension

## Configuration

Prism loads its config on startup from the standard per-user location:

- Linux/macOS: `$XDG_CONFIG_HOME/runtime/prism/config.json` (fallback `~/.config/runtime/prism/config.json`)
- Windows: `%APPDATA%\runtime\prism\config.json`

The `theme` field selects the palette. A missing config is fine; a malformed config never prevents launch — Prism falls back to the default theme. See [docs/configuration.md](configuration.md).

## Limitations

- The whole document is read into memory and parsed before display, so very large documents are bounded by available memory.
- YAML maps keyed by non-string scalars are normalised to string keys for display.
