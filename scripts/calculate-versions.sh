#!/usr/bin/env bash
set -euo pipefail

# ── Usage ──────────────────────────────────────────────────────────────────
# ./scripts/calculate-versions.sh [--app-override VERSION] [--helm-override VERSION]
#
# Outputs (GITHUB_OUTPUT when in CI, stdout otherwise):
#   app_version   — next app semver (e.g. 1.2.3)
#   helm_version  — next helm semver (e.g. 0.3.0)
#   release_app   — true/false
#   release_helm  — true/false

# ── Paths that trigger each track ──────────────────────────────────────────
APP_PATHS=("cmd/" "internal/" "plugins/" "web/" "Dockerfile" "go.mod" "go.sum")
HELM_PATHS=("deploy/helm/")

# ── Parse arguments ────────────────────────────────────────────────────────
APP_OVERRIDE=""
HELM_OVERRIDE=""

# ── Helper: validate semver format ─────────────────────────────────────────
validate_semver() {
  [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid semver: $1" >&2; exit 1; }
}

while [[ $# -gt 0 ]]; do
  case $1 in
    --app-override)  validate_semver "$2"; APP_OVERRIDE="$2"; shift 2 ;;
    --helm-override) validate_semver "$2"; HELM_OVERRIDE="$2"; shift 2 ;;
    -h|--help) head -11 "$0" | tail -8; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

# ── Helper: emit a key=value pair ──────────────────────────────────────────
emit() {
  local key="$1" value="$2"
  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "${key}=${value}" >> "$GITHUB_OUTPUT"
  fi
  echo "${key}=${value}"
}

# ── Helper: get latest tag matching a pattern ──────────────────────────────
latest_tag() {
  local pattern="$1"
  git tag --list | grep -E "$pattern" | sort -V | tail -1 || true
}

# ── Helper: determine bump level from conventional commits ─────────────────
# Reads commit messages between $1..HEAD. Returns: major, minor, or patch.
determine_bump() {
  local since="$1"
  local range
  if [[ -z "$since" ]]; then
    range="HEAD"
  else
    range="${since}..HEAD"
  fi

  local bump="patch"
  while IFS= read -r msg; do
    # BREAKING CHANGE in body/footer or ! after type
    if echo "$msg" | grep -qiE '^[a-z]+(\(.+\))?!:|BREAKING CHANGE'; then
      echo "major"
      return
    fi
    if echo "$msg" | grep -qE '^feat(\(.+\))?:'; then
      bump="minor"
    fi
  done < <(git log --format='%B' "$range" 2>/dev/null)

  echo "$bump"
}

# ── Helper: bump a semver string ───────────────────────────────────────────
# Usage: bump_semver "1.2.3" "minor" → "1.3.0"
bump_semver() {
  local version="$1" level="$2"
  if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "bump_semver: invalid semver '${version}'" >&2; exit 1
  fi
  IFS='.' read -r major minor patch <<< "$version"
  case "$level" in
    major) echo "$((major + 1)).0.0" ;;
    minor) echo "${major}.$((minor + 1)).0" ;;
    patch) echo "${major}.${minor}.$((patch + 1))" ;;
  esac
}

# ── Helper: check if paths changed since a tag ────────────────────────────
# Usage: paths_changed "v0.1.0" "cmd/" "internal/" ...
# Returns 0 (true) if any path has changes, 1 (false) otherwise.
paths_changed() {
  local since="$1"; shift
  local paths=("$@")
  local range

  if [[ -z "$since" ]]; then
    # No previous tag — everything is new
    return 0
  fi

  range="${since}..HEAD"
  for p in "${paths[@]}"; do
    if git diff --name-only "$range" -- "$p" | grep -q .; then
      return 0
    fi
  done
  return 1
}

# ══════════════════════════════════════════════════════════════════════════════
# Main
# ══════════════════════════════════════════════════════════════════════════════

# ── App version ────────────────────────────────────────────────────────────
LATEST_APP_TAG=$(latest_tag '^v[0-9]+\.[0-9]+\.[0-9]+$')

if [[ -n "$APP_OVERRIDE" ]]; then
  APP_VERSION="$APP_OVERRIDE"
  RELEASE_APP=true
elif [[ -z "$LATEST_APP_TAG" ]]; then
  APP_VERSION="0.1.0"
  RELEASE_APP=true
elif paths_changed "$LATEST_APP_TAG" "${APP_PATHS[@]}"; then
  BUMP=$(determine_bump "$LATEST_APP_TAG")
  APP_VERSION=$(bump_semver "${LATEST_APP_TAG#v}" "$BUMP")
  RELEASE_APP=true
else
  APP_VERSION="${LATEST_APP_TAG#v}"
  RELEASE_APP=false
fi

# ── Helm version ───────────────────────────────────────────────────────────
LATEST_HELM_TAG=$(latest_tag '^helm/v[0-9]+\.[0-9]+\.[0-9]+$')

if [[ -n "$HELM_OVERRIDE" ]]; then
  HELM_VERSION="$HELM_OVERRIDE"
  RELEASE_HELM=true
elif [[ -z "$LATEST_HELM_TAG" ]]; then
  HELM_VERSION="0.1.0"
  RELEASE_HELM=true
elif paths_changed "$LATEST_HELM_TAG" "${HELM_PATHS[@]}"; then
  BUMP=$(determine_bump "$LATEST_HELM_TAG")
  HELM_VERSION=$(bump_semver "${LATEST_HELM_TAG#helm/v}" "$BUMP")
  RELEASE_HELM=true
else
  HELM_VERSION="${LATEST_HELM_TAG#helm/v}"
  RELEASE_HELM=false
fi

# ── Pre-release suffix (feature branches) ──────────────────────────────────
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
if [[ "$BRANCH" != "main" && "$BRANCH" != "master" ]]; then
  SHORT_SHA=$(git rev-parse --short HEAD)
  APP_VERSION="${APP_VERSION}-${SHORT_SHA}"
  HELM_VERSION="${HELM_VERSION}-${SHORT_SHA}"
fi

# ── Output ─────────────────────────────────────────────────────────────────
emit "app_version"  "$APP_VERSION"
emit "helm_version" "$HELM_VERSION"
emit "release_app"  "$RELEASE_APP"
emit "release_helm" "$RELEASE_HELM"

if [[ -n "${GITHUB_ACTIONS:-}" ]]; then
  echo "::notice::App version  : ${APP_VERSION}  (release=${RELEASE_APP})"
  echo "::notice::Helm version : ${HELM_VERSION}  (release=${RELEASE_HELM})"
fi
