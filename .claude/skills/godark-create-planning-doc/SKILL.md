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

4. **Explore the change surface** — For each issue slug, before discussing it
   with the user, explore the codebase to understand what the issue would
   actually touch:
   - Search for types, functions, and packages the issue would likely affect
     (grep for relevant symbols from the issue description and roadmap slug).
   - Read the key files to understand their size, complexity, and coupling
     (count lines, note function signatures, check imports and callers).
   - Cross-reference touched files against `docs/architecture.json` to identify
     which layers are involved and whether the change spans layer boundaries.
   - Count: files that would change, approximate lines affected, number of
     callers/importers of code being modified.
   - Present a brief impact summary when discussing each issue with the user:
     "This issue would touch N files across M layers: [list]. The main
     complexity is [X]."
   - Use these concrete numbers — not description-based guesses — to drive the
     task sizing rules.

5. **Discuss each issue** — For each issue slug in the phase, work with the user
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

6. **Write the planning doc** — Create the file in `docs/planning/` using the
   format below. The filename should be `phase-N-<kebab-slug>.md` matching the
   phase name.

7. **Print summary** — List the file path and issue count.

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
- Do not modify the roadmap — that is handled by `/godark-create-milestone`.
- If a planning doc already exists for this phase, ask the user whether to
  replace it or update specific issues.

## Cross-layer moves

When an issue moves, copies, or re-exports code from one architecture layer to
another (e.g., `features → shared`, `app → shared`), the spec must trace the
import chain and resolve every violation **before the issue is written**:

1. **List every import** in the file(s) being moved. For each import, check
   whether it is legal in the destination layer (per `docs/architecture.json`).
2. **For each illegal import**, specify the exact resolution in the Key
   constraints section:
   - Extract the dependency to the destination layer (or a layer it can reach).
   - Add a re-export in the original location so existing callers still compile.
   - Or restructure the dependency in some other explicit way.
3. **Verify constraints are consistent.** If the spec says "don't modify file X"
   but an import resolution requires changing file X, that's a contradiction.
   Resolve it (e.g., use re-exports so file X stays untouched) or split into
   separate issues (one to extract, one to migrate callers).

The goal is to prevent the implementer from having to improvise layer-violation
fixes at implementation time, which causes review/retry fights when the fix
conflicts with other constraints in the same issue.

## Task sizing

Each issue must be small enough for an agent to implement in a single run
(~15 minutes). An issue is too large if any of these apply:

- **More than 5 acceptance criteria** — split into separate deliverables.
- **More than 7 test cases** — this usually means multiple concerns are bundled.
- **Creates new code AND modifies existing code** — split "add new package" from
  "wire it into existing code" from "remove/migrate old code."
- **Touches more than 3 existing files** — use the file count from step 4, not
  a guess from the description. The agent loses context on cross-cutting changes.
- **High fan-out modification** — if a function or type being changed has more
  than 5 callers (found via grep in step 4), split into "change the interface"
  and "update callers" rather than doing both at once.
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
