# Strip AI Co-Authored-By Lines Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `commit-msg` lefthook command that silently removes `Co-Authored-By:` trailers from AI agents (Claude, Copilot, Gemini, ChatGPT, OpenAI, Cursor) before commitlint runs.

**Architecture:** A single `perl -i -pe` one-liner is added as a new `strip-ai-coauthors` command in the `commit-msg` section of `lefthook.yml`, with `priority: 1` so it runs before the existing `commitlint` command (`priority: 2`). No new files are created.

**Tech Stack:** lefthook, perl (available on macOS and Linux by default)

---

### Task 1: Add strip-ai-coauthors command to lefthook.yml

**Files:**
- Modify: `lefthook.yml`

- [ ] **Step 1: Open lefthook.yml and read current state**

Current content of `lefthook.yml`:
```yaml
pre-commit:
  parallel: true
  commands:
    go-fmt:
      run: test -z "$(gofmt -l .)"
      fail_text: "Go files need formatting. Run: make format"
    go-lint:
      run: golangci-lint run
    web-lint:
      run: cd web && bun run type-check

commit-msg:
  commands:
    commitlint:
      run: cd web && bunx commitlint --config commitlint.config.ts --edit {1}

pre-push:
  commands:
    go-test:
      run: make test
      priority: 1
    web-test:
      run: make web-test
      priority: 1
    arch-test:
      run: make arch-test
      priority: 2
```

- [ ] **Step 2: Replace the commit-msg section**

Replace the `commit-msg` block with:
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

- [ ] **Step 3: Verify the file looks correct**

Run:
```bash
cat lefthook.yml
```

Expected output — the `commit-msg` section should read:
```
commit-msg:
  commands:
    strip-ai-coauthors:
      run: perl -i -pe 's/^Co-Authored-By:.*?(claude|copilot|gemini|chatgpt|openai|cursor).*\n?//i' {1}
      priority: 1
    commitlint:
      run: cd web && bunx commitlint --config commitlint.config.ts --edit {1}
      priority: 2
```

- [ ] **Step 4: Manually test the perl command**

Create a test commit message file and run the perl command directly:
```bash
cat > /tmp/test-commit-msg.txt << 'EOF'
feat: add something

Co-Authored-By: Claude <claude@anthropic.com>
Co-Authored-By: Alice Smith <alice@example.com>
Co-Authored-By: GitHub Copilot <copilot@github.com>
EOF

perl -i -pe 's/^Co-Authored-By:.*?(claude|copilot|gemini|chatgpt|openai|cursor).*\n?//i' /tmp/test-commit-msg.txt

cat /tmp/test-commit-msg.txt
```

Expected output — AI lines removed, human line kept:
```
feat: add something

Co-Authored-By: Alice Smith <alice@example.com>
```

- [ ] **Step 5: Commit**

```bash
git add lefthook.yml
git commit -m "chore: strip AI Co-Authored-By trailers in commit-msg hook"
```

---

### Task 2: Reinstall hooks so lefthook picks up the change

**Files:**
- No file changes — hook reinstall only

- [ ] **Step 1: Reinstall lefthook hooks**

```bash
make hooks
```

Expected output: lefthook prints something like:
```
SYNCING
  commit-msg
  pre-commit
  pre-push
```
(exact output varies by lefthook version; exit code 0 is what matters)

- [ ] **Step 2: Verify the commit-msg hook script was updated**

```bash
cat .git/hooks/commit-msg
```

Expected: the file should reference lefthook and exist (non-empty). Exact content depends on lefthook version.

- [ ] **Step 3: Do a live smoke test**

Create a scratch branch, make a trivial change, commit with an AI co-author trailer, and confirm it is stripped from the recorded commit:

```bash
git checkout -b test/strip-coauthor-smoke-test
echo "// smoke test" >> lefthook.yml
git add lefthook.yml

# Write the commit message manually to a temp file to bypass interactive editor
cat > /tmp/smoke-commit.txt << 'EOF'
chore: smoke test strip-ai-coauthors hook

Co-Authored-By: Claude <claude@anthropic.com>
EOF

GIT_EDITOR="cp /tmp/smoke-commit.txt" git commit
```

Then inspect the recorded commit:
```bash
git log -1 --format="%B"
```

Expected output — no `Co-Authored-By` line:
```
chore: smoke test strip-ai-coauthors hook

```

- [ ] **Step 4: Clean up the smoke test branch**

```bash
git checkout -
git branch -D test/strip-coauthor-smoke-test
```

- [ ] **Step 5: Confirm no further commits needed**

No new files were created in this task. Nothing to commit.
