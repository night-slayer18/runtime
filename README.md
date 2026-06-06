# Runtime

A family of terminal-native developer tools built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea). Each tool is keyboard-first, cross-platform, and shares a common foundation — a unified keymap, theme system, and configuration model — while installing and releasing on its own.

Runtime is a Go monorepo: five applications under `apps/` and eleven shared packages under `packages/`. The repository root is **not** a Go module; every app and package is its own module, wired together for local development through `go.work`.

## The app lineup

| App | What it does |
|-----|--------------|
| **grid** | Tabular data workbench — reads CSV, TSV, XLSX, Parquet, and Arrow files into a virtualized table. |
| **prism** | Structured document explorer — tree navigation and search over JSON, YAML, TOML, and XML. |
| **pulse** | Log exploration — live tailing, regex/substring filtering, and similar-error grouping. |
| **strata** | Database exploration — connects to PostgreSQL, MySQL, SQLite, MongoDB, and Cassandra through a driver registry. |
| **vault** | Secrets and configuration explorer — inspects env files, JWTs, certificates, API keys, and Kubernetes secrets with secret masking. |

## Architecture

```
runtime/
├── go.work                 Go workspace tying every module together for local dev
├── Makefile                build, test, lint, install, and versioning targets
├── apps/                   application modules (independent go modules)
│   ├── grid/               Tabular data workbench (CSV · TSV · XLSX · Parquet · Arrow)
│   ├── prism/              Structured document explorer (JSON · YAML · TOML · XML)
│   ├── pulse/              Log exploration and analysis
│   ├── strata/             Database exploration and administration
│   └── vault/              Secrets and configuration explorer
├── packages/               shared modules (importable by any app)
│   ├── tui/                Bubble Tea primitives & the universal keymap
│   ├── theme/              Design language: palettes, styles, JSON theme files
│   ├── config/             Cross-platform config loading, persistence & live reload
│   ├── table/              Virtualized + streaming table component
│   ├── tree/               Tree navigation component
│   ├── search/             Search / filter primitives
│   ├── plugin/             Plugin registry, capability sandbox & integrity verification
│   ├── datasource/         DataSource abstraction (schema / query / iterate)
│   ├── export/             Export utilities (CSV · JSON · XML · XLSX)
│   ├── schema/             Schema definition & validation
│   └── validation/         Input validation primitives
├── scripts/                build & release automation (version.sh, build-smoke.sh)
├── docs/                   project and per-app documentation
├── examples/               sample data for every app + a testing guide
└── .github/workflows/      CI/CD pipelines (ci, lint, security, build, release)
```

The dependency direction is one-way: **apps import packages; packages never import apps.** See [docs/architecture.md](docs/architecture.md) for the full layout, module boundaries, and the registry/plugin patterns.

## Requirements

- Go **1.25.x** (every module declares `go 1.25.0`)

## Quick start

```sh
# 1. Clone
git clone https://github.com/runtime-sh/runtime.git
cd runtime

# 2. Build all apps (binaries land in ./bin/)
make build

# 3. Run an app against an example file
./bin/runtime-grid   examples/grid/people.csv
./bin/runtime-prism  examples/prism/config.yaml
./bin/runtime-pulse  examples/pulse/app.log
./bin/runtime-vault  examples/vault/sample.env
./bin/runtime-strata sqlite:examples/strata/sample.db
```

Build a single app:

```sh
make app APP=prism
```

Run directly without building a binary:

```sh
cd apps/prism && go run ./cmd/prism ../../examples/prism/config.yaml
```

Install all apps onto your `PATH` (via `go install`):

```sh
make install
```

## Universal keymap

Every app honours the same navigation, search, help, and quit bindings (defined in `packages/tui`). Apps add their own bindings on top — see the per-app docs.

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `←` / `h` | Move left |
| `→` / `l` | Move right |
| `pgup` / `ctrl+u` | Page up |
| `pgdown` / `ctrl+d` | Page down |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `/` | Search |
| `?` | Toggle help |
| `q` / `ctrl+c` | Quit |

## Documentation

- [docs/README.md](docs/README.md) — documentation index
- [docs/architecture.md](docs/architecture.md) — monorepo layout, module boundaries, registry & plugin patterns, testing approach
- [docs/configuration.md](docs/configuration.md) — config schema, file locations, themes, live reload
- [docs/releasing.md](docs/releasing.md) — cutting releases with `scripts/version.sh` and the release pipeline
- Per-app guides: [grid](docs/grid.md) · [prism](docs/prism.md) · [pulse](docs/pulse.md) · [strata](docs/strata.md) · [vault](docs/vault.md)
- [examples/TESTING.md](examples/TESTING.md) — how to exercise each app against the sample data in `examples/`

## Versioning and releases

Each module is versioned and released **independently**. The current version of every module is tracked in a per-module `VERSION` file (all baselined at `v0.1.0`) and managed through [`scripts/version.sh`](scripts/version.sh), which can bump a single module, all apps, all packages, or everything at once, and create + push the matching git tags.

```sh
make version-list                                  # show every module's version
make version-bump MODULE=apps/grid BUMP=patch      # bump one module
make version-bump-all BUMP=minor ARGS="--tag"      # coordinated bump + tag all
```

Pushing a tag triggers `.github/workflows/release.yml`. Tag conventions:

| Tag | Release |
|-----|---------|
| `<app>/vX.Y.Z` (e.g. `grid/v0.1.0`) | App release: cross-platform binaries (linux/darwin/windows × amd64/arm64), `SHA256SUMS.txt`, and a changelog. |
| `packages/<pkg>/vX.Y.Z` (e.g. `packages/theme/v0.1.0`) | Library release: a GitHub Release with a changelog and no binaries. |

Full details in [docs/releasing.md](docs/releasing.md) and [.github/workflows/README.md](.github/workflows/README.md).

## Design principles

- **Terminal-first** — keyboard workflows are primary; mouse support is an optional enhancement
- **Performance-first** — virtualization, streaming, and lazy loading throughout
- **Cross-platform** — Linux, macOS, and Windows with no platform-specific code
- **Shared ecosystem** — one design language, one keymap, one theme and config system
- **Independent applications** — each app installs and releases on its own

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for workspace setup, the module boundary rules, how to add a new app or shared package, the signed-off commit convention, and how CI gates pull requests.
