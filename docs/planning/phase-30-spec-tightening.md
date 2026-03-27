# Phase 30: Spec Tightening

> **Goal:** The specification layer is strict enough that agents receive
> unambiguous, testable requirements - and the system can detect what changed
> at the requirements level when code lands. Hardens specs before Phase 31
> introduces a planner that depends on spec quality.

## Milestone

`Phase 30: Spec Tightening`

---

## Issue 678: Require --tag on vet scenarios and scope to phase subdirectory

### Description

Make the `--tag` (or `--milestone`) flag required on `godark vet scenarios`,
matching the existing behavior of `godark vet issues`. When a tag is provided,
scope scenario file discovery to the `tests/scenarios/phase-N/` subdirectory
instead of walking the entire scenario directory. This prevents old phases from
failing validation when new rules are added in later phases.

### Key constraints

- `internal/cmd/vet_scenarios.go`: add the same guard that `vet_issues.go:21`
  has: `if repo == "" || milestone == "" { return error }`. Remove the existing
  block at lines 23-25 that only requires repo when milestone is set.
- When `--tag phase-N` is provided, compute the phase subdirectory as
  `filepath.Join(scenarioDir, milestoneToLabel(milestone))` and pass that
  scoped path to `vet.ValidateScenarios()` instead of the root `scenarioDir`.
  `milestoneToLabel` already exists in `vet_helpers.go:29`.
- `internal/vet/scenarios.go`: no changes to `ValidateScenarios` signature -
  it already accepts a `scenarioDir` string. The scoping happens at the call
  site in the command handler.
- If the computed phase subdirectory does not exist, return a clear error
  (e.g., "no scenario directory found for phase-30: tests/scenarios/phase-30/").

### Acceptance criteria

- [ ] `godark vet scenarios` without `--tag` or `--milestone` returns an error
      with a usage message
- [ ] `godark vet scenarios --tag phase-30 --repo owner/repo` validates only
      files in `tests/scenarios/phase-30/`
- [ ] Missing phase subdirectory produces a clear error message
- [ ] `go build ./...` and `go vet ./...` pass

### Test cases

- **Missing tag errors**: Run `vet scenarios` with `--repo` but no `--tag` or
  `--milestone`, expect error containing "required"
- **Scoped directory**: Create temp dir with `phase-1/` and `phase-2/`
  subdirectories containing specs. Run with `--tag phase-1`, verify only
  phase-1 specs are validated
- **Missing subdir errors**: Run with `--tag phase-99` where no
  `tests/scenarios/phase-99/` exists, expect error naming the missing path

---

## Issue 680: GIVEN/WHEN/THEN validation in vet scenarios

**Blocked by**: #678

### Description

Extend `validateScenarioFile` in `internal/vet/scenarios.go` to require
GIVEN/WHEN/THEN structure in scenario case outcomes. Each case under
`## Cases` must have at least one `- GIVEN`, one `- WHEN`, and one `- THEN`
bullet. This replaces the current check that only requires at least one
`- ` bullet per case.

### Key constraints

- `internal/vet/scenarios.go`: modify the case outcome validation in
  `validateScenarioFile` (currently around lines 124-131). After collecting
  bullet lines for a case, check that at least one starts with `- GIVEN `
  (case-insensitive), one with `- WHEN `, and one with `- THEN `.
- Error messages must include the file path, case name, and which clause is
  missing (e.g., "phase-30/123-foo.md: case 'Happy path' missing WHEN clause").
- The existing check for "at least one outcome bullet" is subsumed by the new
  check - if GIVEN/WHEN/THEN are all present, there are at least 3 bullets.
  Remove the old check to avoid duplicate findings.
- `internal/vet/scenarios_test.go`: add tests for the new validation. Update
  existing test specs that use plain `- outcome` bullets to use GIVEN/WHEN/THEN
  format so they continue passing.

### Acceptance criteria

- [ ] A scenario case with `- GIVEN`, `- WHEN`, and `- THEN` bullets passes
- [ ] A case missing any of the three clauses produces an Error-severity finding
- [ ] The finding message names the file, case, and missing clause
- [ ] Existing tests updated and passing
- [ ] `go build ./...` and `go vet ./...` pass

### Test cases

- **Valid GWT case passes**: Spec with `- GIVEN x`, `- WHEN y`, `- THEN z`
  produces no findings for that case
- **Missing GIVEN**: Case has WHEN and THEN but no GIVEN, expect error naming
  "GIVEN"
- **Missing WHEN**: Case has GIVEN and THEN but no WHEN, expect error naming
  "WHEN"
- **Missing THEN**: Case has GIVEN and WHEN but no THEN, expect error naming
  "THEN"
- **Case-insensitive**: `- given x`, `- When y`, `- THEN z` all accepted
- **Multiple clauses**: Case with 2 GIVEN + 1 WHEN + 3 THEN passes (multiples
  are fine)

---

## Issue 679: Spec delta package

### Description

Create a new `internal/specdelta/` package in the domain layer that diffs two
scenario spec strings (before and after merge) and produces a structured delta
describing which requirements and cases were added, changed, or removed.
Includes a markdown formatter for posting deltas as PR comments.

### Key constraints

- New package at `internal/specdelta/` in the domain layer
- Add `"internal/specdelta/"` to the `domain` layer's `paths` array in
  `docs/architecture.json`
- Domain layer: may depend on `foundation` and `content` only. No imports from
  `infrastructure`, `orchestration`, `service`, or `cmd`.
- Core function: `Diff(before, after string) Delta` - takes two raw scenario
  spec strings and returns a structured delta
- `Delta` struct contains:
  - `AddedCases []string` - case names present in after but not before
  - `RemovedCases []string` - case names present in before but not after
  - `ChangedCases []CaseChange` - cases present in both but with different
    content
  - `SetupChanged bool` - whether the Setup section differs
- `CaseChange` struct: `Name string`, `Before string`, `After string`
- `Format(d Delta) string` - renders a Delta as a markdown summary suitable
  for a PR comment body. Empty delta produces empty string (no comment needed).
- Parsing reuses the same section-splitting approach as `vet/scenarios.go`
  (split on `### ` headings). Extract to a shared helper in `mdutil` if
  duplication is significant, otherwise inline.
- `IsEmpty(d Delta) bool` - returns true when nothing changed (all slices
  empty, SetupChanged false)

### Acceptance criteria

- [ ] `Diff("", after)` treats all cases as added
- [ ] `Diff(before, "")` treats all cases as removed
- [ ] `Diff(before, after)` with identical content returns an empty delta
- [ ] Changed cases include before/after content
- [ ] `Format()` produces readable markdown with sections for added, removed,
      and changed cases
- [ ] Empty delta formats to empty string
- [ ] `go build ./...` and `go vet ./...` pass

### Test cases

- **All added**: Empty before, after has 3 cases. Delta has 3 AddedCases,
  no removed or changed
- **All removed**: Before has 2 cases, empty after. Delta has 2 RemovedCases
- **No change**: Identical before and after. `IsEmpty()` returns true
- **Mixed changes**: Before has cases A, B, C. After has B (modified), C, D.
  Delta: removed A, changed B, added D, C unchanged
- **Setup changed**: Same cases but different Setup section.
  `SetupChanged == true`
- **Format added cases**: `Format()` output contains "### Added" heading and
  case names
- **Format empty delta**: `Format()` returns ""

---

## Issue 681: Wire spec delta into pipeline

**Blocked by**: #679

### Description

Integrate the `specdelta` package into the agent pipeline. Before merging a PR,
stash the current scenario spec content. After merge, read the updated spec,
compute the delta, post it as a PR comment, and store it in run data.

### Key constraints

- `internal/agent/loop.go`: in `runFunctionalReviewCycle`, before the merge at
  line ~888:
  - Call `punchlist.ReadScenarioSpec(cfg.ScenarioDir, issue.Number)` to capture
    the pre-merge spec content into a local variable
  - After the successful merge (line ~888) and before returning at line ~902,
    read the spec again (post-merge content is available because the merge
    target branch now has the PR's changes)
  - Call `specdelta.Diff(before, after)` and if not empty, format and post as
    a PR comment via `GuardRunner("gh", "pr", "comment", ...)`
  - The comment should have a recognizable header (e.g.,
    "## Spec Delta") for future tooling to find/update
- `internal/rundata/writer.go`: add `WriteSpecDelta(issueNum int, delta SpecDeltaData) error`
  following the `WritePunchlist` pattern. Writes to
  `issues/<issueNum>/spec-delta.json`.
- Add `SpecDeltaData` struct to rundata: `Before string`, `After string`,
  `AddedCases []string`, `RemovedCases []string`, `ChangedCases int`,
  `SetupChanged bool`
- `internal/agent/loop.go`: call `hook.WriteSpecDelta()` after computing delta
- The `RunDataHook` interface in loop.go needs a new `WriteSpecDelta` method.
  Update the interface and all implementations (production writer + test stubs).
- If no scenario spec exists (both before and after are empty), skip delta
  entirely - no comment, no run data write.

### Acceptance criteria

- [ ] PR comment with "## Spec Delta" header is posted when scenario spec
      changes
- [ ] No comment is posted when spec is unchanged or no spec exists
- [ ] `spec-delta.json` is written to run data directory for the issue
- [ ] `RunDataHook` interface includes `WriteSpecDelta` and all implementations
      compile
- [ ] `go build ./...` and `go vet ./...` pass

### Test cases

- **Delta posted**: Mock merge with spec change, verify `gh pr comment` is
  called with body containing "## Spec Delta"
- **No spec skipped**: Issue with no scenario spec before or after merge,
  verify no comment call
- **Unchanged spec skipped**: Spec exists but identical before and after merge,
  verify no comment call
- **Run data written**: Verify `WriteSpecDelta` is called with correct issue
  number and delta data
- **Hook interface compiles**: All RunDataHook implementations (production +
  test stubs) satisfy the updated interface

---

## Issue 682: Update create-scenarios skill and spec_generator prompt for GIVEN/WHEN/THEN

**Blocked by**: #680

### Description

Update the scenario generation skill and prompt template to emit GIVEN/WHEN/THEN
format by default, matching the new validation requirements. This is a
content-only change - no Go code modified.

### Key constraints

- `prompts/spec_generator.txt`: update the format example (lines 17-33) to use
  GIVEN/WHEN/THEN bullets instead of plain outcome bullets:
  ```
  ### <Case name>
  - GIVEN <precondition>
  - WHEN <action>
  - THEN <expected outcome>
  ```
- `internal/skills/godark-create-scenarios/SKILL.md`: update any format examples
  or instructions to show GIVEN/WHEN/THEN structure
- Copy updated skill to `.claude/skills/godark-create-scenarios/SKILL.md`
- Do not modify any Go files or test files

### Acceptance criteria

- [ ] `prompts/spec_generator.txt` format example uses GIVEN/WHEN/THEN bullets
- [ ] `internal/skills/godark-create-scenarios/SKILL.md` format examples use
      GIVEN/WHEN/THEN bullets
- [ ] `.claude/skills/` copy matches `internal/skills/` copy
- [ ] No Go files modified

### Test cases

- **Prompt format updated**: `spec_generator.txt` contains "- GIVEN", "- WHEN",
  and "- THEN" in the format example block
- **Skill format updated**: `godark-create-scenarios/SKILL.md` contains
  "- GIVEN", "- WHEN", and "- THEN"
- **Copies in sync**: `diff` between internal and .claude skill copies returns
  no differences
