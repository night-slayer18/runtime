---
layout: default
title: Grid (Tabular)
nav_order: 4
---

# runtime-grid

Tabular data workbench. Grid imports a data file into a virtualized table you can navigate and filter entirely from the keyboard.

## Supported inputs

Grid imports five formats, each backed by a real decoder. The import format is chosen from the file extension.

| Format | Extensions | Backing library |
|--------|-----------|-----------------|
| CSV | `.csv` | `encoding/csv` (stdlib) |
| TSV | `.tsv` | `encoding/csv` (stdlib), tab-delimited |
| XLSX | `.xlsx` | `archive/zip` + `encoding/xml` (stdlib) — a minimal reader; an `.xlsx` file is a ZIP of XML parts |
| Parquet | `.parquet` | [`github.com/parquet-go/parquet-go`](https://github.com/parquet-go/parquet-go) (pure Go) |
| Arrow | `.arrow`, `.ipc` | [`github.com/apache/arrow-go/v18`](https://github.com/apache/arrow-go) Arrow IPC / Feather v2 |

The first row of a delimited file is treated as the header and becomes the column names.

### Fail-closed launch policy

Grid enforces a fail-closed startup check: it will **not launch** unless every required import format reports that it can be decoded by the build. All five formats ship with working decoders, so Grid starts normally; if a format ever failed to initialize, Grid would refuse to start and print which format is unavailable rather than launching with partial support.

## Launch

```sh
runtime-grid [file]
```

The first argument, if present, is the file imported on startup. Examples:

```sh
runtime-grid examples/grid/people.csv
runtime-grid examples/grid/people.tsv
runtime-grid examples/grid/people.xlsx
runtime-grid examples/grid/people.parquet
runtime-grid examples/grid/people.arrow
runtime-grid examples/grid/sales.csv
```

Launched with no argument, Grid starts on an empty workbench.

## Keymap

Universal bindings (see the [main keymap](../README.md#universal-keymap)) plus:

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j`, `←`/`h`, `→`/`l` | Move the cell cursor |
| `pgup`/`ctrl+u`, `pgdown`/`ctrl+d` | Page up / down |
| `g` / `G` | Jump to top / bottom |
| `/` | Search — filters rows live as you type |
| `enter` (in search) | Commit the filter and report the match count |
| `esc` (in search) | Cancel search and clear the filter |
| `?` | Toggle help |
| `q` / `ctrl+c` | Quit |

## Features

- Virtualized table rendering that only draws visible rows, for responsive navigation over large files
- Live case-insensitive row filtering via the shared `search` package
- Export of the loaded data through the shared `export` package (CSV, JSON, XML, XLSX), with the format chosen from the target extension

## Configuration

Grid loads its config on startup from the standard per-user location:

- Linux/macOS: `$XDG_CONFIG_HOME/runtime/grid/config.json` (fallback `~/.config/runtime/grid/config.json`)
- Windows: `%APPDATA%\runtime\grid\config.json`

The `theme` field selects the palette (`default`, `light`, or a custom theme). A missing config is fine; a malformed config never prevents launch — Grid falls back to the default theme. See [docs/configuration.md](configuration.md) for the full schema and theme files.

## Limitations

- XLSX is read through a minimal stdlib reader (ZIP + XML). It targets the common worksheet layout and is not a full Office Open XML implementation.
- Imported data is loaded fully into an in-memory datasource, so very large files are bounded by available memory.
- All imported cells are presented as text; Grid does not perform type inference on import.
