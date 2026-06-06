# Contributing to Runtime

Thanks for contributing. Runtime is a Go monorepo of independently versioned modules. This guide covers setting up, the workspace, the module boundary rules, and how changes get gated.

## Prerequisites

- Go **1.25.x** (every module declares `go 1.25.0`)
- `make`
- `golangci-lint` (for `make lint`)

## Setup

```sh
git clone https://github.com/runtime-sh/runtime.git
cd runtime
make build      # builds every app into ./bin/
```

## The go.work workspace

The repository root is **not** a Go module. Each app under `apps/*` and each package under `packages/*` is its own module. `go.work` lists every module so cross-module imports resolve locally without published versions:

```
go 1.25.0

use (
    ./apps/grid
    ./apps/prism
    ...
    ./packages/tui
    ./packages/theme
    ...
)
```

Each app's `go.mod` also carries `replace` directives pointing at the sibling package directories, so apps build against your local package changes.

Because the root is not a module, you cannot run `go test ./...` from the root. Run per-module commands (the Makefile does this for you) or `cd` into a module first.

## Building, testing, and linting

```sh
make build         # build all apps into ./bin/
make app APP=grid  # build a single app
make build-smoke   # assert every module in the workspace compiles
make test          # go test ./... in every module
make lint          # golangci-lint run ./... in every module
make install       # go install every app onto your PATH
make clean         # remove ./bin/
```

To work on one module:

```sh
cd packages/table
go test ./...
go test -run Property ./...   # property-based tests
```

### Tests

Add tests with your changes. The ecosystem uses a dual approach:

- **Example-based tests** (`*_test.go`) — specific cases and edge cases.
- **Property-based tests** (`*_property_test.go`) — universal properties over generated inputs, using Go's `testing/quick`. Tag each with a comment of the form:

  ```
  Feature: runtime-ecosystem, Property <N>: <property text>
  Validates: Requirements <list>
  ```

Run `make build-smoke` and `make test` before opening a PR.

## Module boundary rules

The dependency direction is one-way and must be preserved:

- **Apps import packages; packages never import apps.** Nothing under `packages/` may import anything under `apps/`.
- Packages may import other packages, but avoid introducing import cycles.
- Shared functionality used by more than one app belongs in `packages/`, not duplicated in apps.
- App-specific logic (format adapters, DB drivers, inspectors) lives in that app's `internal/` tree.

See [docs/architecture.md](docs/architecture.md) for the full layout.

## Adding a new shared package

1. Create `packages/<pkg>/` with `module github.com/runtime-sh/runtime/packages/<pkg>` in its `go.mod` (`go 1.25.0`).
2. Add a `VERSION` file containing `v0.1.0`.
3. Add the module to `go.work`'s `use` block.
4. Register the module path in `scripts/version.sh` (the `PACKAGES` array) so versioning/releasing picks it up.
5. Add a `gomod` entry for the new directory in `.github/dependabot.yml`.
6. Consuming apps add it to their `require` block plus a `replace` directive pointing at `../../packages/<pkg>`.

## Adding a new app

1. Create `apps/<app>/` with `module github.com/runtime-sh/runtime/apps/<app>` (`go 1.25.0`) and the standard layout: `cmd/<app>/main.go` and `internal/{keymap,model,ui}`.
2. Add a `VERSION` file containing `v0.1.0`.
3. Add the module to `go.work` and to the `APPS` array in both the `Makefile` and `scripts/version.sh`.
4. Add a `gomod` entry to `.github/dependabot.yml`.
5. Embed `tui.GlobalKeyMap` in the app's keymap so it inherits the universal bindings.
6. Add a `docs/<app>.md` guide and link it from `docs/README.md` and the root `README.md`.

## Commit conventions

- Keep commits focused and write clear messages.
- **Sign off** every commit with the Developer Certificate of Origin:

  ```sh
  git commit -s
  ```

  This appends a `Signed-off-by: Your Name <you@example.com>` trailer.
- Push to a branch and open a pull request; do not push directly to `main`.

## How CI gates pull requests

Pull requests run the workflows under `.github/workflows/` (see its [README](.github/workflows/README.md)):

- **ci.yml** — `go build ./...` and `go test -race ./...` per module in a `fail-fast: false` matrix (each module reports independently), plus a `build-smoke` job.
- **lint.yml** — `golangci-lint` per module, a repo-wide `gofmt -l` gate that fails on any unformatted file, and `go vet ./...` per module.
- **security.yml** — `govulncheck` and `go mod verify` per module, plus CodeQL.

Before pushing, run `gofmt -l .`, `make lint`, `make build-smoke`, and `make test` locally to mirror the gates.

## Releasing

Releases are independent per module and cut with `scripts/version.sh`. See [docs/releasing.md](docs/releasing.md).
