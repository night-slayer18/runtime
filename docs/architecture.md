---
layout: default
title: Architecture
nav_order: 2
---

# Architecture

Runtime is a Go monorepo of independently versioned modules: five applications under `apps/` and eleven shared packages under `packages/`. This document describes how the pieces fit together.

## Monorepo layout

```
runtime/
├── go.work              workspace tying every module together for local dev
├── Makefile             build / test / lint / install / versioning
├── apps/                application modules
│   ├── grid/  prism/  pulse/  strata/  vault/
├── packages/            shared modules
│   ├── tui/  theme/  config/  table/  tree/  search/
│   ├── plugin/  datasource/  export/  schema/  validation/
├── scripts/             version.sh, build-smoke.sh
├── docs/                documentation
├── examples/            sample data + TESTING.md
└── .github/workflows/   CI/CD pipelines
```

The repository root is **not** a Go module. Each app and package is its own module:

- Apps: `module github.com/runtime-sh/runtime/apps/<app>`
- Packages: `module github.com/runtime-sh/runtime/packages/<pkg>`

Every module declares `go 1.25.0`. Because the root is not a module, `go test ./...` cannot be run from the root — tests run per module (the Makefile and CI iterate over modules). `go.work` lists every module so local builds resolve cross-module imports without published versions; each app's `go.mod` also carries `replace` directives pointing at the sibling package directories.

## Apps vs packages

| | Apps (`apps/`) | Packages (`packages/`) |
|--|----------------|------------------------|
| Purpose | Domain-specific TUI executables | Reusable, app-agnostic building blocks |
| Output | A binary (`cmd/<app>/main.go`) | A library imported by apps |
| Versioning | `<app>/vX.Y.Z` tags | `packages/<pkg>/vX.Y.Z` tags |
| Imports | May import any package | Must not import any app |

### Dependency direction

The single most important rule: **apps import packages; packages never import apps.** Packages may import other packages (for example, `theme` is used widely; `table` and `tree` are used by apps via the shared style set), but nothing under `packages/` imports anything under `apps/`. This keeps the shared foundation reusable and prevents dependency cycles.

### Standard app structure

Every app follows the same internal shape:

```
apps/<app>/
├── cmd/<app>/main.go      entry point: panic handler, arg parsing, tea.NewProgram
└── internal/
    ├── keymap/            app-specific key bindings (embeds tui.GlobalKeyMap)
    ├── model/             application state & business logic
    │   └── datasource/    app-specific format / backend adapters (grid, strata)
    └── ui/root.go         top-level Bubble Tea model (Init / Update / View)
```

Each `main.go` installs a top-level `recover()` that prints `runtime-<app>: panic: …` with a stack trace and exits 1. The first CLI argument, when present, is the file (or connection string, for Strata) loaded on startup. Programs run with `tea.WithAltScreen()` and `tea.WithMouseCellMotion()`.

## Shared packages

| Package | Responsibility |
|---------|----------------|
| `tui` | Bubble Tea primitives, the `GlobalKeyMap`, and the `Dispatch` action mapping that gives every app identical navigation handling |
| `theme` | `Palette` + derived `Styles`, built-in `default`/`light` palettes, JSON theme file loading, and a name→palette registry |
| `config` | Cross-platform config dir resolution, JSON load/save, and a polling `Watch` for live reload |
| `table` | Virtualized, streaming table component with sorting/filtering |
| `tree` | Hierarchical tree navigation with expand/collapse |
| `search` | Case-insensitive search/filter primitives |
| `plugin` | Plugin registry, capability sandbox, and integrity verification |
| `datasource` | The `DataSource` abstraction (schema / query / iterate / close) and an in-memory implementation |
| `export` | Exporters for CSV, JSON, XML, and XLSX |
| `schema` | Field/type schema definitions and validation |
| `validation` | Composable input validators |

## Registry patterns

Two apps use a registry so concrete decoders/drivers are pluggable without touching core code.

### Grid: format registry

Grid's `internal/model/datasource` keeps a registry of import `Format`s keyed by name. Each format reports whether it is `Available` (decodable by this build) and supplies a `Reader`. `Open(path)` dispatches on the file extension to the matching reader.

Grid enforces a **fail-closed launch policy**: `CheckAvailability()` returns an error enumerating any format that is not available, and `main.go` refuses to launch when it does. All five formats (CSV, TSV, XLSX, Parquet, Arrow) ship with real readers, so the check passes — but the mechanism stays in place so a format that fails to initialize is reported honestly rather than launching with partial support.

### Strata: driver registry

Strata's `internal/model/datasource` registers a `ConnectionFactory` per backend name from `init` functions in `drivers.go` — the single, isolated place that pulls concrete drivers into the build. `Connect(backend, dsn)` resolves the factory by name. SQL backends (Postgres, MySQL, SQLite) are thin adapters over `database/sql`; MongoDB and Cassandra are dedicated adapters over their native clients. `ParseConnectionString` maps a connection-string scheme to a backend and the DSN its driver expects. Adding a backend is one `Register` call; an unreachable server surfaces as a connection error, and a backend whose driver was intentionally left out surfaces as `ErrDriverUnavailable`.

## Plugin model

The `plugin` package provides an in-process plugin system in two layers:

- **Registry** (`plugin.go`) — hosts plugins and the extension points they register: commands, data sources, views, and an event bus. Each plugin receives a scoped `PluginContext` during `Init`, so every registration is attributed to its owner and torn down on `Unregister`. The registry is safe for concurrent use.
- **Sandbox** (`sandbox.go`) — a capability guard plus integrity verification. Capabilities (`filesystem`, `network`, `process`) are **deny-by-default**; a plugin must be granted a capability to use the mediated API for it. Plugin entry points run in isolated goroutines with panic recovery and an optional timeout. Integrity is verified before execution via an **ed25519** detached signature and/or a **SHA-256** checksum; verification fails closed (no strategy configured ⇒ not verified ⇒ refuse to run).

The package is explicit about the honest limit of in-process sandboxing: Go cannot revoke capabilities from code running in the same process, so the guard enforces *policy at the API boundary*, not OS-level isolation. True isolation (separate process, seccomp/landlock, WASM, container) must be layered underneath by a host loading untrusted code.

## Configuration & theming

Configuration is shared through the `config` package: a `BaseConfig` (theme, mouse, log level, plugin paths/enablement) loaded per app from the platform config directory, with an optional polling watcher for live reload. Theming is shared through the `theme` package: a `Palette` of adaptive color tokens, derived `Styles`, built-in `default` and `light` palettes, and JSON theme files resolvable by name. See [configuration.md](configuration.md).

## Correctness properties & PBT testing

The ecosystem is tested with a dual approach (see the spec's design document):

- **Example-based tests** verify specific cases and edge cases (`*_test.go`).
- **Property-based tests** verify universal properties across generated inputs (`*_property_test.go`), using Go's `testing/quick`.

Property tests are tagged in a comment with the form:

```
Feature: runtime-ecosystem, Property <N>: <property text>
Validates: Requirements <list>
```

The documented correctness properties cover, among others: universal keyboard navigation (Property 1), theme consistency across apps (Property 2), config schema compatibility (Property 3), large-dataset performance (Property 4), streaming load (Property 5), plugin sandboxing (Property 6), plugin API consistency (Property 7), and theme update performance (Property 8). Property suites live alongside the code they exercise — for example `packages/tui/tui_property_test.go`, `packages/table/streaming_property_test.go` and `performance_property_test.go`, `packages/plugin/sandbox_property_test.go`, and `packages/theme/ecosystem_property_test.go`.

Run everything per module with `make test`; verify every module compiles with `make build-smoke`.
