# CI/CD Workflows

Automated build, test, security, and release pipelines for the Runtime Go
monorepo. The repo root is **not** a Go module: each app under `apps/*` and each
package under `packages/*` is its own module, so every job operates per-module.
All jobs pin Go `1.25.x` to match the modules' `go 1.25.0` directive.

## Workflows

| File | Trigger | Purpose |
|------|---------|---------|
| `ci.yml` | push to `main`, pull_request | Dynamically discovers all module dirs and runs `go build ./...` + `go test -race ./...` per module in a `fail-fast: false` matrix (so each module reports independently — Requirement 11.2), plus a `build-smoke` job running `scripts/build-smoke.sh`. Caches the Go build/module cache. |
| `lint.yml` | push to `main`, pull_request | `golangci-lint` per module via the official action, a repo-wide `gofmt -l` gate that fails on any unformatted file, and `go vet ./...` per module. |
| `security.yml` | push, pull_request, weekly cron (Mon 06:00 UTC) | `govulncheck ./...` per module, `go mod verify` per module, and CodeQL static analysis for Go. |
| `build.yml` | `workflow_call`, `workflow_dispatch` | Reusable cross-platform build matrix (GOOS linux/darwin/windows × GOARCH amd64/arm64) building each app with `CGO_ENABLED=0 -trimpath` and uploading binaries as artifacts. Reused by `release.yml`. |
| `release.yml` | tag push (see below) | Parses the tag to determine app vs package release, then either cross-compiles + packages + checksums + publishes an app release, or publishes a binary-free library release. |

## Release tag formats

Releases are independent per module — releasing one app never requires another.

| Tag pushed | Detected as | Result |
|------------|-------------|--------|
| `grid/v1.2.3` (also `prism`, `pulse`, `strata`, `vault`) | **App release** | Cross-compiles that app for all 6 OS/arch combos (`CGO_ENABLED=0`, `-trimpath`, version stamped via `-ldflags -X main.version`), packages `tar.gz` (unix) / `zip` (windows), generates `SHA256SUMS.txt` and a changelog (git log since the previous `<app>/v*` tag), and publishes a GitHub Release with the artifacts. |
| `packages/theme/v0.2.0` (any `packages/<pkg>/vX.Y.Z`) | **Package release** | Publishes a GitHub Release with a changelog and **no binaries** (library release). |

Artifacts are named `runtime-<app>-<goos>-<goarch>` (with `.exe` on Windows),
e.g. `runtime-grid-linux-amd64`, `runtime-grid-darwin-arm64`,
`runtime-grid-windows-amd64.exe`.

Tags are produced by `scripts/version.sh ... --tag --push` (see the script
header for the bump/coordinated-release options).

## Dependabot

`.github/dependabot.yml` keeps every Go module (`gomod`, one entry per module
directory) and the GitHub Actions (`github-actions`) up to date on a weekly
schedule.
