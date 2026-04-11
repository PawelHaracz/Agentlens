# Design: Strip AI Co-Authored-By Lines via commit-msg Hook

**Date:** 2026-04-11  
**Status:** Approved

## Problem

AI coding assistants (Claude, Copilot, Gemini, etc.) inject `Co-Authored-By:` trailers into commit messages. These lines are unwanted in this repository's commit history.

## Solution

Add a `strip-ai-coauthors` command to the existing `commit-msg` hook in `lefthook.yml`. It runs a `perl` one-liner that edits the commit message file in-place, removing any `Co-Authored-By:` line whose name or email matches a known AI agent pattern.

## Matching Rules

A `Co-Authored-By:` trailer is removed if the name or email portion contains any of the following (case-insensitive):

- `claude`
- `copilot`
- `gemini`
- `chatgpt`
- `openai`
- `cursor`

## Implementation

### `lefthook.yml` change

Add `strip-ai-coauthors` as the first command under `commit-msg` (runs before `commitlint`):

```yaml
commit-msg:
  commands:
    strip-ai-coauthors:
      run: perl -i -pe 's/^Co-Authored-By:.*?(claude|copilot|gemini|chatgpt|openai|cursor).*\n?//i' {1}
      priority: 1
    commitlint:
      run: cd web && bunx commitlint --config commitlint.config.ts --edit {1}
      priority: 2
```

`{1}` is lefthook's placeholder for the commit message file path.

## Behaviour

- Runs on every `git commit`.
- Silently removes matching lines — no output to the user.
- Non-matching `Co-Authored-By:` lines (human collaborators) are left untouched.
- `perl -i` works identically on macOS and Linux (no portability issue unlike `sed -i`).
- If no matching lines exist, the file is unchanged and the command exits 0.

## Out of Scope

- Interactive confirmation before removal.
- Logging/auditing removed lines.
- Stripping other AI-generated commit message content (subject, body).
