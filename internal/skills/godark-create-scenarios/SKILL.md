---
name: godark-create-scenarios
description: Generate scenario spec files for a phase or individual issue
argument-hint: "<phase-number> or <issue-number>"
disable-model-invocation: true
---

# Create Scenario Specs

Generate scenario spec files for all issues in a phase, or for a single issue.

## Steps

1. **Determine scope** — If the argument looks like a phase number (small
   integer, or matches a planning doc), treat it as a phase and generate specs
   for all issues in that phase. If it looks like a larger issue number,
   generate a spec for that single issue.

2. **Gather issues** — For a phase:
   - Read `godark.yaml` to get the `repo` and `planning_dir`
     (default: `docs/planning/`).
   - Find the planning doc by globbing `<planning_dir>/phase-<N>-*.md`.
   - Extract all issue titles and numbers from the planning doc.
   - Run `gh issue list --milestone "<Phase Name>" --repo <repo> --state all
     --limit 200` to get issue numbers if not already in the planning doc.

   For a single issue:
   - Run `gh issue view <issue-number> --json title,body,milestone` to get the
     issue details.

3. **Check existing specs** — List files in `tests/scenarios/` and its
   subdirectories. Skip any issue that already has a scenario spec file
   (matching by `Relates to: Issue #N`). Report which issues are skipped.

4. **Read existing specs for reference** — Read one or two existing specs to
   understand the project's scenario spec style and conventions.

5. **Confirm with user** — Present the list of issues, noting which will get
   new specs and which are being skipped. Ask the user to confirm before
   generating.

6. **Generate spec files** — For each issue that needs a spec, create a new
   file in the appropriate phase subdirectory under `tests/scenarios/`
   (e.g., `tests/scenarios/phase-3/`), creating the subdirectory if needed.
   The filename must be kebab-case derived from the scenario title
   (e.g., `config-loading.md`).

7. **Print summary** — List each created file path and any skipped issues.

## Format

The canonical scenario spec format is defined in `prompts/spec_generator.txt`.
Follow that file exactly when generating specs.

The structure is: `# Scenario:` header, `Relates to: Issue #N` line, `## Setup`
section with fixture prerequisites, and `## Cases` section with one or more
`### <Case name>` subsections each containing GIVEN/WHEN/THEN bullets:

```markdown
### <Case name>
- GIVEN <precondition>
- WHEN <action>
- THEN <expected outcome>
```

## Rules

- Every spec **must** include the `Relates to: Issue #N` line.
- The `## Setup` section is **required** even if minimal (e.g., a single bullet
  describing the test environment). Never omit it.
- Every `### Case` **must** have GIVEN, WHEN, and THEN bullets. A case
  without these is not testable. If the outcome seems obvious,
  state it explicitly anyway — agents need concrete expectations.
- Filenames must be kebab-case and end in `.md`.
- Do **not** modify or overwrite existing spec files.
- A single spec file covers one logical scenario. If the issue needs multiple
  scenarios, create multiple files.
- When generating for a phase, process issues in dependency order (issues with
  no dependencies first) so that earlier specs can inform the style of later
  ones.

## CRITICAL — Workflow complete

This is the final skill in the planning workflow. The full order is:

1. `/godark-create-milestone`
2. `/godark-create-planning-doc`
3. `/godark-create-issues`
4. `/godark-create-scenarios` (this skill)

The project is now ready for `godark run`.
