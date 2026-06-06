#!/usr/bin/env bash
#
# build-smoke.sh - Workspace build smoke test.
#
# The repo root is not a Go module (modules live under apps/ and packages/),
# so `go build ./...` cannot be run from the root. This script discovers every
# module in the workspace (the directory of each go.mod under apps/ and
# packages/) and runs `go build ./...` inside it, asserting that every module
# compiles.
#
# Exits non-zero if any module fails to build, listing the offenders.
#
# Usage:
#   scripts/build-smoke.sh
#
# Requirements: 5.1 (each app/module builds independently), 6.2 (apps import
# shared packages; the whole workspace must compile together).

set -uo pipefail

# Resolve the repository root as the parent of this script's directory so the
# script works regardless of the current working directory.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"

# Discover every module: the directory containing a go.mod under apps/ or
# packages/. Sorted for deterministic output.
modules=()
while IFS= read -r mod; do
	modules+=("${mod}")
done < <(find apps packages -maxdepth 2 -name go.mod -exec dirname {} \; 2>/dev/null | sort)

if [[ ${#modules[@]} -eq 0 ]]; then
	echo "build-smoke: no Go modules found under apps/ or packages/" >&2
	exit 1
fi

echo "build-smoke: checking ${#modules[@]} module(s) in ${REPO_ROOT}"

failed=()
for mod in "${modules[@]}"; do
	echo "→ go build ./...  (${mod})"
	if ! (cd "${mod}" && go build ./...); then
		failed+=("${mod}")
	fi
done

echo
if [[ ${#failed[@]} -ne 0 ]]; then
	echo "build-smoke: FAILED — ${#failed[@]} module(s) did not compile:" >&2
	for mod in "${failed[@]}"; do
		echo "  ✗ ${mod}" >&2
	done
	exit 1
fi

echo "build-smoke: OK — all ${#modules[@]} module(s) compiled successfully"
