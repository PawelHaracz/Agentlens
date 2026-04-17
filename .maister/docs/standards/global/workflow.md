# Global: Development Workflow

Project-specific workflow conventions for AI agents and human contributors.

### Read `.maister/docs/INDEX.md` Before Any Task

Always read `.maister/docs/INDEX.md` first — it indexes the project's coding standards, tech stack, and architecture decisions. Follow standards in `.maister/docs/standards/` when writing code — they represent team decisions. Source: CLAUDE.md.

### Prefer code-review-graph MCP Tools Over Grep/Glob/Read

Use code-review-graph MCP tools (`semantic_search_nodes`, `query_graph`, `get_impact_radius`, `detect_changes`, `get_affected_flows`, `get_architecture_overview`, `refactor_tool`) BEFORE falling back to Grep/Glob/Read. They are faster, use fewer tokens, and provide structural context. Token rules: `get_minimal_context` first; `detail_level="minimal"` default; escalate only for high-risk items; **max 3 graph calls per turn**. Sources: CLAUDE.md, .cursorrules.

### Execute `/maister:*` Commands via Skill Tool

When any `/maister:*` command is invoked, execute it via the Skill tool immediately — do not skip workflows for "straightforward" tasks. Source: CLAUDE.md.

### PR Title and Type Checkbox Must Match Actual Scope

Reviewers flag PRs marked "Documentation only" that also ship production code (migrations, routes, handlers, tests). Update the PR title and type checkboxes before merge so reviewers apply the right scrutiny and CI expectations match the actual change set. Source: PR reviews (#7, #21, #23).

### CI Gates Before Commit

`rtk make test` must pass before committing. CI must show green on: lint + test + build. Sources: CLAUDE.md, AGENTS.md.
