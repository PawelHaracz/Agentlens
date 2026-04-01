#!/bin/bash
# git-safe: PreToolUse hook for Claude Code
# Enforces git safety rules as a hook because built-in deny rules
# have known bypass vectors (multiline, heredoc, compound commands).
# See: https://github.com/anthropics/claude-code/issues/30519
#
# Inspired by: https://github.com/Bande-a-Bonnot/Boucle-framework/tree/main/tools/git-safe

set -euo pipefail

# LOG="$HOME/Library/Logs/Claude/hooks-git-safe.log"
log() { [ -n "${LOG:-}" ] && echo "$(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG" || true; }

input=$(cat)

tool_name=$(echo "$input" | jq -r '.tool_name // empty')
[ "$tool_name" != "Bash" ] && exit 0

cmd=$(echo "$input" | jq -r '.tool_input.command // empty')
[ -z "$cmd" ] && exit 0
echo "$cmd" | grep -q 'git\b' 2>/dev/null || exit 0

log "evaluating: $cmd"

block() {
  local reason="$1"
  log "BLOCK: $reason"
  echo "{\"decision\":\"block\",\"reason\":\"git-safe: $reason\"}"
  exit 0
}

ask() {
  local reason="$1"
  log "ASK: $reason"
  echo "{\"decision\":\"ask\",\"message\":\"git-safe: $reason\"}"
  exit 0
}

# Block destructive operations
if echo "$cmd" | grep -qE 'git\s+push\s.*--force' 2>/dev/null; then
  block "Force push can rewrite remote history and lose commits."
fi
if echo "$cmd" | grep -qE 'git\s+push\s+(-[a-zA-Z]*f\b|.*\s-[a-zA-Z]*f\b)' 2>/dev/null; then
  block "Force push (-f) can rewrite remote history."
fi
if echo "$cmd" | grep -qE 'git\s+reset\s.*--hard' 2>/dev/null; then
  block "git reset --hard discards all uncommitted changes permanently."
fi
if echo "$cmd" | grep -qE 'git\s+checkout\s+\.\s*$' 2>/dev/null; then
  block "git checkout . discards all uncommitted changes."
fi
if echo "$cmd" | grep -qE 'git\s+checkout\s+--\s' 2>/dev/null; then
  block "git checkout -- discards uncommitted changes to specified files."
fi
if echo "$cmd" | grep -qE 'git\s+restore\s+\.\s*$' 2>/dev/null; then
  block "git restore . discards all uncommitted changes."
fi
if echo "$cmd" | grep -qE 'git\s+clean\s.*-[a-zA-Z]*f' 2>/dev/null; then
  block "git clean -f permanently deletes untracked files."
fi
if echo "$cmd" | grep -qE 'git\s+branch\s.*-[a-zA-Z]*D' 2>/dev/null; then
  block "git branch -D force-deletes a branch even if not fully merged."
fi
if echo "$cmd" | grep -qE 'git\s+stash\s+drop' 2>/dev/null; then
  block "git stash drop permanently deletes stashed changes."
fi
if echo "$cmd" | grep -qE 'git\s+stash\s+clear' 2>/dev/null; then
  block "git stash clear permanently deletes all stashed changes."
fi
if echo "$cmd" | grep -qE 'git\s+reflog\s+(expire|delete)' 2>/dev/null; then
  block "git reflog expire/delete destroys recovery data."
fi
if echo "$cmd" | grep -qE 'git\s+push(\s|$)' 2>/dev/null; then
  block "git push is not allowed-- use MR workflow"
fi

# Ask for mutating operations
if echo "$cmd" | grep -qE 'git\s+add(\s|$)' 2>/dev/null; then
  ask "git add — confirm staging changes"
fi
if echo "$cmd" | grep -qE 'git\s+rm(\s|$)' 2>/dev/null; then
  ask "git rm — confirm staging removals"
fi
if echo "$cmd" | grep -qE 'git\s+commit(\s|$)' 2>/dev/null; then
  ask "git commit — confirm committing"
fi
log "pass: no rules matched"
exit 0