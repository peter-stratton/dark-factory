# Phase 30: Spec Tightening

Agents work best when requirements are unambiguous. Phase 30 hardens the specification layer in two directions: it enforces GIVEN/WHEN/THEN structure in scenario specs so agents receive testable requirements, and it introduces spec delta generation so the system can detect and report what changed at the requirements level when a PR merges. This phase was a prerequisite for Phase 31 (Planner Agent), which depends on spec quality to produce reliable implementation plans.

---

## Vet Scenarios Phase Scoping

**What it does:** The `godark vet scenarios` command now requires a `--tag` flag and scopes validation to the matching phase subdirectory. This prevents old phases from failing when new validation rules are added in later phases - only the targeted phase's specs are checked.

**Example:** Validating specs for a specific phase:

```
$ godark vet scenarios --tag "Phase 30: Spec Tightening" --repo owner/repo
```

The command extracts the phase label from the milestone title using `milestoneToLabel` in `internal/cmd/vet_helpers.go`:

```go
func milestoneToLabel(milestone string) string {
    if m := patterns.Phase.FindStringSubmatch(milestone); m != nil {
        return "phase-" + m[1]
    }
    return strings.ReplaceAll(strings.ToLower(milestone), " ", "-")
}
```

"Phase 30: Spec Tightening" becomes `phase-30`, and validation scopes to `tests/scenarios/phase-30/`. If the subdirectory doesn't exist, the command fails with a clear error naming the missing path rather than silently validating nothing.

Running without `--tag` produces a usage error - the flag is required to prevent accidentally validating the entire scenario tree against the latest rules.

---

## GIVEN/WHEN/THEN Validation

**What it does:** Each scenario case under `## Cases` must have at least one `- GIVEN`, one `- WHEN`, and one `- THEN` bullet. This replaces the previous check that only required at least one `- ` bullet per case, and produces error-severity findings with the file path, case name, and missing clause.

**Example:** A scenario spec at `tests/scenarios/phase-30/680-gwt-validation.md`:

```markdown
## Cases

### Happy path
- GIVEN a scenario file with GIVEN, WHEN, and THEN clauses
- WHEN godark vet scenarios validates the file
- THEN validation passes with no findings

### Missing clause
- GIVEN a scenario file with only GIVEN and THEN
- THEN validation reports a missing WHEN clause
```

The second case is missing a WHEN clause. Running `godark vet scenarios --tag phase-30` produces:

```
ERROR  phase-30/680-gwt-validation.md: case "Missing clause" missing WHEN clause
```

The validation logic in `validateCaseOutcomes` in `internal/vet/scenarios.go` walks each case's bullets and checks for all three clauses using case-insensitive matching:

```go
up := strings.ToUpper(trimmed)
if strings.HasPrefix(up, "- GIVEN ") {
    hasGiven = true
}
if strings.HasPrefix(up, "- WHEN ") {
    hasWhen = true
}
if strings.HasPrefix(up, "- THEN ") {
    hasThen = true
}
```

Multiple clauses of the same type are fine - a case with 2 GIVEN + 1 WHEN + 3 THEN passes validation.

---

## Spec Delta Package

**What it does:** The `internal/specdelta/` package diffs two scenario spec strings (before and after merge) and produces a structured delta describing which requirements and cases were added, changed, or removed. A markdown formatter renders deltas for PR comments.

**Example:** A PR modifies a scenario spec, removing case "Legacy flow", changing case "Happy path", and adding case "Error handling". The delta computation:

```go
delta := specdelta.Diff(specBefore, specAfter)
// delta.RemovedCases  = ["Legacy flow"]
// delta.ChangedCases  = [{Name: "Happy path", Before: "...", After: "..."}]
// delta.AddedCases    = ["Error handling"]
// delta.SetupChanged  = false
```

The `Delta` struct in `internal/specdelta/specdelta.go`:

```go
type Delta struct {
    AddedCases   []string
    RemovedCases []string
    ChangedCases []CaseChange
    SetupChanged bool
}

type CaseChange struct {
    Name   string
    Before string
    After  string
}
```

`Format(delta)` renders it as markdown:

```markdown
### Removed
- Legacy flow

### Changed
**Happy path**

**Before:**
- GIVEN an authenticated user
- WHEN they submit the form
- THEN the record is created

**After:**
- GIVEN an authenticated user with valid permissions
- WHEN they submit the form with all required fields
- THEN the record is created and an audit log entry is written

### Added
- Error handling
```

`IsEmpty(delta)` returns true when nothing changed - no PR comment is posted and no run data is written in that case.

---

## Pipeline Integration

**What it does:** The agent loop captures the scenario spec content before merging a PR, reads it again after merge, computes the delta, posts it as a PR comment with a `## Spec Delta` header, and persists the delta to run data.

**Example:** During `ProcessIssue` in `internal/agent/loop.go`, the spec is captured at two points:

```go
// Capture pre-merge spec content for delta computation.
specBefore, err := punchlist.ReadScenarioSpec(cfg.ScenarioDir, issue.Number)
if err != nil {
    logger.Warn("failed to read pre-merge scenario spec", "error", err)
}

// ... PR merge happens ...

// Capture post-merge spec content and compute delta.
specAfter, err := punchlist.ReadScenarioSpec(cfg.ScenarioDir, issue.Number)

delta := specdelta.Diff(specBefore, specAfter)
logger.Info("spec delta computed", "empty", specdelta.IsEmpty(delta),
    "added", len(delta.AddedCases), "removed", len(delta.RemovedCases),
    "changed", len(delta.ChangedCases))
```

When the delta is non-empty, it's posted as a PR comment:

```go
if !specdelta.IsEmpty(delta) {
    comment := "## Spec Delta\n\n" + specdelta.Format(delta)
    GuardRunner("gh", "pr", "comment", fmt.Sprintf("%d", prNum),
        "--repo", cfg.Repo, "--body", comment)
}
```

The `## Spec Delta` header makes these comments findable by future tooling. If no scenario spec exists for the issue (both before and after are empty), the delta step is skipped entirely.

---

## Run Data Persistence

**What it does:** Spec deltas are written to `spec-delta.json` in the issue's run data directory, making them available to the dashboard and analysis pipeline.

**Example:** After computing a delta, the pipeline writes it via the `RunDataHook` interface:

```go
WriteSpecDelta(issueNum int, delta rundata.SpecDeltaData) error
```

The `SpecDeltaData` struct in `internal/rundata/writer.go`:

```go
type SpecDeltaData struct {
    Before       string   `json:"before"`
    After        string   `json:"after"`
    AddedCases   []string `json:"added_cases,omitempty"`
    RemovedCases []string `json:"removed_cases,omitempty"`
    ChangedCases int      `json:"changed_cases"`
    SetupChanged bool     `json:"setup_changed"`
}
```

The resulting file at `~/.godark/runs/owner/repo/<timestamp>/issues/340/spec-delta.json`:

```json
{
  "before": "# Scenario: Rate limiting\n\n...",
  "after": "# Scenario: Rate limiting\n\n...",
  "added_cases": ["Error handling"],
  "removed_cases": ["Legacy flow"],
  "changed_cases": 1,
  "setup_changed": false
}
```

Both the raw before/after content and the structured summary are stored, so the dashboard can render the delta and downstream analysis can track spec churn over time.

---

## Spec Generator and Scenario Skill Updates

**What it does:** The spec generator prompt and the `/godark-create-scenarios` skill were updated to emit GIVEN/WHEN/THEN format by default, matching the new validation requirements.

**Example:** The format example in `prompts/spec_generator.txt` shows the required structure:

```markdown
## Cases

### <Case name>
- GIVEN <precondition>
- WHEN <action>
- THEN <expected outcome>
```

The prompt includes explicit guidance:

```
Every case MUST have at least one GIVEN, one WHEN, and one THEN bullet.
This is validated by `godark vet scenarios` and will fail without them.
```

The `/godark-create-scenarios` skill in `internal/skills/godark-create-scenarios/SKILL.md` mirrors this format and lists it as a hard requirement: every `### Case` must have GIVEN, WHEN, and THEN bullets. This means both human-authored specs (validated by `godark vet`) and agent-generated specs (guided by the prompt) follow the same structure.
