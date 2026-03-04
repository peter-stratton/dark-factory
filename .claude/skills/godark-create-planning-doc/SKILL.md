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

3. **Read project context** — Read `CLAUDE.md` and `docs/CONTEXT.md` (if they
   exist) for architecture and coding conventions.

4. **Discuss each issue** — For each issue slug in the phase, work with the user
   to flesh out:
   - What exactly should be built
   - Key constraints (package paths, function signatures, dependencies)
   - How it relates to / depends on other issues
   - Acceptance criteria (concrete, checkable outcomes)
   - Test cases (named, with inputs and expected outputs)

   Ask clarifying questions. Don't assume — the user knows what they want, and
   vague specs lead to bad agent output.

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
