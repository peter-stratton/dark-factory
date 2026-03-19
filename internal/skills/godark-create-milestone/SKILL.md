---
name: godark-create-milestone
description: Create a project roadmap and GitHub milestones through conversation
argument-hint: "<project-goal>"
disable-model-invocation: true
---

# Create Milestone

Help the user define a phased roadmap for their project and write it to
`docs/ROADMAP.md`. Create a GitHub milestone for each phase.

## Steps

1. **Gather context** — Read `CLAUDE.md` (if it exists) to understand the
   project. Read `docs/architecture.json` and
   `docs/conventions.md` if they exist to understand current architecture layers
   and coding conventions. Read `godark.yaml` to find the `repo` and
   `roadmap_path` (default: `docs/ROADMAP.md`).

   If `docs/architecture.json` is empty or missing, note this and suggest running
   `/godark-define-architecture` before or after creating milestones so that
   structural decisions are captured in a machine-readable form.

2. **Discuss goals** — Ask the user about:
   - What the project does and who it's for
   - What a working MVP looks like
   - Key constraints or technical decisions already made
   - Rough features they have in mind
   - Architecture layers if the conversation reveals structural decisions (e.g.
     new packages, service boundaries, or layer responsibilities)

3. **Propose phases** — Based on the discussion, propose a phased breakdown.
   Each phase should have:
   - A clear goal (what's true when the phase is done)
   - A short list of issue-level work items (not full specs — just slugs)
   - Dependencies on prior phases

   Ask the user to review. Iterate until they're satisfied with the phasing.

4. **Write the roadmap** — Create `docs/ROADMAP.md` (or the configured
   `roadmap_path`) using the format below. Create parent directories if needed.

5. **Create milestones** — For each phase, run:
   ```
   gh milestone create "<Phase Name>" --repo <repo>
   ```
   If the milestone already exists, skip it and note that in the output.

6. **Print summary** — List the phases, milestone names, and file path.

## Format

```markdown
# <Project Name> — Roadmap

> One-line project description.

---

## Phase 1: <Name>

**Goal**: <What's true when this phase is complete.>

**Milestone**: `<Phase Name>` | **Label**: `phase-1`

- <Issue slug — brief description of a work item>
- <Issue slug — brief description of a work item>

---

## Phase 2: <Name>
...
```

## Rules

- Every phase must have a **Goal**, **Milestone**, and **Label**.
- Milestone names must match exactly between the roadmap and GitHub.
- Labels follow the pattern `phase-N`.
- Issue slugs are short descriptions, not full specs — those come later in
  `/godark-create-planning-doc`.
- If a roadmap already exists, ask the user whether to replace it or append new
  phases.
- Do not create GitHub issues — that is handled by `/godark-create-issues`.

## CRITICAL — Next step

When this skill completes, suggest the user run `/godark-create-planning-doc`
next. Do NOT attempt to create the planning doc yourself — the skill contains
specific formatting, structure, and issue-splitting directives that must be
followed exactly. The full workflow order is:

1. `/godark-create-milestone` (this skill)
2. `/godark-create-planning-doc`
3. `/godark-create-issues`
4. `/godark-create-scenarios`
