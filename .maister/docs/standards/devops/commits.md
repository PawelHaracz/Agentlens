# DevOps: Commit Conventions

Conventional Commits enforced via commitlint in the commit-msg git hook. AI co-author trailers are stripped automatically.

### Conventional Commits Required
Format: `<type>[(scope)]: <subject>`. Allowed types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, `build`, `perf`, `revert`. Subject line max 100 characters. Scope must be lower-case. Enforced by `web/commitlint.config.ts` via the `commit-msg` lefthook. Examples: `feat(auth): add account lockout after 5 failures`, `fix(store): prevent duplicate endpoint upsert`. Sources: web/commitlint.config.ts, lefthook.yml, CLAUDE.md, ADR-002.

### Strip AI Co-Author Trailers
The `commit-msg` hook runs `perl -i` to strip `Co-Authored-By:` lines matching `claude|copilot|gemini|chatgpt|openai|cursor` before commitlint validation. Sources: lefthook.yml.

### Branch Naming
Feature/fix branches must use `feat/short-description` or `fix/short-description` prefixes. Sources: CLAUDE.md, AGENTS.md.
