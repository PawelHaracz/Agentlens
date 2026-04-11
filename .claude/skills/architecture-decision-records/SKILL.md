---
name: architecture-decision-records
description: >
  Use when starting brainstorming or writing a spec — reads existing ADRs before
  designing, flags conflicts between proposed design and prior decisions, and
  creates a new ADR when the design introduces or changes an architectural decision.
  Also use when a user asks to "check ADRs", "create an ADR", or "update architecture docs".
---

# Architecture Decision Records (ADR)

## When this skill activates

This skill is a **mandatory step inside the brainstorming flow**, injected at two points:

1. **Before** proposing design approaches — read existing ADRs to understand constraints.
2. **After** the user approves the design — check if any architectural decisions were made or changed, and write a new ADR if so.

It is also invoked standalone when the user explicitly asks about ADRs.

---

## ADR location convention

ADRs live in `docs/adr/` by default.  
Format: `docs/adr/NNNN-<kebab-case-title>.md` (e.g. `docs/adr/0003-use-postgres-for-persistence.md`)  
If the project uses a different location, respect it — check `CLAUDE.md`, `AGENTS.md`, or ask the user once and remember the answer.

---

## Phase 1 — Read ADRs before brainstorming

**Trigger:** At the very start of the `brainstorming` skill, before asking clarifying questions or proposing approaches.

Steps:
1. Check if `docs/adr/` exists. If not, note that no ADRs exist yet and proceed.
2. Read all ADR files. Focus on:
   - **Status** (`Accepted`, `Deprecated`, `Superseded`) — ignore Deprecated/Superseded for active constraints.
   - **Decision** — what was chosen and why.
   - **Consequences** — what it rules out.
3. Build a short internal summary: *"Active architectural constraints from ADRs: ..."*
4. Use this summary silently when evaluating options during brainstorming.
   - If a proposed approach **conflicts** with an accepted ADR, **flag it explicitly** to the user before continuing:
     > ⚠️ This approach conflicts with ADR-NNNN (*title*): [one sentence why]. Do you want to proceed and supersede that ADR, or pick a different approach?
   - Wait for user confirmation before moving on.

---

## Phase 2 — Write a new ADR after design approval

**Trigger:** After the user approves the spec (at the end of `brainstorming`, before invoking `writing-plans`).

Steps:
1. Review the approved design for architectural decisions — choices that:
   - Select a technology, framework, or protocol
   - Define system boundaries or integration patterns
   - Constrain future implementation options
   - Change or supersede a previously accepted decision
2. If **no** new architectural decisions were made (purely implementation-level work): skip ADR creation, note this to the user, proceed to `writing-plans`.
3. If **one or more** new architectural decisions exist:
   a. Determine the next ADR number (read existing files, increment highest N).
   b. For each decision, write an ADR file using the template below.
   c. If this decision supersedes an existing ADR, update the old ADR's `Status` field to `Superseded by ADR-NNNN`.
   d. `git add` and `git commit` all ADR files with message: `docs(adr): add ADR-NNNN <title>`.
   e. Tell the user: *"Created ADR-NNNN: [title]. Proceeding to writing-plans."*

---

## ADR file template

```markdown
# ADR-NNNN: <Title — short imperative phrase>

Date: YYYY-MM-DD  
Status: Accepted  
<!-- Other valid statuses: Proposed | Deprecated | Superseded by ADR-XXXX -->

## Context

What problem or situation prompted this decision?  
What forces are at play (technical, team, constraints)?

## Decision

What was decided? State it clearly and directly.

## Consequences

### Positive
- ...

### Negative / Trade-offs
- ...

### Neutral
- ...

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| ...    | ...          |
```

---

## Conflict resolution rules

| Situation | Action |
|-----------|--------|
| Proposed design matches existing ADRs | Proceed silently |
| Proposed design extends an ADR (same technology, new usage) | Note the ADR in the spec, no new ADR needed |
| Proposed design contradicts an accepted ADR | **Stop and ask** the user — supersede or change approach |
| Proposed design makes a new architectural choice not covered by any ADR | Write a new ADR after approval |
| User explicitly says "ignore ADRs for now" | Proceed, but warn once that ADRs will be out of sync |

---

## Integration with brainstorming flow

```
[brainstorming starts]
        ↓
[invoke superpowers:architecture-decision-records — Phase 1]
        ↓
[read existing ADRs → build constraint summary]
        ↓
[ask clarifying questions, propose approaches]
        ↓  ← conflict check happens here, with ADR flags if needed
[user approves design]
        ↓
[invoke superpowers:architecture-decision-records — Phase 2]
        ↓
[write new ADR(s) if needed → commit]
        ↓
[invoke superpowers:writing-plans]
```

---

## Notes

- ADRs are **not** implementation tasks. Do not add ADR writing to the plan's task list — it happens during brainstorming, before the plan.
- One ADR per distinct decision. If the design has three independent architectural choices, write three ADRs.
- Keep ADRs short. The context + decision + consequences should fit on one screen.
- If the user has a preferred ADR format (MADR, Nygard, Y-statements), use it. Ask once if uncertain.
