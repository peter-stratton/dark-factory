---
name: godark-create-planning-doc
description: Create a detailed planning doc for a roadmap phase
argument-hint: "<phase-number>"
disable-model-invocation: true
---

# Create Planning Doc

Generate a detailed planning document for a single phase of the project roadmap.
The planning doc expands each issue slug from the roadmap into a full spec with
description, constraints, acceptance criteria, and test cases.

## Steps

1. **Read the roadmap** — Read `docs/ROADMAP.md` (or the configured
   `roadmap_path`) and find the specified phase. Extract the phase name, goal,
   milestone, and issue slugs.

2. **Read prior planning docs** — Read existing files in `docs/planning/` (or
   the configured `planning_dir`) to understand the project's conventions,
   naming patterns, and level of detail.

3. **Read project context** — Read `CLAUDE.md` (if it exists) for architecture
   and coding conventions. Also read
   `docs/architecture.json` and `docs/conventions.md` if they exist to
   understand the current architecture layers and agreed coding conventions.

4. **Discuss each issue** — For each issue slug in the phase, work with the user
   to flesh out:
   - What exactly should be built
   - Key constraints (package paths, function signatures, dependencies)
   - How it relates to / depends on other issues
   - Acceptance criteria (concrete, checkable outcomes)
   - Test cases (named, with inputs and expected outputs)

   Ask clarifying questions. Don't assume — the user knows what they want, and
   vague specs lead to bad agent output.

   When new phases introduce packages that don't fit any existing layer in
   `docs/architecture.json`, prompt the user to update `docs/architecture.json`
   before finalising the spec (or suggest running `/godark-define-architecture`
   to revise the layer definitions).

5. **Write the planning doc** — Create the file in `docs/planning/` using the
   format below. The filename should be `phase-N-<kebab-slug>.md` matching the
   phase name.

6. **Print summary** — List the file path and issue count.

## Format

```markdown
# Phase N: <Phase Name>

> **Goal:** <Phase goal from roadmap>

## Milestone

`<Milestone Name>`

---

## Issue: <Title>

**Blocked by**: #N (if applicable)

### Description

<What to build and why.>

### Key constraints

- <Package path, function signature, or design constraint>

### Acceptance criteria

- [ ] <Concrete, verifiable outcome>
- [ ] <Concrete, verifiable outcome>

### Test cases

- **<Test name>**: <Description of input and expected output>
- **<Test name>**: <Description of input and expected output>

---

## Issue: <Title>
...
```

## Rules

- Every issue must have Description, Acceptance criteria, and Test cases
  sections.
- Acceptance criteria must use `- [ ]` checkbox format.
- Test cases must use `- **Name**: description` format.
- Dependencies use `**Blocked by**: #N` notation (with issue numbers if known,
  or issue titles if numbers haven't been assigned yet).
- Issue headings use `## Issue: <Title>` (not `## Issue N:`) — issue numbers
  are added later by `/godark-create-issues`.
- Do not create GitHub issues — that is handled by `/godark-create-issues`.
- Do not modify the roadmap — that is handled by `/godark-create-roadmap`.
- If a planning doc already exists for this phase, ask the user whether to
  replace it or update specific issues.

## Task sizing

Each issue must be small enough for an agent to implement in a single run
(~15 minutes). An issue is too large if any of these apply:

- **More than 5 acceptance criteria** — split into separate deliverables.
- **More than 7 test cases** — this usually means multiple concerns are bundled.
- **Creates new code AND modifies existing code** — split "add new package" from
  "wire it into existing code" from "remove/migrate old code."
- **Touches more than 3 existing files** — the agent loses context and makes
  mistakes on cross-cutting changes.
- **Combines additive and destructive changes** — adding a new system and
  removing the old one should be separate issues so each is independently
  verifiable.
- **Multiple execution modes** — if the description says "in mode X do A, in
  mode Y do B" (e.g., host mode vs sandbox mode, sync vs async), split each
  mode into its own issue. The first mode establishes the interface, subsequent
  modes add variants.
- **Wiring + retry/loop logic** — if an issue both integrates a new subsystem
  AND adds retry/fix/fallback behavior around it, split the basic integration
  (happy path + fail-fast) from the retry loop.

When an issue triggers multiple flags, ask the user whether to split it. Present
the natural seams you see (e.g., "this issue has 9 test cases and two execution
modes — would you like to split host-mode wiring from sandbox-mode?") and let
them decide. Don't auto-split without confirmation.

When an issue is too large, split it along natural boundaries:

1. **New package/module** — pure new code with its own tests, no existing files
   modified.
2. **Integration/wiring** — connect the new code to existing callers. Happy path
   and fail-fast only.
3. **Retry/fix loops** — add retry, fix cycle, or fallback behavior on top of
   the basic wiring.
4. **Variant modes** — add alternative execution paths (sandbox, async, etc.)
   that share the same interface.
5. **Migration/cleanup** — remove old code, update config, rename fields.

Each sub-issue should be independently deployable and testable. Use `Blocked by`
dependencies to enforce ordering.
