---
name: godark-create-issues
description: Create GitHub issues from a phase planning doc
argument-hint: "<phase-number>"
disable-model-invocation: true
---

# Create Issues

Read a planning doc and create GitHub issues with the proper structure for agent
consumption. Update the planning doc and roadmap with assigned issue numbers.

## Steps

1. **Read config** — Read `godark.yaml` to get the `repo`, `roadmap_path`
   (default: `docs/ROADMAP.md`), and `planning_dir` (default: `docs/planning/`).

2. **Find the planning doc** — Use the phase number argument to locate the
   planning doc by globbing `<planning_dir>/phase-<N>-*.md`. If no match is
   found, report an error. Parse the file to extract the milestone name and
   each issue's title, description, acceptance criteria, test cases, and
   dependencies.

3. **Check existing issues** — Run `gh issue list --milestone "<Milestone>"
   --repo <repo> --state all --limit 200` to see what already exists. Match by
   title to avoid creating duplicates.

4. **Confirm with user** — Present the list of issues, noting:
   - Which issues are new (will be created)
   - Which already exist and whether their body differs from the planning doc
     (offer to update)
   - Dependency ordering
   - The milestone and labels that will be applied

   Ask the user to confirm before creating or updating anything.

5. **Create or update issues** — For each new issue, run:
   ```
   gh issue create --title "<Title>" --body "<body>" \
     --milestone "<Milestone>" --label "phase-N" --repo <repo>
   ```

   The issue body must include:
   - `## Description` section
   - `## Acceptance criteria` section with `- [ ]` checkboxes
   - `## Test cases` section with `- **Name**:` entries
   - `**Blocked by**: #N` line if the issue has dependencies (using the real
     issue numbers from GitHub, not the planning doc order)

   For existing issues the user chose to update, run:
   ```
   gh issue edit <number> --body "<body>" --repo <repo>
   ```

6. **Update the planning doc** — Replace `## Issue: <Title>` headings with
   `## Issue N: <Title>` using the assigned GitHub issue numbers.

7. **Update the roadmap** — In `docs/ROADMAP.md`, add or update the `**Issues**`
   line for this phase with the issue number range (e.g., `**Issues**: #14–#17`).

8. **Print summary** — List each created issue with its number, title, and URL.

## Rules

- Never create duplicate issues — always check existing issues first.
- Issue bodies must follow the exact format that `godark vet issues` validates:
  `## Description`, `## Acceptance criteria` (with checkboxes), `## Test cases`
  (with named entries).
- Dependencies must use `**Blocked by**: #N` format with real issue numbers.
- If a dependency hasn't been created yet (e.g., within the same batch), create
  dependency issues first, then use their numbers.
- Apply the phase label (e.g., `phase-2`) to every issue.
- All issues must be assigned to the correct milestone.
- Do not modify scenario specs — that is handled by `/godark-create-scenarios`.
- If some issues already exist and some are new, only create the new ones and
  update the planning doc with all numbers (existing + new).
