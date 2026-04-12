#!/bin/sh
# Patch lefthook-generated git hooks to find lefthook in GOPATH/bin.
# Lefthook install generates hooks that only check system PATH and node_modules.
# This adds $GOPATH/bin as fallback so "go install lefthook" users work too.

GOPATH_BIN=$(go env GOPATH)/bin
LEFTHOOK="$GOPATH_BIN/lefthook"

if [ ! -f "$LEFTHOOK" ]; then
  echo "patch-hooks: lefthook not found at $LEFTHOOK, skipping patch"
  exit 0
fi

for hook in .git/hooks/pre-commit .git/hooks/pre-push .git/hooks/commit-msg; do
  if [ ! -f "$hook" ]; then
    continue
  fi
  if grep -q "$LEFTHOOK" "$hook"; then
    echo "patch-hooks: $hook already patched"
    continue
  fi
  perl -i -0pe "s{(elif lefthook -h.*?lefthook \"\\\$\@\"\\n)(\\s+else\\b)}{\$1  elif test -f \"$LEFTHOOK\"\\n  then\\n    \"$LEFTHOOK\" \"\\\$\@\"\\n\$2}s" "$hook"
  echo "patch-hooks: patched $hook"
done
