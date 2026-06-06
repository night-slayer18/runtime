# Releasing

Every module in the monorepo is versioned and released **independently**. Releasing one app never requires touching another. This guide covers cutting a release with `scripts/version.sh` and what the release pipeline produces.

## Where versions live

Go modules carry their released version in git tags, but tags are awkward to read locally, so each module keeps a `VERSION` file (`apps/<app>/VERSION`, `packages/<pkg>/VERSION`) holding the current semver as the working source of truth. All modules are currently baselined at `v0.1.0`.

See every module's version at a glance:

```sh
make version-list
# or
scripts/version.sh --list
```

## Tag conventions

`scripts/version.sh` creates tags using the per-module convention the release pipeline understands:

| Module | Tag form | Example |
|--------|----------|---------|
| App | `<app>/vX.Y.Z` | `grid/v0.1.0` |
| Package | `packages/<pkg>/vX.Y.Z` | `packages/theme/v0.1.0` |

## Cutting a release

`scripts/version.sh` bumps `VERSION` files and, with `--tag`, creates the matching git tags. Nothing is pushed unless you pass `--push`. Tagging is refused when the working tree is dirty unless you pass `--force`. Use `--dry-run` to preview without writing anything.

`<bump>` is `major`, `minor`, `patch`, or an explicit semver (`1.2.3` or `v1.2.3`).

### Individual release (one module)

```sh
# Preview a patch bump for Grid
scripts/version.sh --module apps/grid patch --dry-run

# Bump, tag, and push Grid
scripts/version.sh --module apps/grid patch --tag --push
# (equivalently, via the Makefile)
make version-bump MODULE=apps/grid BUMP=patch ARGS="--tag --push"
```

Set an explicit version instead of bumping:

```sh
scripts/version.sh --set apps/grid v0.2.0 --tag --push
```

### Coordinated release (many modules)

```sh
# Bump every module together, tag and push
scripts/version.sh --all minor --tag --push
make version-bump-all BUMP=minor ARGS="--tag --push"

# Just the apps, or just the packages
scripts/version.sh --apps patch --tag --push
scripts/version.sh --packages patch --tag --push
```

A bad bump/version spec is rejected before any file is written, so a coordinated `--all` run never half-applies.

## What the release pipeline produces

Pushing a tag triggers `.github/workflows/release.yml`, which parses the tag to decide app vs package:

### App release (`<app>/vX.Y.Z`)

- Cross-compiles that app for all six OS/arch combinations: `linux`, `darwin`, `windows` × `amd64`, `arm64`
- Builds with `CGO_ENABLED=0` and `-trimpath`, stamping the version via `-ldflags -X main.version`
- Packages `tar.gz` (unix) / `zip` (windows) artifacts, named `runtime-<app>-<goos>-<goarch>` (with `.exe` on Windows) — e.g. `runtime-grid-linux-amd64`, `runtime-grid-darwin-arm64`, `runtime-grid-windows-amd64.exe`
- Generates `SHA256SUMS.txt` and a changelog (git log since the previous `<app>/v*` tag)
- Publishes a GitHub Release with the artifacts

### Package release (`packages/<pkg>/vX.Y.Z`)

- Publishes a GitHub Release with a changelog and **no binaries** (a library release)

The cross-platform build matrix itself lives in the reusable `build.yml` workflow, which `release.yml` calls. For the full pipeline description (including `ci.yml`, `lint.yml`, and `security.yml`), see [.github/workflows/README.md](../.github/workflows/README.md).

## Typical flow

1. Land your changes on `main` (CI must be green).
2. Decide the scope: a single module or a coordinated bump.
3. Dry-run the bump: `scripts/version.sh … --dry-run`.
4. Commit the updated `VERSION` file(s).
5. Tag and push: re-run with `--tag --push`.
6. Watch the release workflow publish the GitHub Release.
