# Phase 19: Spring Cleaning

> **Goal:** The codebase has zero duplicated patterns, all agent output parsing
> uses structured formats with unified parsers, magic strings are typed
> constants, and shared helpers replace copy-pasted boilerplate — making every
> file a clean example of the project's conventions.

## Milestone

`Phase 19`

---

## Issue #384: Create `internal/mdutil` package with `WalkMarkdownFiles`

### Description

Create a new foundation-layer package for markdown file utilities. The first
(and for now only) export is `WalkMarkdownFiles`, which wraps the repeated
`filepath.WalkDir` + `.md` suffix filter pattern found in three packages.

### Key constraints

- New file `internal/mdutil/walk.go`
- Signature: `func WalkMarkdownFiles(dir string, fn func(path string) error) error`
  - Calls `filepath.WalkDir`, skips directories, skips non-`.md` files,
    passes matching paths to `fn`
  - Returns first error from `fn` or walk itself
- Add `internal/mdutil/` to the `foundation` layer in
  `docs/architecture.json` (paths array on the foundation layer object)
- Update `docs/architecture.md` to list `mdutil` under foundation layer

### Acceptance criteria

- [ ] `mdutil.WalkMarkdownFiles` exists with the specified signature
- [ ] Foundation layer in `architecture.json` includes `internal/mdutil/`
- [ ] `architecture.md` lists `mdutil` under foundation layer

### Test cases

- **Walks only markdown files**: Given a temp directory with `.md`, `.go`, and
  `.txt` files, only `.md` paths are passed to `fn`
- **Skips subdirectories**: Directories are not passed to `fn`
- **Propagates fn error**: When `fn` returns an error, `WalkMarkdownFiles`
  returns it
- **Handles missing directory**: Returns an error when `dir` does not exist

---

## Issue #400: Migrate callers to `mdutil.WalkMarkdownFiles`

**Blocked by**: #384

### Description

Replace the three inline `filepath.WalkDir` + `.md` filter patterns with calls
to `mdutil.WalkMarkdownFiles`.

### Key constraints

- Modify `internal/punchlist/punchlist.go` (lines 70-77): replace WalkDir
  closure with `mdutil.WalkMarkdownFiles` call
- Modify `internal/vet/scenarios.go` (lines 24-31): same replacement
- Modify `internal/agent/guardrails.go` (lines 140-146): same replacement
- Each caller currently has ~7 lines of boilerplate that reduces to a 1-line
  call plus the processing closure
- Do NOT modify `internal/cmd/init.go` — it uses `fs.WalkDir` on an embedded
  filesystem, not `filepath.WalkDir`

### Acceptance criteria

- [ ] `punchlist.go` uses `mdutil.WalkMarkdownFiles` instead of inline WalkDir
- [ ] `vet/scenarios.go` uses `mdutil.WalkMarkdownFiles`
- [ ] `agent/guardrails.go` uses `mdutil.WalkMarkdownFiles`
- [ ] No remaining `filepath.WalkDir` + `.md` suffix filter in these three files
- [ ] All existing tests pass without modification

### Test cases

- **Punchlist reads scenarios**: `ReadScenarioSpec` still finds and parses
  scenario files (existing test)
- **Vet validates scenarios**: `ValidateScenarios` still collects `.md` files
  (existing test)
- **Guardrails detects spec**: `HasScenarioSpec` still returns true when spec
  exists (existing test)

---

## Issue #385: Create `internal/exec` package with `CommandRunnerFunc`

### Description

Create a foundation-layer package that defines the shared function type used by
all package-level command runner variables across the codebase. Currently 6
packages independently define `var CommandRunner = func(name string, args
...string) ([]byte, error) { ... }` with identical signatures. A shared named
type reduces boilerplate and enables shared test helpers.

### Key constraints

- New file `internal/exec/exec.go`
- Define: `type CommandRunnerFunc func(name string, args ...string) ([]byte, error)`
- Provide a default: `var Default CommandRunnerFunc = func(name string, args
  ...string) ([]byte, error) { return osexec.Command(name, args...).CombinedOutput() }`
  (using `osexec` as import alias for `os/exec`)
- Add `internal/exec/` to the `foundation` layer paths in
  `docs/architecture.json`
- Update `docs/architecture.md` to list `exec` under foundation layer
- No callers updated in this issue — that's handled by subsequent issues

### Acceptance criteria

- [ ] `exec.CommandRunnerFunc` type exists with the specified signature
- [ ] `exec.Default` provides a working default implementation
- [ ] Foundation layer in `architecture.json` includes `internal/exec/`
- [ ] `architecture.md` lists `exec` under foundation layer

### Test cases

- **Default runs command**: `exec.Default("echo", "hello")` returns output
  containing "hello"
- **Type is assignable**: A custom function literal is assignable to
  `CommandRunnerFunc`

---

## Issue #401: Adopt `CommandRunnerFunc` in infrastructure and foundation packages

**Blocked by**: #385

### Description

Update the `var CommandRunner` declarations in infrastructure and foundation
layer packages to use the shared `exec.CommandRunnerFunc` type. This is a
mechanical type annotation change — the function literals and all callers remain
unchanged.

### Key constraints

- Modify `internal/github/issues.go` (line 44): add `exec.CommandRunnerFunc`
  type annotation to `var CommandRunner`
- Modify `internal/sandbox/build.go` (line 16): same change
- Modify `internal/config/config.go` (line 17): same change
- The function literal assigned to each var is unchanged — only the type
  annotation is added
- All callers are unchanged — the call syntax is identical

### Acceptance criteria

- [ ] `github.CommandRunner` is typed as `exec.CommandRunnerFunc`
- [ ] `sandbox.CommandRunner` is typed as `exec.CommandRunnerFunc`
- [ ] `config.CommandRunner` is typed as `exec.CommandRunnerFunc`
- [ ] All existing tests pass without modification

### Test cases

- **Type compatibility**: Existing test fakes (which are function literals with
  the same signature) remain assignable to the typed variables
- **Existing tests pass**: All tests in github/, sandbox/, config/ pass
  unchanged

---

## Issue #402: Adopt `CommandRunnerFunc` in domain and orchestration packages

**Blocked by**: #385

### Description

Update the `var CommandRunner` and `var GuardRunner` declarations in domain and
orchestration layer packages to use the shared `exec.CommandRunnerFunc` type.

### Key constraints

- Modify `internal/punchlist/punchlist.go` (line 13): add type annotation
- Modify `internal/orchestrator/orchestrator.go` (line 669): add type annotation
- Modify `internal/agent/guardrails.go` (line 16): add type annotation to
  `var GuardRunner` — same signature as `CommandRunnerFunc`

### Acceptance criteria

- [ ] `punchlist.CommandRunner` is typed as `exec.CommandRunnerFunc`
- [ ] `orchestrator.CommandRunner` is typed as `exec.CommandRunnerFunc`
- [ ] `agent.GuardRunner` is typed as `exec.CommandRunnerFunc`
- [ ] All existing tests pass without modification

### Test cases

- **Type compatibility**: Existing test fakes remain assignable
- **Existing tests pass**: All tests in punchlist/, orchestrator/, agent/ pass
  unchanged

---

## Issue #403: Refactor doctor.go to use `CommandRunnerFunc` with external timeout

**Blocked by**: #385

### Description

Update `internal/doctor/doctor.go` to use the shared `CommandRunnerFunc` type
(which does not take `context.Context`). The current 15-second per-check
timeout moves from `exec.CommandContext` inside each check closure to `Run()`,
which enforces the deadline externally by running each `c.run()` in a goroutine
with a timer.

### Key constraints

- Modify `internal/doctor/doctor.go`:
  - Change `var CommandRunner` (line 15) from
    `func(ctx context.Context, name string, args ...string) ([]byte, error)` to
    `exec.CommandRunnerFunc`
  - Default implementation: `exec.Default` (or inline equivalent without
    context)
  - Change `Check.run` field (line 27) from `func(ctx context.Context) bool`
    to `func() bool`
  - Remove `ctx` parameter from all check closure bodies (lines 59-123) —
    they now call `CommandRunner(name, args...)` without context
  - Update `Run()` (line 131): for each check, run `c.run()` in a goroutine,
    select on result channel vs 15-second timer; treat timeout as failure
- Update `internal/doctor/doctor_test.go` to match new signatures

### Acceptance criteria

- [ ] `doctor.CommandRunner` is typed as `exec.CommandRunnerFunc`
- [ ] `Check.run` field no longer takes `context.Context`
- [ ] `Run()` enforces 15-second timeout per check via goroutine + timer
- [ ] Timed-out checks are reported as failures
- [ ] All existing doctor tests pass (with updated fakes)

### Test cases

- **All checks pass**: When all commands succeed, `Run` returns true and
  prints `[PASS]` for each
- **Check failure**: When a command fails, `Run` returns false and prints
  `[FAIL]` with fix hint
- **Timeout handling**: When a check takes longer than the timeout, it is
  reported as `[FAIL]`
- **Fake injection**: Test fakes without context parameter are assignable to
  `CommandRunner`

---

## Issue #386: Define `OutcomeStatus` type and constants in loop.go

### Description

Replace the 43 bare string assignments to `outcome.Status` in `loop.go` with
typed constants. The `IssueOutcome.Status` field becomes `OutcomeStatus` type,
and four constants are defined. This catches typos at compile time and makes the
status vocabulary discoverable.

### Key constraints

- Modify `internal/agent/loop.go`:
  - Define `type OutcomeStatus string` near the `IssueOutcome` struct (line 22)
  - Define constants:
    ```go
    const (
        StatusImplemented      OutcomeStatus = "implemented"
        StatusReadyToMerge     OutcomeStatus = "ready-to-merge"
        StatusNeedsHumanReview OutcomeStatus = "needs-human-review"
        StatusFailed           OutcomeStatus = "failed"
    )
    ```
  - Change `IssueOutcome.Status` field type from `string` to `OutcomeStatus`
  - Replace all 43 string literal assignments with the named constants
- The JSON serialization of `OutcomeStatus` is unchanged (Go marshals string
  types as their underlying value)
- Consumer files (implement.go, orchestrator.go, etc.) are updated in a
  separate issue

### Acceptance criteria

- [ ] `OutcomeStatus` type and 4 constants are defined
- [ ] `IssueOutcome.Status` is typed `OutcomeStatus`
- [ ] All string literal status assignments in `loop.go` use named constants
- [ ] No bare `"implemented"`, `"ready-to-merge"`, `"needs-human-review"`, or
  `"failed"` strings remain in `loop.go`
- [ ] All existing loop tests pass

### Test cases

- **Constants have correct values**: Each constant's string value matches the
  original literal
- **JSON marshal unchanged**: `json.Marshal` of an `IssueOutcome` produces the
  same JSON as before
- **Existing loop tests pass**: All `loop_test.go` tests pass without
  modification (status comparisons still work because constants equal the
  original strings)

---

## Issue #404: Update outcome status consumers to use `OutcomeStatus` constants

**Blocked by**: #386

### Description

Update files outside `loop.go` that read or switch on `outcome.Status` to use
the named `OutcomeStatus` constants instead of string literals.

### Key constraints

- Modify `internal/cmd/implement.go` (lines 226-239): replace 3 string
  literals in the status switch with `agent.StatusImplemented`,
  `agent.StatusReadyToMerge`, `agent.StatusNeedsHumanReview`
- Modify `internal/orchestrator/orchestrator.go` (lines 369-387): replace 3
  string literals in status switch
- Modify `internal/dashboard/handlers.go` (lines 559-568): replace 4 string
  literals in badge rendering switch
- Modify `internal/rundata/writer.go` (lines 77-82): if the `Outcome` struct
  has a `Status string` field, consider whether it should also use
  `OutcomeStatus` or remain a string (it's serialized to JSON from run data).
  If it remains a string, no change needed. If it changes, update the 3
  references.
- All changes are mechanical: replace `"implemented"` with
  `agent.StatusImplemented`, etc.

### Acceptance criteria

- [ ] `implement.go` status switch uses `agent.OutcomeStatus` constants
- [ ] `orchestrator.go` status switch uses constants
- [ ] `dashboard/handlers.go` badge switch uses constants
- [ ] No bare status string literals remain in these files
- [ ] All existing tests pass

### Test cases

- **Implement summary output**: `implement` command still prints correct
  status summaries (existing test)
- **Orchestrator statistics**: Orchestrator still counts outcomes correctly
  (existing test)
- **Dashboard badges**: Badge CSS classes still render correctly (existing test)

---

## Issue #387: Define `MergeStrategy` type with validation

### Description

Replace inline string checks for auto-merge strategy values with a typed enum.
Currently `config.go` validates against string lists and `loop.go` switches on
bare strings. A named type with a `Valid()` method makes the vocabulary
explicit.

### Key constraints

- Modify `internal/config/config.go`:
  - Define `type FeatureMergeStrategy string` with constants: `MergeNone`,
    `MergeLowRisk`, `MergeAll`
  - Define `type RollupMergeStrategy string` with constants: `RollupNone`,
    `RollupManual`, `RollupAuto`
  - Add `Valid() bool` method on each type
  - Update `AutoMerge` struct fields to use the typed fields
  - Replace the inline validation switch (lines 358-368) with calls to
    `Valid()`
  - Update `applyDefaults()` to use the typed constants
- Modify `internal/agent/loop.go` (lines 408-420): replace string comparisons
  in the merge decision switch with the typed constants

### Acceptance criteria

- [ ] `FeatureMergeStrategy` and `RollupMergeStrategy` types are defined
- [ ] Constants exist for all valid values
- [ ] `Valid()` methods return false for invalid strings
- [ ] Config validation uses `Valid()` instead of inline string lists
- [ ] `loop.go` merge decision uses typed constants

### Test cases

- **Valid strategies accepted**: `"none"`, `"low_risk"`, `"all"` are valid
  `FeatureMergeStrategy` values
- **Invalid strategy rejected**: `"invalid"` returns `Valid() == false`
- **Config validation**: Config with `auto_merge.feature: "bad"` returns a
  validation error
- **YAML round-trip**: Marshal/unmarshal preserves strategy values
- **Loop merge decision**: Merge logic still triggers correctly for `MergeAll`
  and `MergeLowRisk` (existing tests)

---

## Issue #388: Extract `issueDir` method on `rundata.Writer`

### Description

Replace the 16 occurrences of `fmt.Sprintf("%d", issueNum)` in path
construction across `rundata/writer.go` with a single `issueDir` helper method.

### Key constraints

- Modify `internal/rundata/writer.go`:
  - Add unexported method: `func (w *Writer) issueDir(issueNum int) string`
    returning `filepath.Join(w.dir, "issues", strconv.Itoa(issueNum))`
  - Replace all 16 inline `filepath.Join(w.dir, "issues",
    fmt.Sprintf("%d", issueNum), ...)` calls with
    `filepath.Join(w.issueDir(issueNum), ...)`
- No signature changes to any exported methods
- The retry subdirectory pattern
  (`filepath.Join(..., "retries", fmt.Sprintf("%d", retryNum), ...)`) also
  benefits — consider an `issueRetryDir(issueNum, retryNum int) string` helper

### Acceptance criteria

- [ ] `issueDir` method exists and is used by all writer methods
- [ ] No remaining `fmt.Sprintf("%d", issueNum)` in path construction
- [ ] All existing writer tests pass

### Test cases

- **Path correctness**: `issueDir(42)` returns
  `<basedir>/issues/42`
- **Write methods produce same paths**: All JSON files are written to the same
  locations as before (existing tests)

---

## Issue #389: Group truncation limits into config struct

### Description

Consolidate scattered magic-number truncation constants into a single
`TruncationLimits` struct, making them discoverable and eventually
configurable.

### Key constraints

- Modify `internal/agent/verify.go` (line 41): move `verifyOutputLimit = 4096`
- Modify `internal/agent/punchlist.go` (line 28): move `maxPRDiffLen = 30_000`
- Define a `TruncationLimits` struct (in `internal/agent/` or
  `internal/config/`) with fields:
  - `VerifyOutput int` (default 4096)
  - `PRDiff int` (default 30000)
- Pass limits via existing config or as parameter to the functions that use them
- `internal/quality/risk.go` (lines 9-10) has `defaultMaxLines = 200` and
  `defaultMaxFiles = 10` — these are risk thresholds, not truncation limits;
  leave them in place

### Acceptance criteria

- [ ] `TruncationLimits` struct defined with `VerifyOutput` and `PRDiff` fields
- [ ] `verify.go` reads limit from struct instead of package-level const
- [ ] `punchlist.go` reads limit from struct instead of package-level const
- [ ] Default values match the originals (4096 and 30000)

### Test cases

- **Defaults applied**: When no custom limits are set, verify truncation uses
  4096 and PR diff uses 30000
- **Custom limits**: Overriding the struct values changes truncation behavior
- **Existing tests pass**: Verify and punchlist tests pass with default limits

---

## Issue #390: Extract unified verdict parser

### Description

`ParseReviewResult()` (guardrails.go:114-132) and `ParseQualityResult()`
(quality_reviewer.go:79-96) are near-identical — both scan lines for chained
`strings.Contains()` checks, differing only in one keyword ("REVIEW" vs
"QUALITY"). Extract a single parameterized parser.

### Key constraints

- New file `internal/agent/verdict.go`:
  - `func ParseVerdict(stdout, keyword string) string` — scans lines for
    `APPROVED` + keyword + `RESULT` or `CHANGES_REQUESTED` + keyword, returns
    `"APPROVED"`, `"CHANGES_REQUESTED"`, or `""`
  - Logic is identical to the existing functions (case-insensitive, first
    match wins, supports both `=` and whitespace-separated formats)
- Modify `internal/agent/guardrails.go`:
  - Replace `ParseReviewResult` body with `return ParseVerdict(stdout,
    "REVIEW")`
  - Keep `ParseReviewResult` as a thin wrapper for backwards compatibility
    (callers don't need to change)
- Modify `internal/agent/quality_reviewer.go`:
  - Replace `ParseQualityResult` body with `return ParseVerdict(stdout,
    "QUALITY")`
  - Keep `ParseQualityResult` as a thin wrapper
- Move all verdict-specific tests to `verdict_test.go`; wrapper tests stay in
  their existing test files

### Acceptance criteria

- [ ] `ParseVerdict(stdout, keyword)` exists in `verdict.go`
- [ ] `ParseReviewResult` delegates to `ParseVerdict` with `"REVIEW"`
- [ ] `ParseQualityResult` delegates to `ParseVerdict` with `"QUALITY"`
- [ ] All 11 existing verdict tests pass without modification

### Test cases

- **Approved with keyword**: `ParseVerdict("REVIEW_RESULT=APPROVED", "REVIEW")`
  returns `"APPROVED"`
- **Changes requested**: `ParseVerdict("QUALITY_RESULT=CHANGES_REQUESTED",
  "QUALITY")` returns `"CHANGES_REQUESTED"`
- **No match**: `ParseVerdict("some other output", "REVIEW")` returns `""`
- **First match wins**: Multiple verdict lines — first one is returned
- **Case insensitive**: Mixed-case input still matches
- **Colon format**: `REVIEW_RESULT: APPROVED` is accepted

---

## Issue #405: Update prompts to unified `AGENT_RESULT` verdict prefix

**Blocked by**: #390

### Description

Replace the divergent `REVIEW_RESULT=` and `QUALITY_RESULT=` sentinel prefixes
in prompt templates with a single `AGENT_RESULT=` prefix. Update the parser
keyword accordingly.

### Key constraints

- Modify `prompts/reviewer.txt`: replace all `REVIEW_RESULT=` references with
  `AGENT_RESULT=` (lines 57, 59, 92, 93)
- Modify `prompts/quality_reviewer.txt`: replace all `QUALITY_RESULT=`
  references with `AGENT_RESULT=` (lines 68, 69)
- Modify `internal/agent/guardrails.go`: update `ParseReviewResult` to call
  `ParseVerdict(stdout, "AGENT")`
- Modify `internal/agent/quality_reviewer.go`: update `ParseQualityResult` to
  call `ParseVerdict(stdout, "AGENT")`
- Update existing tests to use `AGENT_RESULT=` format in test fixtures

### Acceptance criteria

- [ ] `reviewer.txt` uses `AGENT_RESULT=APPROVED` / `AGENT_RESULT=CHANGES_REQUESTED`
- [ ] `quality_reviewer.txt` uses `AGENT_RESULT=` prefix
- [ ] Both parsers use `"AGENT"` as the keyword
- [ ] All verdict tests updated and passing

### Test cases

- **Reviewer approved**: Agent output containing `AGENT_RESULT=APPROVED` is
  parsed correctly by `ParseReviewResult`
- **Quality changes requested**: Agent output containing
  `AGENT_RESULT=CHANGES_REQUESTED` is parsed correctly by `ParseQualityResult`
- **Old format not matched**: `REVIEW_RESULT=APPROVED` is no longer matched
  (verifies clean break)

---

## Issue #391: Extract shared critical rules template variable

### Description

The "CRITICAL RULES" section is duplicated across 5 prompt templates with
subtle variations. Extract the universally shared subset (protected paths +
scenario dir protection) into a `{{.SharedRules}}` template variable generated
by Go code. Agent-specific rules (branch creation, ReviewDir mandates,
generated paths) remain in each prompt.

### Key constraints

- Modify `internal/agent/prompt.go`:
  - Add `SharedRules string` field to `PromptData` struct
  - Generate it in `newPromptData()` by assembling:
    - `Do NOT modify any protected paths: <paths>` (always present)
    - `Do NOT modify files in <ScenarioDir>` (when ScenarioDir is set)
  - Use `strings.Builder` or `fmt.Sprintf`, not a sub-template
- Modify `prompts/implementer.txt` (lines 36-38): replace shared rules with
  `{{.SharedRules}}`, keep branch creation and generated paths rules
- Modify `prompts/implementer_retry.txt` (lines 31-33): replace with
  `{{.SharedRules}}`, keep ReviewDir rule
- Modify `prompts/spec_generator.txt` (lines 18-19): replace protected paths
  rule with `{{.SharedRules}}`; keep the narrower "existing files" ScenarioDir
  rule as agent-specific (it intentionally differs from the shared version)
- Modify `prompts/reviewer.txt` (lines 39-40): replace shared rules with
  `{{.SharedRules}}`, keep generated paths and ReviewDir mandate rules
- Modify `prompts/verify_fix.txt` (line 15): replace protected paths rule with
  `{{.SharedRules}}`, keep "push to existing branch" rule
- Do NOT include ScenarioDir in `{{.SharedRules}}` for `spec_generator.txt` —
  it uses narrower wording ("existing files"). Instead, `spec_generator.txt`
  uses `{{.SharedRules}}` for protected paths only and keeps its own
  ScenarioDir line

### Acceptance criteria

- [ ] `SharedRules` field exists on `PromptData`
- [ ] `newPromptData()` populates `SharedRules` with protected paths and
  scenario dir rules
- [ ] All 5 prompts use `{{.SharedRules}}` for the shared subset
- [ ] Agent-specific rules remain in each prompt
- [ ] `spec_generator.txt` retains its narrower ScenarioDir wording

### Test cases

- **SharedRules contains protected paths**: When `ProtectedPaths` is set,
  `SharedRules` includes the protection line
- **SharedRules contains scenario dir**: When `ScenarioDir` is set,
  `SharedRules` includes the scenario dir line
- **SharedRules empty when no paths**: When both are empty, `SharedRules` is
  empty
- **Implementer prompt renders correctly**: Full render includes shared rules
  plus branch creation and generated paths
- **Spec generator keeps narrow rule**: Rendered spec_generator prompt contains
  "existing files" ScenarioDir wording separate from SharedRules

---

## Issue #392: Single-source scenario spec format

### Description

The scenario spec markdown format is defined identically in both
`prompts/spec_generator.txt` (lines 25-39) and
`internal/skills/godark-create-scenarios/SKILL.md` (lines 51-66). Deduplicate
by having the SKILL.md reference the prompt as the authoritative source.

### Key constraints

- Modify `internal/skills/godark-create-scenarios/SKILL.md`:
  - Replace the inline format example (lines 51-66) with a note directing the
    user to use the format defined in `prompts/spec_generator.txt`
  - Keep enough context that the skill is self-contained (brief summary of
    the format structure without the full example)
- `prompts/spec_generator.txt` is unchanged — it remains the authoritative
  format definition

### Acceptance criteria

- [ ] `SKILL.md` no longer contains a duplicate format specification
- [ ] `SKILL.md` references `prompts/spec_generator.txt` as the format source
- [ ] `spec_generator.txt` is unchanged
- [ ] SKILL.md still explains enough context to be usable without reading the
  prompt file

### Test cases

- **SKILL.md content**: Skill file contains a reference to
  `prompts/spec_generator.txt` and does not contain the full duplicated format
  block
- **Existing skills tests pass**: `godark_create_scenarios_test.go` still
  passes

---

## Issue #393: Extract non-blocking agent result handler in loop.go

### Description

The spec-gen (lines 79-110), recon (lines 123-148), and verify phases in
`loop.go` all follow the same pattern: check error → log warning + write hook →
continue; check timeout → log warning + write hook → continue; on success →
write hook. Extract a helper that handles this 3-way dispatch.

### Key constraints

- Modify `internal/agent/loop.go`:
  - New unexported function:
    ```go
    func handleNonBlockingResult(
        result *Result,
        err error,
        agentName string,
        logger *slog.Logger,
        writeHook func(rundata.StepResult) error,
    ) (resultText string)
    ```
  - Returns the agent's result text on success, empty string on error/timeout
  - Logs warnings for error and timeout cases
  - Calls `writeHook` in all three cases (error, timeout, success)
  - Handles the `hook != nil` guard internally
  - Refactor the spec-gen block (lines 79-110) to call this helper
  - Refactor the recon block (lines 123-148) to call this helper
  - The verify phase pattern is inside `runVerifyPhase` and has slightly
    different structure (per-module loop) — leave it for the review cycle
    extraction issues to address

### Acceptance criteria

- [ ] `handleNonBlockingResult` helper exists
- [ ] Spec-gen result handling uses the helper
- [ ] Recon result handling uses the helper
- [ ] Warnings are still logged for errors and timeouts
- [ ] Hook writes still occur in all three cases

### Test cases

- **Error case**: When `err` is non-nil, returns empty string and calls
  `writeHook` with error details
- **Timeout case**: When `result.TimedOut` is true, returns empty string and
  calls `writeHook` with timeout error
- **Success case**: Returns `result.ResultText` and calls `writeHook` with
  result data
- **Nil hook**: Does not panic when `writeHook` is nil
- **Existing loop tests pass**: ProcessIssue tests with spec-gen and recon
  still pass

---

## Issue #406: Extract drift-check and handoff policy helpers

**Blocked by**: #393

### Description

Extract two small helpers to reduce repeated patterns in `loop.go`:

1. A drift-check wrapper that consolidates the 7 repeated
   `checkDriftAndClose() + set outcome + return` blocks into a single call.
2. A handoff policy function that encapsulates the
   `attempt >= cfg.MaxResumeRetries` decision.

### Key constraints

- Modify `internal/agent/loop.go`:
  - New unexported function:
    ```go
    func driftGuard(baseSHA string, cfg *config.Config, prNum int, logger *slog.Logger) error
    ```
    Wraps `checkDriftAndClose` — same behavior, but callers can use a
    consistent pattern: `if err := driftGuard(...); err != nil { outcome.Status
    = StatusFailed; outcome.Err = err; return outcome }`
    Note: the 3-line return pattern must stay at call sites (Go has no
    exceptions), but giving it a clearer name improves readability.
  - New unexported function:
    ```go
    func shouldHandoff(attempt int, maxResumeRetries int) bool
    ```
    Returns `attempt >= maxResumeRetries`
  - Replace the 2 inline `if attempt >= cfg.MaxResumeRetries` checks
    (lines 278-281, 566-569) with `shouldHandoff(attempt,
    cfg.MaxResumeRetries)`
  - Rename `checkDriftAndClose` call sites to use `driftGuard` where it
    improves clarity (optional — only if the wrapper adds value beyond naming)

### Acceptance criteria

- [ ] `shouldHandoff` function exists and is used at both retry decision points
- [ ] Drift-check call sites use a consistent pattern
- [ ] All existing loop tests pass

### Test cases

- **shouldHandoff true**: `shouldHandoff(2, 2)` returns true
- **shouldHandoff false**: `shouldHandoff(1, 2)` returns false
- **shouldHandoff zero threshold**: `shouldHandoff(0, 0)` returns true (all
  retries use fresh mode)
- **Existing drift tests pass**: `TestCheckDriftAndClose_*` tests still pass

---

## Issue #407: Extract quality review cycle function

**Blocked by**: #406

### Description

Extract the quality review cycle (lines 239-350, ~111 lines) from
`ProcessIssue` into a dedicated function, flattening the nesting in the main
orchestration flow.

### Key constraints

- Modify `internal/agent/loop.go`:
  - New unexported function:
    ```go
    func runQualityReviewCycle(
        ctx context.Context,
        issue github.Issue,
        prNum int,
        baseSHA string,
        cfg *config.Config,
        prompts *Prompts,
        authEnv map[string]string,
        logger *slog.Logger,
        hook RunDataHook,
        sessionID *string,
    ) (passed bool, err error)
    ```
  - Move the `for qAttempt := 0; qAttempt < qualityMaxAttempts` loop and its
    body into this function
  - Return `(true, nil)` when quality review passes, `(false, err)` on
    failure or drift
  - `ProcessIssue` calls this function and maps the result to outcome status
  - The function uses `shouldHandoff` and `handleNonBlockingResult` from
    prior issues

### Acceptance criteria

- [ ] `runQualityReviewCycle` function exists
- [ ] Quality review loop is no longer inline in `ProcessIssue`
- [ ] `ProcessIssue` calls `runQualityReviewCycle` and handles its return
- [ ] All existing quality review tests pass

### Test cases

- **Quality approved first try**: Cycle returns `(true, nil)` when quality
  reviewer approves immediately
- **Quality approved after retry**: Cycle retries on CHANGES_REQUESTED and
  returns `(true, nil)` after fix
- **Quality exhausts retries**: Cycle returns `(false, nil)` when max attempts
  reached
- **Drift during quality**: Cycle returns `(false, driftErr)` when drift is
  detected
- **Existing ProcessIssue tests pass**: Full integration tests still pass

---

## Issue #408: Extract functional review cycle function

**Blocked by**: #407

### Description

Extract the functional review cycle (lines 352-628, ~276 lines) from
`ProcessIssue` into a dedicated function, completing the loop.go
simplification.

### Key constraints

- Modify `internal/agent/loop.go`:
  - New unexported function:
    ```go
    func runFunctionalReviewCycle(
        ctx context.Context,
        issue github.Issue,
        prNum int,
        branch string,
        baseSHA string,
        cfg *config.Config,
        prompts *Prompts,
        authEnv map[string]string,
        logger *slog.Logger,
        hook RunDataHook,
        sessionID *string,
        fixCycles *int,
    ) (status OutcomeStatus, prMerged bool, err error)
    ```
  - Move the `for attempt := 0; attempt < maxAttempts` loop into this function
  - Return status directly: `StatusImplemented` (merged), `StatusReadyToMerge`
    (approved, not merged), `StatusNeedsHumanReview` (max retries), or
    `StatusFailed` (error/drift)
  - Includes the merge decision logic, CI check wait, and verify phase calls
  - `ProcessIssue` calls this function and maps the result to outcome

### Acceptance criteria

- [ ] `runFunctionalReviewCycle` function exists
- [ ] Functional review loop is no longer inline in `ProcessIssue`
- [ ] `ProcessIssue` calls `runFunctionalReviewCycle` and sets outcome from
  return values
- [ ] All existing functional review tests pass
- [ ] `ProcessIssue` is significantly shorter (target: under 100 lines for the
  main body)

### Test cases

- **Approved and merged**: Returns `StatusImplemented` when review approves
  and merge succeeds
- **Approved not merged**: Returns `StatusReadyToMerge` when auto-merge is
  disabled
- **Changes requested then approved**: Retries and returns success after fix
- **Max retries exhausted**: Returns `StatusNeedsHumanReview`
- **Drift during review**: Returns `StatusFailed` with drift error
- **Existing ProcessIssue tests pass**: Full integration tests still pass

---

## Issue #394: Extract CLI flag parser helper

### Description

The 28-line flag parsing block in `run.go` (lines 30-57) is duplicated
verbatim in `implement.go` (lines 48-75). Extract a shared helper.

### Key constraints

- New helper in `internal/cmd/` (either in a new `cmdutil.go` file or added to
  an existing shared file):
  - `func parseCLIFlags(cmd *cobra.Command) config.CLIFlags`
  - Checks `cmd.Flags().Changed()` for: repo, max-retries, no-sandbox,
    auto-merge-feature, auto-merge-rollup, base-branch, default-branch
  - Returns a populated `config.CLIFlags` struct
- Modify `internal/cmd/run.go` (lines 30-57): replace inline parsing with
  `parseCLIFlags(cmd)` call
- Modify `internal/cmd/implement.go` (lines 48-75): same replacement

### Acceptance criteria

- [ ] `parseCLIFlags` helper exists
- [ ] `run.go` uses `parseCLIFlags` instead of inline flag checks
- [ ] `implement.go` uses `parseCLIFlags` instead of inline flag checks
- [ ] All existing command tests pass

### Test cases

- **All flags parsed**: When all flags are set, returned `CLIFlags` has all
  fields populated
- **No flags changed**: When no flags are explicitly set, returned `CLIFlags`
  has nil/zero fields
- **Partial flags**: Only changed flags are populated
- **Existing run tests pass**: `run` command behavior unchanged
- **Existing implement tests pass**: `implement` command behavior unchanged

---

## Issue #395: Consolidate config and tag/milestone resolution

### Description

`run.go` (lines 63-97) has inline tag/milestone resolution logic with deep
nesting that duplicates what `vet_helpers.go`'s `resolveTag` already does.
Consolidate by reusing the existing helper.

### Key constraints

- Modify `internal/cmd/run.go` (lines 63-97):
  - Replace the inline tag/milestone resolution with a call to `resolveTag(cmd)`
    from `vet_helpers.go`
  - `resolveTag` already handles mutual exclusivity of `--tag` and
    `--milestone`, repo resolution, and `ResolveMilestoneByTag`
  - If `resolveTag`'s error messages differ from `run.go`'s current messages,
    align them (prefer the clearer wording)
- Verify that `resolveTag` in `vet_helpers.go` handles the `--issue` flag
  validation that `run.go` currently does inline (if not, add it or keep that
  check in `run.go`)

### Acceptance criteria

- [ ] `run.go` no longer has inline tag/milestone resolution logic
- [ ] `run.go` calls `resolveTag` for milestone resolution
- [ ] Error messages are clear and consistent
- [ ] `--tag` and `--milestone` mutual exclusivity still enforced
- [ ] All existing run and vet tests pass

### Test cases

- **Tag resolves milestone**: `--tag v1.0` still resolves to the correct
  milestone via `resolveTag`
- **Mutual exclusivity**: `--tag` and `--milestone` together still error
- **Missing repo with tag**: `--tag` without `--repo` (and no config) still
  errors
- **Existing run tests pass**: All run command tests pass

---

## Issue #396: Extract vet data fetcher helper

### Description

Three vet commands repeat the same `FetchMilestoneIssues` +
`FetchAllIssueNumbers` fetch-and-error-handle block. `vet_scenarios.go` has a
conditional variant with a duplicated `FetchAllIssueNumbers` call. Extract a
shared helper.

### Key constraints

- Modify `internal/cmd/vet_helpers.go`:
  - Add function:
    ```go
    func fetchVetData(repo, milestone string) (
        issues []github.Issue,
        allNums map[int]bool,
        err error,
    )
    ```
  - When `milestone` is non-empty: fetch milestone issues AND all issue
    numbers
  - When `milestone` is empty but `repo` is non-empty: fetch all issue
    numbers only (issues slice is nil)
  - Wraps errors with context
- Modify `internal/cmd/vet_issues.go` (lines 26-37): replace with
  `fetchVetData` call
- Modify `internal/cmd/vet_roadmap.go` (lines 27-35): replace with
  `fetchVetData` call
- Modify `internal/cmd/vet_scenarios.go` (lines 24-46): replace conditional
  block with `fetchVetData` call

### Acceptance criteria

- [ ] `fetchVetData` helper exists in `vet_helpers.go`
- [ ] All three vet commands use `fetchVetData`
- [ ] `vet_scenarios.go` no longer has the conditional duplicate fetch
- [ ] Error messages are preserved
- [ ] All existing vet tests pass

### Test cases

- **Milestone mode**: `fetchVetData(repo, "Phase 1")` returns both issues and
  allNums
- **Repo-only mode**: `fetchVetData(repo, "")` returns nil issues and allNums
- **Fetch error**: When `FetchMilestoneIssues` fails, error is wrapped and
  returned
- **Existing vet tests pass**: All vet command tests pass

---

## Issue #397: Consolidate file scaffold functions with `writeFileWithDirs`

### Description

`init.go` and `new.go` both have duplicated `docFiles` and `promptFiles` loops
with repeated `os.MkdirAll` + `os.WriteFile` + error wrapping. Extract a
`writeFileWithDirs` helper and consolidate the scaffold loops.

### Key constraints

- Modify `internal/cmd/init.go`:
  - Extract `writeFileWithDirs(path string, data []byte) error` helper
    (creates parent dirs, writes file, wraps errors)
  - Consolidate `writeHarnessDocs()` (lines 187-260) to use the helper
- Modify `internal/cmd/new.go`:
  - Extract the shared `docFiles` and `promptFiles` definitions into a common
    location (either a shared function or package-level slices)
  - `writeNewProjectHarnessDocs()` (lines 144-191) delegates to the shared
    scaffold logic
  - The skip-if-exists behavior in `init.go` vs always-write in `new.go` must
    be preserved — the shared function takes a `skipIfExists bool` parameter

### Acceptance criteria

- [ ] `writeFileWithDirs` helper exists and is used by both commands
- [ ] `docFiles` and `promptFiles` are defined in one place
- [ ] `init.go` still skips existing files
- [ ] `new.go` still overwrites
- [ ] All existing init and new tests pass

### Test cases

- **writeFileWithDirs creates dirs**: Writing to `a/b/c.txt` creates `a/b/`
- **writeFileWithDirs writes content**: File contains expected bytes
- **Init skips existing**: Existing doc files are not overwritten
- **New always writes**: Files are written even if they exist
- **Existing init tests pass**: `init` command tests pass
- **Existing new tests pass**: `new` command tests pass

---

## Issue #398: Clean up punchlist parsing

### Description

Two punchlist-related files have fragile string parsing that can be simplified
with small helper functions:

1. `agent/punchlist.go` (lines 136-167): JSON extraction uses
   `strings.Index("[")` / `strings.LastIndex("]")` which matches unrelated
   brackets.
2. `punchlist/punchlist.go` (lines 192-215): checkbox and bullet parsing has
   duplicated `HasPrefix`/`TrimPrefix` chains.

### Key constraints

- Modify `internal/agent/punchlist.go`:
  - Extract `extractJSONArray(text string) (string, bool)` helper that:
    1. Strips markdown code fences if present
    2. Tries `json.Unmarshal` on the full text first
    3. On failure, finds the outermost `[...]` pair (not just first `[` and
       last `]` — track bracket depth)
    4. Returns the JSON substring and whether it was found
  - Replace the inline parsing at lines 136-167 with a call to this helper
- Modify `internal/punchlist/punchlist.go`:
  - Extract `extractPrefixedItem(line string, prefixes ...string) (string,
    bool)` helper that checks each prefix and returns the trimmed content
  - Replace the checkbox chain (lines 192-200) with
    `extractPrefixedItem(trimmed, "- [ ] ", "* [ ] ")`
  - Replace the bullet chain (lines 205-215) with
    `extractPrefixedItem(trimmed, "- ", "* ")`

### Acceptance criteria

- [ ] `extractJSONArray` handles code fences, plain JSON, and embedded JSON
- [ ] `extractJSONArray` uses bracket-depth matching instead of first/last index
- [ ] `extractPrefixedItem` replaces duplicated prefix chains
- [ ] All existing punchlist tests pass

### Test cases

- **Plain JSON array**: `extractJSONArray("[\"a\",\"b\"]")` succeeds
- **JSON in code fence**: `` ```json\n["a"]\n``` `` extracts correctly
- **JSON with surrounding text**: `"Here are tests: [\"a\"] done"` extracts
  `["a"]`
- **Nested brackets**: `"text [with [nested] brackets]"` handles depth
  correctly
- **No JSON found**: Returns `("", false)` for text without brackets
- **Checkbox dash**: `extractPrefixedItem("- [ ] foo", "- [ ] ", "* [ ] ")`
  returns `("foo", true)`
- **Checkbox star**: `extractPrefixedItem("* [ ] foo", "- [ ] ", "* [ ] ")`
  returns `("foo", true)`
- **No match**: `extractPrefixedItem("plain text", "- ")` returns
  `("", false)`

---

## Issue #399: Consolidate skills test helpers

### Description

Six skills test files each define a nearly identical `readXxxSkill` function
and share a `parseFrontmatter` function defined only in
`godark_define_architecture_test.go`. Consolidate into a single shared test
helper file.

### Key constraints

- New file `internal/skills/helpers_test.go`:
  - `func readSkill(t *testing.T, name string) string` — reads
    `skills.SkillFiles` for `<name>/SKILL.md`, returns content
  - `func parseFrontmatter(content string) string` — moved from
    `godark_define_architecture_test.go` (lines 21-27)
- Update 6 test files to use the shared helpers:
  - `godark_create_roadmap_test.go`
  - `godark_create_planning_doc_test.go`
  - `godark_configure_project_test.go`
  - `godark_define_architecture_test.go`
  - `godark_define_conventions_test.go`
  - `godark_create_phase_overview_test.go`
- Remove the per-file `readXxxSkill` functions (6 functions deleted)
- Remove the `parseFrontmatter` definition from
  `godark_define_architecture_test.go`

### Acceptance criteria

- [ ] `helpers_test.go` exists with `readSkill` and `parseFrontmatter`
- [ ] All 6 per-file `readXxxSkill` functions are removed
- [ ] All test files use `readSkill(t, "godark-xxx")` instead
- [ ] All existing skills tests pass

### Test cases

- **readSkill loads file**: `readSkill(t, "godark-create-roadmap")` returns
  non-empty content containing expected frontmatter
- **readSkill panics on missing**: `readSkill(t, "nonexistent")` calls
  `t.Fatalf`
- **All existing tests pass**: Full `go test ./internal/skills/...` passes
