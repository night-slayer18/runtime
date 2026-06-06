#!/usr/bin/env bash
#
# version.sh - Centralized version management for the Runtime monorepo.
#
# The repo is a Go monorepo of independently versioned/released modules: five
# applications under apps/{grid,prism,pulse,strata,vault} and shared packages
# under packages/{tui,theme,config,...}. Each is its own Go module.
#
# Go modules carry their released version in git tags (not a file), but tags
# are awkward to read/diff locally, so this script keeps a VERSION file per
# module (apps/<app>/VERSION, packages/<pkg>/VERSION) holding the current
# semver as the working source of truth, and can create the matching git tag
# on demand. Tag form follows the per-module convention described in design.md:
#   - apps:     <app>/vX.Y.Z              (e.g. grid/v0.1.0)
#   - packages: packages/<pkg>/vX.Y.Z     (e.g. packages/theme/v0.1.0)
#
# Versioning supports both INDEPENDENT releases (a single module) and a
# COORDINATED/common release (every module bumped together).
#
# Usage:
#   scripts/version.sh --list
#   scripts/version.sh --all <bump> [--tag] [--push] [--dry-run] [--force]
#   scripts/version.sh --apps <bump> [...]
#   scripts/version.sh --packages <bump> [...]
#   scripts/version.sh --module <path> <bump> [...]
#   scripts/version.sh --set <path> <version> [...]
#   scripts/version.sh -h | --help
#
#   <bump> is one of: major | minor | patch | an explicit semver (1.2.3 or v1.2.3)
#
# Flags:
#   --tag       After bumping, create the per-module git tag(s).
#   --push      Push the created tags (only meaningful with --tag).
#   --dry-run   Print what would change; write no files and create no tags.
#   --force     Allow tagging even when the working tree is dirty.
#   -h, --help  Show this help.
#
# Safety: nothing is pushed unless --push is given explicitly. Tagging is
# refused when the working tree is dirty unless --force is supplied.
#
# Requirements: 5 (independent versioning, changelogs, binaries, release
# pipelines per app) plus a coordinated/common release for the whole repo.

set -uo pipefail

# Resolve the repository root as the parent of this script's directory so the
# script works regardless of the current working directory.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

readonly APPS=(grid prism pulse strata vault)
readonly PACKAGES=(tui theme config table tree search plugin datasource export schema validation)
readonly DEFAULT_VERSION="v0.0.0"

# ----------------------------------------------------------------------------
# Output helpers
# ----------------------------------------------------------------------------
err()  { echo "version: $*" >&2; }
die()  { err "$*"; exit 1; }

usage() {
	# Strip the leading "# " from the header comment block for help output.
	sed -n '3,40p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

# ----------------------------------------------------------------------------
# Module helpers
# ----------------------------------------------------------------------------

# all_modules: print every module path (apps then packages), one per line.
all_modules() {
	local a p
	for a in "${APPS[@]}";     do echo "apps/${a}"; done
	for p in "${PACKAGES[@]}"; do echo "packages/${p}"; done
}

# is_known_module <path>: succeed if <path> is a valid module path.
is_known_module() {
	local target="$1" m
	while IFS= read -r m; do
		[[ "${m}" == "${target}" ]] && return 0
	done < <(all_modules)
	return 1
}

# version_file <module-path>: path to that module's VERSION file.
version_file() { echo "${REPO_ROOT}/$1/VERSION"; }

# tag_name <module-path> <version>: per-module git tag for a module + version.
#   apps/grid      v0.1.0 -> grid/v0.1.0
#   packages/theme v0.1.0 -> packages/theme/v0.1.0
tag_name() {
	local path="$1" version="$2"
	case "${path}" in
		apps/*)     echo "${path#apps/}/${version}" ;;
		packages/*) echo "${path}/${version}" ;;
		*)          die "cannot derive tag for unknown module path: ${path}" ;;
	esac
}

# read_version <module-path>: print the current version, defaulting (and
# creating, unless dry-run) to v0.0.0 when the VERSION file is missing.
read_version() {
	local path="$1" file
	file="$(version_file "${path}")"
	if [[ -f "${file}" ]]; then
		local v
		v="$(tr -d '[:space:]' < "${file}")"
		[[ -n "${v}" ]] && { echo "${v}"; return 0; }
	fi
	echo "${DEFAULT_VERSION}"
}

# ----------------------------------------------------------------------------
# Semver helpers
# ----------------------------------------------------------------------------

# normalize_version <ver>: validate ^v?MAJOR.MINOR.PATCH$ and echo with a
# leading "v". Exits non-zero (with message) on invalid input.
normalize_version() {
	local raw="$1"
	if [[ "${raw}" =~ ^v?([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
		echo "v${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.${BASH_REMATCH[3]}"
		return 0
	fi
	return 1
}

# validate_bump_spec <bump>: succeed if <bump> is major|minor|patch or a
# normalizable semver. Used to fail fast BEFORE any files are written (so a
# bad value never leaves a coordinated --all run half-applied).
validate_bump_spec() {
	case "$1" in
		major|minor|patch) return 0 ;;
		*) normalize_version "$1" >/dev/null ;;
	esac
}

# bump_version <current> <bump>: echo the new version. <bump> is one of
# major|minor|patch or an explicit semver. The spec is assumed pre-validated
# (see validate_bump_spec); a corrupt current version is reported here.
#
# NOTE: this is called via $(...) so it must not rely on die() to abort the
# whole script — a non-zero return propagates to the caller, which checks it.
bump_version() {
	local current="$1" bump="$2" major minor patch
	if [[ "${current}" =~ ^v?([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
		major="${BASH_REMATCH[1]}"; minor="${BASH_REMATCH[2]}"; patch="${BASH_REMATCH[3]}"
	else
		err "cannot parse current version: ${current}"
		return 1
	fi

	case "${bump}" in
		major) echo "v$((major + 1)).0.0" ;;
		minor) echo "v${major}.$((minor + 1)).0" ;;
		patch) echo "v${major}.${minor}.$((patch + 1))" ;;
		*)     normalize_version "${bump}" ;;
	esac
}

# ----------------------------------------------------------------------------
# Git helpers
# ----------------------------------------------------------------------------

working_tree_dirty() {
	# True (0) when there are uncommitted changes.
	[[ -n "$(git status --porcelain 2>/dev/null)" ]]
}

# ----------------------------------------------------------------------------
# Actions
# ----------------------------------------------------------------------------

DRY_RUN=0
DO_TAG=0
DO_PUSH=0
FORCE=0

# write_version <module-path> <new-version>: persist (or, in dry-run, just
# report) the new version into the module's VERSION file.
write_version() {
	local path="$1" newv="$2" file
	file="$(version_file "${path}")"
	if [[ ${DRY_RUN} -eq 1 ]]; then
		return 0
	fi
	echo "${newv}" > "${file}"
}

# CHANGED_PATHS / CHANGED_VERSIONS accumulate applied changes for tagging and
# the final summary. Parallel arrays keyed by index.
CHANGED_PATHS=()
CHANGED_VERSIONS=()

# apply_bump <module-path> <bump>: compute + record + write the new version.
apply_bump() {
	local path="$1" bump="$2" current newv
	current="$(read_version "${path}")"
	if ! newv="$(bump_version "${current}" "${bump}")"; then
		die "could not bump ${path} (${current}) with '${bump}'"
	fi
	printf '  %-22s %s -> %s\n' "${path}" "${current}" "${newv}"
	write_version "${path}" "${newv}"
	CHANGED_PATHS+=("${path}")
	CHANGED_VERSIONS+=("${newv}")
}

# apply_set <module-path> <version>: set an explicit (normalized) version.
apply_set() {
	local path="$1" version="$2" current newv
	current="$(read_version "${path}")"
	if ! newv="$(normalize_version "${version}")"; then
		die "invalid version '${version}': expected a semver like 1.2.3 or v1.2.3"
	fi
	printf '  %-22s %s -> %s\n' "${path}" "${current}" "${newv}"
	write_version "${path}" "${newv}"
	CHANGED_PATHS+=("${path}")
	CHANGED_VERSIONS+=("${newv}")
}

# create_tags: tag every recorded change using the per-module convention.
create_tags() {
	[[ ${#CHANGED_PATHS[@]} -eq 0 ]] && { err "no changes to tag"; return 0; }

	if working_tree_dirty; then
		if [[ ${DRY_RUN} -eq 1 ]]; then
			err "NOTE: working tree is dirty; a real run would refuse to tag without --force"
		elif [[ ${FORCE} -eq 1 ]]; then
			err "WARNING: working tree is dirty; tagging anyway because --force was given"
		else
			die "refusing to tag: working tree is dirty (commit/stash changes, or pass --force)"
		fi
	fi

	local created=()
	local i path version tag
	for i in "${!CHANGED_PATHS[@]}"; do
		path="${CHANGED_PATHS[$i]}"
		version="${CHANGED_VERSIONS[$i]}"
		tag="$(tag_name "${path}" "${version}")"
		if [[ ${DRY_RUN} -eq 1 ]]; then
			echo "  would tag: ${tag}"
		else
			echo "  tagging: ${tag}"
			git tag -a "${tag}" -m "Release ${tag}" || die "failed to create tag ${tag}"
		fi
		created+=("${tag}")
	done

	if [[ ${DO_PUSH} -eq 1 ]]; then
		if [[ ${DRY_RUN} -eq 1 ]]; then
			echo "  would push ${#created[@]} tag(s) to origin"
		else
			echo "  pushing ${#created[@]} tag(s) to origin"
			git push origin "${created[@]}" || die "failed to push tags"
		fi
	fi
}

# list_versions: print every module and its current version.
list_versions() {
	local m v
	echo "Runtime modules:"
	while IFS= read -r m; do
		v="$(read_version "${m}")"
		printf '  %-22s %s\n' "${m}" "${v}"
	done < <(all_modules)
}

# ----------------------------------------------------------------------------
# Summary
# ----------------------------------------------------------------------------
print_summary() {
	echo
	if [[ ${#CHANGED_PATHS[@]} -eq 0 ]]; then
		echo "Summary: no version changes."
		return 0
	fi
	local label="applied"
	[[ ${DRY_RUN} -eq 1 ]] && label="planned (dry-run, nothing written)"
	echo "Summary: ${#CHANGED_PATHS[@]} module(s) ${label}."
	local i
	for i in "${!CHANGED_PATHS[@]}"; do
		printf '  %-22s -> %s\n' "${CHANGED_PATHS[$i]}" "${CHANGED_VERSIONS[$i]}"
	done
	if [[ ${DO_TAG} -eq 0 ]]; then
		echo "  (tags not created; re-run with --tag to create git tags)"
	fi
}

# ----------------------------------------------------------------------------
# Argument parsing
# ----------------------------------------------------------------------------
# We use a small hand-rolled parser so we can support the mixed positional
# subcommand styles (--module <path> <bump>, --set <path> <version>) alongside
# plain flags cleanly.

MODE=""            # list | all | apps | packages | module | set
BUMP=""            # bump spec or explicit version
TARGET_PATH=""     # module path for --module / --set

set_mode() {
	[[ -n "${MODE}" ]] && die "conflicting actions: --${MODE} and $1 cannot be combined"
	MODE="$2"
}

[[ $# -eq 0 ]] && { usage; exit 1; }

while [[ $# -gt 0 ]]; do
	case "$1" in
		-h|--help)
			usage; exit 0 ;;
		--list)
			set_mode "--list" "list"; shift ;;
		--all)
			set_mode "--all" "all"
			[[ $# -ge 2 ]] || die "--all requires a <bump> argument"
			BUMP="$2"; shift 2 ;;
		--apps)
			set_mode "--apps" "apps"
			[[ $# -ge 2 ]] || die "--apps requires a <bump> argument"
			BUMP="$2"; shift 2 ;;
		--packages)
			set_mode "--packages" "packages"
			[[ $# -ge 2 ]] || die "--packages requires a <bump> argument"
			BUMP="$2"; shift 2 ;;
		--module)
			set_mode "--module" "module"
			[[ $# -ge 3 ]] || die "--module requires <path> <bump> arguments"
			TARGET_PATH="$2"; BUMP="$3"; shift 3 ;;
		--set)
			set_mode "--set" "set"
			[[ $# -ge 3 ]] || die "--set requires <path> <version> arguments"
			TARGET_PATH="$2"; BUMP="$3"; shift 3 ;;
		--tag)
			DO_TAG=1; shift ;;
		--push)
			DO_PUSH=1; shift ;;
		--dry-run)
			DRY_RUN=1; shift ;;
		--force)
			FORCE=1; shift ;;
		*)
			die "unknown argument: $1 (try --help)" ;;
	esac
done

[[ -n "${MODE}" ]] || die "no action specified (try --help)"
if [[ ${DO_PUSH} -eq 1 && ${DO_TAG} -eq 0 ]]; then
	die "--push is only meaningful together with --tag"
fi

# Fail fast on a bad bump/version spec so a coordinated run never half-applies.
case "${MODE}" in
	all|apps|packages|module)
		validate_bump_spec "${BUMP}" \
			|| die "invalid bump '${BUMP}': expected major|minor|patch or a semver like 1.2.3" ;;
	set)
		normalize_version "${BUMP}" >/dev/null \
			|| die "invalid version '${BUMP}': expected a semver like 1.2.3 or v1.2.3" ;;
esac

# ----------------------------------------------------------------------------
# Dispatch
# ----------------------------------------------------------------------------
case "${MODE}" in
	list)
		list_versions
		exit 0 ;;
	all)
		echo "Bumping ALL modules (${BUMP}):"
		while IFS= read -r m; do apply_bump "${m}" "${BUMP}"; done < <(all_modules)
		;;
	apps)
		echo "Bumping APPS (${BUMP}):"
		for a in "${APPS[@]}"; do apply_bump "apps/${a}" "${BUMP}"; done
		;;
	packages)
		echo "Bumping PACKAGES (${BUMP}):"
		for p in "${PACKAGES[@]}"; do apply_bump "packages/${p}" "${BUMP}"; done
		;;
	module)
		is_known_module "${TARGET_PATH}" || die "unknown module path: ${TARGET_PATH}"
		echo "Bumping module ${TARGET_PATH} (${BUMP}):"
		apply_bump "${TARGET_PATH}" "${BUMP}"
		;;
	set)
		is_known_module "${TARGET_PATH}" || die "unknown module path: ${TARGET_PATH}"
		echo "Setting module ${TARGET_PATH} to ${BUMP}:"
		apply_set "${TARGET_PATH}" "${BUMP}"
		;;
	*)
		die "internal error: unhandled mode '${MODE}'" ;;
esac

if [[ ${DO_TAG} -eq 1 ]]; then
	echo
	echo "Tags:"
	create_tags
fi

print_summary
