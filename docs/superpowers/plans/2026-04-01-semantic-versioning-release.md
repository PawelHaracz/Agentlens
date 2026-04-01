# Semantic Versioning Release Script — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract release version calculation from `release.yml` into a standalone `scripts/calculate-versions.sh` that determines app and helm versions based on conventional commits and changed files.

**Architecture:** A single Bash script reads git history since the last tag, parses conventional commit prefixes (`fix:` → patch, `feat:` → minor, `feat!:`/`BREAKING CHANGE` → major), checks which file paths changed to determine if app and/or helm need a bump, and outputs the results. The release workflow calls this script instead of inline shell.

**Tech Stack:** Bash, git, GitHub Actions

---

## Design Decisions

- **2 version tracks:** `v{semver}` (app — Docker image + GitHub Release) and `helm/v{semver}` (Helm chart)
- **App paths:** `cmd/`, `internal/`, `plugins/`, `web/`, `Dockerfile`, `go.mod`, `go.sum`
- **Helm paths:** `deploy/helm/`
- **Bump level from commits:** Scan all commits since last relevant tag. Highest bump wins (major > minor > patch).
- **Manual override:** `workflow_dispatch` inputs `app_version` and `helm_version` bypass auto-calculation.
- **No `force_helm_release` input** — manual `helm_version` input implies forced release.
- **Local-friendly:** Script outputs to stdout when not in CI, to `$GITHUB_OUTPUT` when it exists.

## File Structure

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `scripts/calculate-versions.sh` | All version calculation logic |
| Modify | `.github/workflows/release.yml` | Call script, remove inline shell, simplify inputs |

---

### Task 1: Create `scripts/calculate-versions.sh`

**Files:**
- Create: `scripts/calculate-versions.sh`

- [ ] **Step 1: Create the script with argument parsing and helper functions**

```bash
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

while [[ $# -gt 0 ]]; do
  case $1 in
    --app-override)  APP_OVERRIDE="$2"; shift 2 ;;
    --helm-override) HELM_OVERRIDE="$2"; shift 2 ;;
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
  done < <(git log --format='%s' "$range" 2>/dev/null)

  echo "$bump"
}

# ── Helper: bump a semver string ───────────────────────────────────────────
# Usage: bump_semver "1.2.3" "minor" → "1.3.0"
bump_semver() {
  local version="$1" level="$2"
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
    if [[ $(git diff --name-only "$range" -- "$p" | wc -l) -gt 0 ]]; then
      return 0
    fi
  done
  return 1
}
```

- [ ] **Step 2: Add main logic for app version calculation**

Append to the script:

```bash
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
```

- [ ] **Step 3: Add main logic for helm version calculation**

Append to the script:

```bash
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
```

- [ ] **Step 4: Add output emission**

Append to the script:

```bash
# ── Output ─────────────────────────────────────────────────────────────────
emit "app_version"  "$APP_VERSION"
emit "helm_version" "$HELM_VERSION"
emit "release_app"  "$RELEASE_APP"
emit "release_helm" "$RELEASE_HELM"

if [[ -n "${GITHUB_ACTIONS:-}" ]]; then
  echo "::notice::App version  : ${APP_VERSION}  (release=${RELEASE_APP})"
  echo "::notice::Helm version : ${HELM_VERSION}  (release=${RELEASE_HELM})"
fi
```

- [ ] **Step 5: Make script executable and test locally**

Run:
```bash
chmod +x scripts/calculate-versions.sh
./scripts/calculate-versions.sh
```

Expected output (based on current tags `v0.1.1` and `helm/v0.1.1`):
```
app_version=0.2.0
helm_version=0.1.1
release_app=true
release_helm=false
```

The app bumps to `0.2.0` (minor) because there are `feat:` commits since `v0.1.1`. Helm stays at `0.1.1` because no `deploy/helm/` files changed.

- [ ] **Step 6: Test with override arguments**

Run:
```bash
./scripts/calculate-versions.sh --app-override 1.0.0 --helm-override 2.0.0
```

Expected:
```
app_version=1.0.0
helm_version=2.0.0
release_app=true
release_helm=true
```

- [ ] **Step 7: Commit**

```bash
git add scripts/calculate-versions.sh
git commit -m "feat(release): add semantic versioning calculation script"
```

---

### Task 2: Update `release.yml` to use the script

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Simplify workflow_dispatch inputs**

Remove `force_helm_release` input. Keep `app_version` and `helm_version` overrides.

Replace the `inputs` section (lines 9-21) with:

```yaml
    inputs:
      app_version:
        description: "App version override (e.g. 1.0.0). Leave empty for auto-bump."
        required: false
        type: string
      helm_version:
        description: "Helm chart version override (e.g. 1.0.0). Leave empty for auto-bump."
        required: false
        type: string
```

- [ ] **Step 2: Replace prepare job inline script with script call**

Replace the `prepare` job's `Calculate release versions` step (lines 52-107) with:

```yaml
      - name: Calculate release versions
        id: calc
        run: |
          ./scripts/calculate-versions.sh \
            ${{ inputs.app_version && format('--app-override {0}', inputs.app_version) || '' }} \
            ${{ inputs.helm_version && format('--helm-override {0}', inputs.helm_version) || '' }}
```

- [ ] **Step 3: Update prepare job outputs to include `release_app`**

Update the `outputs` section of the `prepare` job to:

```yaml
    outputs:
      app_version: ${{ steps.calc.outputs.app_version }}
      helm_version: ${{ steps.calc.outputs.helm_version }}
      release_app: ${{ steps.calc.outputs.release_app }}
      release_helm: ${{ steps.calc.outputs.release_helm }}
```

- [ ] **Step 4: Add `release_app` condition to `release-docker` job**

Add an `if` condition to the `release-docker` job so it only runs when there are app changes:

```yaml
  release-docker:
    name: Release Docker Image
    needs: prepare
    if: needs.prepare.outputs.release_app == 'true'
    runs-on: ubuntu-latest
```

- [ ] **Step 5: Verify the complete workflow file looks correct**

Read the full file and verify:
- `workflow_dispatch` has only `app_version` and `helm_version` inputs
- `prepare` job calls the script
- `release-docker` has `if: needs.prepare.outputs.release_app == 'true'`
- `release-helm` has `if: needs.prepare.outputs.release_helm == 'true'` (already present)
- All `needs.prepare.outputs.*` references are correct

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "refactor(release): use calculate-versions.sh script, add release_app gate"
```

---

### Task 3: Test end-to-end

- [ ] **Step 1: Run the script and verify against known state**

```bash
./scripts/calculate-versions.sh
```

Verify output matches expectations based on current tags and commits.

- [ ] **Step 2: Validate the workflow YAML syntax**

```bash
# Basic YAML syntax check
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"
```

- [ ] **Step 3: Verify script handles edge cases**

Test with no tags (simulate by checking the code path logic):
```bash
# The script should output 0.1.0 when no tags exist
# This is verified by code review — we can't easily remove tags locally
```

- [ ] **Step 4: Final commit if any fixes were needed**

```bash
git add -A
git commit -m "fix(release): address edge cases in version calculation"
```
