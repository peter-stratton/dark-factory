# Phase 38: Security-Aware Review & Prompt Auditability

> **Goal:** The reviewer catches security anti-patterns before merge, every agent
> prompt is captured for post-hoc audit, and `godark trace --detail` renders the
> full decision chain for any issue as a single readable narrative.

## Milestone

`Phase 38: Security-Aware Review & Prompt Auditability`

---

## Issue 786: Security trace in semi-formal reviewer

### Description

Add a SECURITY TRACE section to the semi-formal reviewer prompt
(`prompts/reviewer_semiformal.txt`) that instructs the reviewer to scan each
changed file for security anti-patterns: hardcoded credentials, tokens without
TTL, sensitive data written to logs or shared caches, and new endpoints missing
authentication. Extend `CheckSemiformalConsistency` in
`internal/quality/quality.go` to catch FLAGGED-but-APPROVED contradictions, so
that a reviewer that flags a security issue but still approves triggers an
automatic re-run.

This issue touches 2 files across 2 layers: `prompts/reviewer_semiformal.txt`
(content) and `internal/quality/quality.go` (domain). The main complexity is
prompt wording that reliably triggers security scanning without overwhelming the
reviewer's token budget.

### Key constraints

- Insert the SECURITY TRACE section in `prompts/reviewer_semiformal.txt`
  between the existing UNCOVERED PATHS section and the FORMAL CONCLUSION
  section. The section must follow this format:
  ```
  ### SECURITY TRACE
  For each changed file, check for:
  - Hardcoded credentials, API keys, or secrets (variable names, string
    literals, config values that look like keys/tokens)
  - Tokens or sessions without expiration or TTL
  - Sensitive data written to logs, caches, or shared storage locations
  - New endpoints or data paths missing authentication or authorization checks
  For each finding, state: CLEAR / FLAGGED (with one-line description)
  ```
- Update the FORMAL CONCLUSION derivation rules to include:
  `If any security finding is FLAGGED -> CHANGES_REQUESTED`
- Update the CRITICAL verdict-matching instruction to include security flags
- In `CheckSemiformalConsistency` in `internal/quality/quality.go`, add a new
  check after the existing `Risk: HIGH` check:
  ```go
  if strings.Contains(output, "FLAGGED") {
      return &Flag{
          Code:    "semiformal_inconsistency",
          Message: "verdict APPROVED but security trace contains FLAGGED",
      }
  }
  ```
  This must only fire when `AGENT_RESULT=APPROVED` is also present (the
  existing early-return on line 138 already handles this)
- Do NOT add the SECURITY TRACE to the standard `prompts/reviewer.txt` - only
  the semi-formal variant. The standard reviewer does not have structured
  sections and adding one would be inconsistent
- The `prompts/` directory uses `//go:embed *.txt` so no changes to `embed.go`
  are needed

### Acceptance criteria

- [ ] `prompts/reviewer_semiformal.txt` contains a SECURITY TRACE section
      between UNCOVERED PATHS and FORMAL CONCLUSION
- [ ] SECURITY TRACE section checks for hardcoded credentials, tokens without
      TTL, sensitive data in logs/caches, and unauthed endpoints
- [ ] FORMAL CONCLUSION derivation includes security FLAGGED as a reason for
      CHANGES_REQUESTED
- [ ] `CheckSemiformalConsistency` returns a flag when APPROVED verdict
      contradicts a FLAGGED security finding
- [ ] `CheckSemiformalConsistency` still returns nil when no FLAGGED findings
      exist and verdict is APPROVED

### Test cases

- **FLAGGED with APPROVED**: Output with "FLAGGED" in security trace and
  `AGENT_RESULT=APPROVED` - returns flag with code `semiformal_inconsistency`
  and message mentioning security trace
- **CLEAR with APPROVED**: Output with only "CLEAR" security findings and
  `AGENT_RESULT=APPROVED` - returns nil (no contradiction)
- **FLAGGED with CHANGES_REQUESTED**: Output with "FLAGGED" and
  `AGENT_RESULT=CHANGES_REQUESTED` - returns nil (verdict is correct)
- **Template renders security section**: Render `reviewer_semiformal.txt` with
  populated PromptData - verify output contains "SECURITY TRACE" header

---

## Issue 787: Prompt capture in run data

**Blocked by**: #786

### Description

Add a `Prompt` field to `rundata.StepResult` and the `step_results` table in
stats.db so that the rendered prompt sent to each agent is persisted alongside
outputs and tool traces. This completes the audit chain that Phase 32 started -
you can already see what happened at each step, but not what instructions the
agent received.

This issue touches 5 files across 2 layers: `internal/rundata/writer.go` and
`internal/stats/schema.go` + `internal/stats/write.go` +
`internal/stats/types.go` (domain), and `internal/agent/rundata.go`
(orchestration). All changes follow established patterns for adding fields to
these structs.

### Key constraints

- Add `Prompt string `json:"prompt,omitempty"`` to `rundata.StepResult` (after
  the `TraceID` field on line 83 of `writer.go`)
- Add `Prompt string` to `stats.StepResultRecord` in
  `internal/stats/types.go` (after `TraceID`)
- Add the idempotent ALTER TABLE migration to `internal/stats/schema.go`:
  ```go
  `ALTER TABLE step_results ADD COLUMN prompt TEXT DEFAULT ''`,
  ```
  Append this to the existing `alterStmts` slice (after the `trace_id` alter)
- Update `doWriteStepResult` in `internal/stats/write.go` to include `prompt`
  in the INSERT column list and values
- Update `ResultToStep` in `internal/agent/rundata.go` to copy `Prompt` from
  a new field. Since `agent.Result` does not currently carry the rendered
  prompt, add `Prompt string` to `agent.Result` in
  `internal/agent/launcher.go` (after `CloneSHA` on line 55)
- The prompt is set by callers in `loop.go` after `RenderPrompt` returns but
  before `ResultToStep` is called. The rendered prompt variable (`rendered`)
  already exists at each call site. However, the prompt is rendered before
  `Run()` is called, and `Run()` returns the `Result` struct. The cleanest
  approach: set `result.Prompt = rendered` on the result immediately after
  `Run()` returns, at each call site. There are 6 distinct render+run sites in
  `loop.go` (implementer, quality reviewer, functional reviewer, verify-fix,
  recon, planner) plus retries. Each must set `result.Prompt = rendered` after
  the `Run` call
- Update the query functions that read step_results
  (`QueryStepResults`, `QueryStepsByTraceID`) in `internal/stats/query.go` to
  SELECT and scan the new `prompt` column
- Update `toStepResult` in `internal/stats/convert.go` to copy the `Prompt`
  field from `StepResultRecord` to `rundata.StepResult`
- The `Prompt` field uses `omitempty` in JSON tags so existing run data files
  without prompts remain backwards compatible
- Prompts can be large (5-20KB rendered). This is acceptable for JSON files on
  disk (they already store full agent output which is larger). For stats.db,
  the TEXT column handles this fine - SQLite has no practical text size limit

### Acceptance criteria

- [ ] `rundata.StepResult` has a `Prompt` field
- [ ] `agent.Result` has a `Prompt` field
- [ ] `ResultToStep` copies `Prompt` from `agent.Result` to `rundata.StepResult`
- [ ] `stats.StepResultRecord` has a `Prompt` field
- [ ] `schema.go` migration adds `prompt` column to `step_results`
- [ ] `doWriteStepResult` writes the prompt to the database
- [ ] `QueryStepResults` and `QueryStepsByTraceID` read the prompt column
- [ ] `toStepResult` in `convert.go` copies `Prompt` to `rundata.StepResult`
- [ ] Implementer run in `loop.go` sets `result.Prompt = rendered` after `Run`
- [ ] Reviewer runs in `loop.go` set `result.Prompt` after `Run`
- [ ] `go build ./...` passes
- [ ] `go test ./internal/agent/...` passes
- [ ] `go test ./internal/stats/...` passes
- [ ] `go test ./internal/rundata/...` passes

### Test cases

- **ResultToStep copies prompt**: Create an `agent.Result` with `Prompt` set -
  verify `ResultToStep` produces a `StepResult` with matching `Prompt`
- **ResultToStep empty prompt**: Create an `agent.Result` without `Prompt` -
  verify `StepResult.Prompt` is empty string
- **Schema migration idempotent**: Run `migrate` twice on the same database -
  verify no error on second run (duplicate column suppressed)
- **WriteStepResult round-trip**: Write a `StepResultRecord` with `Prompt` set,
  query it back via `QueryStepsByTraceID` - verify `Prompt` matches
- **JSON omitempty**: Marshal a `StepResult` with empty `Prompt` - verify
  `"prompt"` key is absent from JSON output

---

## Issue 788: Detailed trace rendering

**Blocked by**: #787

### Description

Extend `godark trace` with a `--detail` flag that walks the run data directory
and renders the complete decision chain for an issue as a single chronological
narrative. Currently `godark trace` shows a summary table (step name, duration,
cost, flags) from stats.db. With `--detail`, it reads the full JSON artifacts
from `~/.godark/runs/` and renders: rendered prompt (truncated), agent output
summary, tool trace highlights, quality flags, verify results, risk assessment,
dialogue entries, and final outcome.

This issue touches 3 files across 2 layers: `internal/cmd/trace.go` (cmd) and
`internal/rundata/reader.go` (domain, existing `LoadRun` method). The main
complexity is bridging from the trace_id/stats.db world to the filesystem-based
run data directory - which is straightforward because the `run_id` in stats.db
is the timestamp directory name, and the `repo` field gives owner/name.

### Key constraints

- Add `--detail` boolean flag to `traceCmd` in `internal/cmd/trace.go`
- When `--detail` is set, after resolving the trace_id and querying the outcome
  from stats.db, use the outcome's `RunID` to look up the run's `repo` from
  the `runs` table, split into owner/name, and pass to
  `rundata.Reader.LoadRun(owner, name, runID)` to get the full `RunDetail`
- Extract the `IssueDetail` for the target issue number from `RunDetail.Issues`
- Render each step in chronological order using this format:
  ```
  === RECON (2m30s, $0.0412) ===
  Prompt: <first 3 lines of rendered prompt, or "[not captured]" for old runs>
  Output: <first 500 chars of agent output, or "[empty]">
  Tool trace: <count> calls
  Flags: <comma-separated flag codes, or "none">

  === IMPLEMENT (8m15s, $0.2130) ===
  ...

  === VERIFY (attempt 0) ===
  Checks: build PASS, lint PASS, test PASS
  Fix attempted: no

  === QUALITY REVIEW (1m45s, $0.0318) ===
  ...

  === FUNCTIONAL REVIEW (3m10s, $0.0891) ===
  ...

  === RISK ASSESSMENT ===
  Low risk: yes
  Gates: lines_changed PASS, files_changed PASS, ...

  === OUTCOME ===
  Status: implemented
  PR: #742
  ```
- For retries, insert them in chronological order between the initial
  implementation and the final review
- Include dialogue entries inline where they chronologically fit
- Include judge interventions if present
- The `--detail` flag is incompatible with `--json` for now - if both are set,
  print an error. JSON detail output can be added later if needed
- If the run data directory does not exist (e.g., purged or old run), fall back
  to the standard summary view with a note: "Run data not found on disk;
  showing summary from stats.db"
- Add a `QueryRunByID` function to `internal/stats/query.go` that returns a
  single `RunRecord` by run_id (the timestamp). This is needed to get the
  `repo` field for locating the run data directory

### Acceptance criteria

- [ ] `godark trace <issue> --detail` renders the full decision chain
- [ ] Each step shows prompt (truncated), output summary, tool trace count,
      and flags
- [ ] Verify results show per-check pass/fail
- [ ] Risk assessment shows gate results
- [ ] Retries appear in chronological order
- [ ] Dialogue entries appear inline
- [ ] Falls back gracefully when run data directory is missing
- [ ] `--detail` and `--json` together produce an error message
- [ ] `QueryRunByID` exists in `internal/stats/query.go`
- [ ] `go build ./...` passes
- [ ] `go test ./internal/cmd/...` passes

### Test cases

- **Detail flag renders steps**: Build a run data directory with recon,
  implement, verify, and review steps - run `runTrace` with detail=true -
  verify output contains section headers for each step
- **Detail with prompt**: Build run data where `StepResult.Prompt` is set -
  verify the detail output includes prompt lines
- **Detail without prompt**: Build run data where `StepResult.Prompt` is empty -
  verify output shows "[not captured]"
- **Detail with retries**: Build run data with one retry cycle - verify the
  retry appears between implement and final review
- **Detail fallback on missing dir**: Query stats.db for a run whose directory
  has been deleted - verify output falls back to summary with a note
- **Detail and JSON error**: Pass both `--detail` and `--json` - verify error
  message
- **QueryRunByID found**: Insert a run record, query by ID - verify it returns
  the correct record
- **QueryRunByID not found**: Query a nonexistent run ID - verify nil return

---

## Integration chain audit

```
SECURITY TRACE section added to reviewer_semiformal.txt in Issue 1 (content)
  -> embedded via go:embed *.txt (automatic, no issue needed)
  -> loaded by LoadPrompts() (existing path, no change needed)
  -> rendered by RenderPrompt() (existing path, no change needed)
  -> output parsed by CheckSemiformalConsistency in Issue 1 (quality.go)
  -> flag checked by hasQualityFlag in runFunctionalReviewCycle (existing path, no change needed)
  -> triggers re-run on semiformal_inconsistency (existing path, no change needed)

Prompt field added to agent.Result in Issue 2 (launcher.go)
  -> set by callers in loop.go after Run() returns in Issue 2
  -> copied by ResultToStep in Issue 2 (rundata.go)
  -> written to JSON via WriteImplementResult/WriteReviewResult/etc (existing Writer methods, no change needed - they accept StepResult which now has Prompt)
  -> written to stats.db via doWriteStepResult in Issue 2 (write.go)
  -> read by QueryStepResults/QueryStepsByTraceID in Issue 2 (query.go)
  -> converted by toStepResult in Issue 2 (convert.go)
  -> read by rundata.Reader.readStep (existing path, no change needed - JSON unmarshal picks up new field)
  -> displayed by godark trace --detail in Issue 3 (trace.go)

QueryRunByID defined in Issue 3 (query.go)
  -> called by runTrace when --detail is set in Issue 3 (trace.go)
  -> provides repo field to split into owner/name for LoadRun

RunDetail loaded by LoadRun in Issue 3 (existing reader.go, no change needed)
  -> IssueDetail extracted by issue number in Issue 3 (trace.go)
  -> rendered by new renderTraceDetail function in Issue 3 (trace.go)
```

All hops covered. No gaps.
