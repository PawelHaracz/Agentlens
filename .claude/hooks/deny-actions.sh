INPUT=$(cat)
TOOL=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null)
DENY_FILE="$HOME/.claude/deny-rules.json"
[ -f "$DENY_FILE" ] || exit 0
if [ -n "$FILE" ]; then
    DENIED=$(jq -r '.deny_paths[]' "$DENY_FILE" 2>/dev/null)
    for pattern in $DENIED; do
        if echo "$FILE" | grep -qE "$pattern"; then
            echo "BLOCKED: Access denied to $FILE (matches deny rule: $pattern)" >&2
            exit 2
        fi
    done
fi
if [ -n "$CMD" ]; then
    DENIED=$(jq -r '.deny_commands[]' "$DENY_FILE" 2>/dev/null)
    for pattern in $DENIED; do
        if echo "$CMD" | grep -qE "$pattern"; then
            echo "BLOCKED: Command denied (matches: $pattern)" >&2
            exit 2
        fi
    done
fi
exit 0