# Phase 2: Quality & Vetting

> **Goal:** `godark vet issues`, `godark vet scenarios`, and `godark vet roadmap`
> validate that GitHub issues, scenario specs, and planning docs are clear,
> unambiguous, and fully actionable by agents.
>
> Built early so it can vet issues for all subsequent phases.

## Milestone

`Phase 2`

---

## Issue 14: Vet command scaffold and validation framework

### Description

Replace the existing `vet` stub with a real command that dispatches to
subcommands: `godark vet issues`, `godark vet scenarios`, `godark vet roadmap`.
Define the shared types that all vet subcommands will use: validation findings
(severity, message, location) and a report output format.

### Key constraints

- Replace the stub in `internal/cmd/vet.go` — do not create a separate file for
  the parent command
- Subcommand files: `internal/cmd/vet_issues.go`, `internal/cmd/vet_scenarios.go`,
  `internal/cmd/vet_roadmap.go`
- Shared types in a new package: `internal/vet/findings.go`
- Finding struct: `Severity` (error, warning, info), `Message`, `Location` (file
  path or issue number)
- Report: list of findings, summary counts by severity, exit code 1 if any errors
- Reuse `internal/github/` for issue fetching (same `CommandRunner` pattern)
- `--repo` and `--milestone` flags (inherited from root or passed explicitly)
- Human-readable table output to stdout, optional `--json` flag for machine output

### Acceptance criteria

- [ ] `godark vet --help` shows subcommands: issues, scenarios, roadmap
- [ ] `godark vet issues --help` shows flags with descriptions
- [ ] `godark vet scenarios --help` shows flags with descriptions
- [ ] `godark vet roadmap --help` shows flags with descriptions
- [ ] `Finding` struct exists with Severity, Message, Location fields
- [ ] Report output includes summary counts (errors, warnings, info)
- [ ] Exit code is 1 when any finding has error severity, 0 otherwise
- [ ] `go test ./internal/vet/` passes
- [ ] `go test ./internal/cmd/` passes

### Test cases

- **Help output**: `godark vet --help` lists issues, scenarios, roadmap subcommands
- **Finding struct**: Creating a Finding with error severity, message, and location populates all fields
- **Report with errors**: A report containing error findings returns exit code 1
- **Report without errors**: A report with only warnings returns exit code 0
- **Empty report**: No findings returns exit code 0 with "no issues found" message
- **JSON output**: `--json` flag produces valid JSON with findings array and summary

---

## Issue 15: Issue structure validation (`vet issues`)

**Blocked by**: #14 (Vet command scaffold)

### Description

Implement the `godark vet issues` subcommand. It fetches issues from a GitHub
milestone and validates that each issue has the required structure for agent
consumption: description, acceptance criteria, test cases, and well-formed
dependency notation.

### Key constraints

- Package: `internal/vet/issues.go`
- Fetch issues using `internal/github/` (same `CommandRunner` pattern)
- Required sections in issue body: `## Description`, `## Acceptance criteria`,
  `## Test cases`
- Acceptance criteria must contain at least one `- [ ]` checkbox item
- Test cases must contain at least one `- **Name**:` entry
- Validate `Blocked by` / `Depends on` notation: referenced issues must exist,
  format must match `**Blocked by**: #N` or `Depends on: #N, #M`
- Check every issue has the milestone's phase label (e.g., `phase-2`)
- Each violation produces a Finding (error or warning depending on severity)

### Acceptance criteria

- [ ] Issues missing `## Acceptance criteria` section produce an error finding
- [ ] Issues missing `## Test cases` section produce an error finding
- [ ] Issues with empty acceptance criteria (no checkboxes) produce an error finding
- [ ] Issues with empty test cases (no entries) produce an error finding
- [ ] Malformed `Blocked by` notation produces a warning finding
- [ ] References to non-existent issues produce a warning finding
- [ ] Issues missing the phase label produce a warning finding
- [ ] All findings include the issue number as location
- [ ] `go test ./internal/vet/` passes

### Test cases

- **Complete issue passes**: Issue with all required sections and valid notation produces no error findings
- **Missing acceptance criteria**: Issue body without `## Acceptance criteria` → error finding
- **Missing test cases**: Issue body without `## Test cases` → error finding
- **Empty acceptance criteria**: Section exists but contains no `- [ ]` items → error finding
- **Empty test cases**: Section exists but contains no `- **Name**:` entries → error finding
- **Malformed blocker**: `Blocked by #1` (missing colon) → warning finding
- **Non-existent blocker reference**: `**Blocked by**: #999` where #999 does not exist → warning finding
- **Missing phase label**: Issue in Phase 2 milestone without `phase-2` label → warning finding
- **Multiple issues**: Validates all issues in milestone, not just the first one

---

## Issue 16: Scenario spec validation (`vet scenarios`)

**Blocked by**: #14 (Vet command scaffold)

### Description

Implement the `godark vet scenarios` subcommand. It reads scenario spec files
from `tests/scenarios/`, validates their format, and cross-references them with
GitHub issues to ensure every issue has spec coverage.

### Key constraints

- Package: `internal/vet/scenarios.go`
- Read `*.md` files from the scenario directory (config `scenario_dir`, default
  `tests/scenarios/`)
- Required format: `# Scenario:` title, `Relates to: Issue #N`, `## Setup`
  section, `## Cases` section
- Each case (level-3 heading under `## Cases`) must have at least one bullet
  point (expected outcome)
- Cross-reference: fetch milestone issues from GitHub, verify every issue has at
  least one spec with `Relates to: Issue #N`
- Verify `Relates to:` references point to real GitHub issues
- Flag issues with no scenario spec coverage as warnings

### Acceptance criteria

- [ ] Spec files missing `## Setup` section produce an error finding
- [ ] Spec files missing `## Cases` section produce an error finding
- [ ] Cases with no expected outcomes (no bullet points) produce an error finding
- [ ] Specs with missing or malformed `Relates to:` line produce an error finding
- [ ] `Relates to:` references to non-existent issues produce a warning finding
- [ ] Issues in milestone with no matching scenario spec produce a warning finding
- [ ] Valid spec files produce no findings
- [ ] All findings include the file path as location
- [ ] `go test ./internal/vet/` passes

### Test cases

- **Valid spec passes**: A correctly formatted spec with valid `Relates to:` produces no findings
- **Missing Setup section**: Spec without `## Setup` → error finding with file path
- **Missing Cases section**: Spec without `## Cases` → error finding with file path
- **Case without outcomes**: A `### Case` heading followed by no bullet points → error finding
- **Missing Relates to**: Spec with no `Relates to:` line → error finding
- **Malformed Relates to**: `Relates to: 5` (missing `Issue #` prefix) → error finding
- **Non-existent issue reference**: `Relates to: Issue #999` where #999 does not exist → warning finding
- **Issue without spec coverage**: Issue in milestone has no matching `Relates to: Issue #N` → warning finding
- **Multiple relates-to lines**: Spec with `Relates to: Issue #14` and `Relates to: Issue #15` covers both issues

---

## Issue 17: Roadmap validation (`vet roadmap`)

**Blocked by**: #14 (Vet command scaffold)

### Description

Implement the `godark vet roadmap` subcommand. It parses planning docs in
`docs/planning/` and cross-references them with GitHub to verify that issue
references are valid, phase labels match milestone names, and no issues are
orphaned.

### Key constraints

- Package: `internal/vet/roadmap.go`
- Scan `docs/planning/*.md` for issue references (patterns like `## Issue N:`,
  `#N`, `Blocked by: #N`)
- Cross-reference with GitHub: every issue mentioned must exist, every issue in
  the milestone must appear in a planning doc
- Validate phase labels match milestone names (e.g., issues with `phase-2` label
  must be in `Phase 2` milestone)
- Flag orphaned issues: in milestone but not mentioned in any planning doc
- Flag phantom issues: referenced in planning doc but do not exist on GitHub

### Acceptance criteria

- [ ] Planning docs referencing non-existent issues produce a warning finding
- [ ] Issues in milestone but not in any planning doc produce a warning finding
- [ ] Issues with phase label not matching their milestone produce an error finding
- [ ] Valid planning docs with correct cross-references produce no findings
- [ ] All findings include the file path or issue number as location
- [ ] `go test ./internal/vet/` passes

### Test cases

- **Valid planning doc passes**: Doc with correct issue references that all exist → no findings
- **Phantom issue reference**: Planning doc mentions `## Issue 99:` but #99 does not exist → warning finding
- **Orphaned issue**: Issue #5 is in Phase 2 milestone but not mentioned in any planning doc → warning finding
- **Label-milestone mismatch**: Issue has `phase-1` label but is in `Phase 2` milestone → error finding
- **Multiple planning docs**: Issues can be referenced across different planning docs
- **No planning docs**: Empty `docs/planning/` directory → warning finding
