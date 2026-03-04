---
name: godark-create-scenario
description: Generate a scenario spec file for a GitHub issue
argument-hint: <issue-number>
disable-model-invocation: true
---

# Create Scenario Spec

Generate a scenario spec file for the given GitHub issue.

## Steps

1. **Fetch the issue** — Run `gh issue view <issue-number> --json title,body,milestone`
   to get the issue title, body, and milestone. If the command fails, stop and
   report the error.

2. **Determine the phase** — Extract the phase from the issue's milestone name
   (e.g., "Phase 3" → `phase-3`). If no milestone is set, ask the user which
   phase subdirectory to use.

3. **Read existing specs for reference** — List files in `tests/scenarios/` and
   its subdirectories, and read one or two to understand the project's scenario
   spec style.

4. **Generate the spec file** — Create a new file in the appropriate phase
   subdirectory under `tests/scenarios/` (e.g., `tests/scenarios/phase-3/`),
   creating the subdirectory if needed. The filename must be kebab-case derived
   from the scenario title (e.g., `config-loading.md`).

5. **Print the path** — After writing the file, print the path so the user can
   review it.

## Format

```markdown
# Scenario: <descriptive title>

Relates to: Issue #<issue-number>

## Setup
- Description of test fixture state needed for these cases

## Cases

### <case name>
Description of what to do.
- Expected outcome 1
- Expected outcome 2
```

## Rules

- Every spec **must** include the `Relates to: Issue #N` line.
- The `## Setup` section is **required** even if minimal (e.g., a single bullet
  describing the test environment). Never omit it.
- Every `### Case` **must** have at least one `- ` outcome bullet. A case
  without expected outcomes is not testable. If the outcome seems obvious,
  state it explicitly anyway — agents need concrete expectations.
- Filenames must be kebab-case and end in `.md`.
- Do **not** modify or overwrite existing spec files.
- A single spec file covers one logical scenario. If the issue needs multiple
  scenarios, create multiple files.
