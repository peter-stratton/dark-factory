# Phase 37: Benchmarking Framework

> **Goal:** A repeatable, automated way to measure godark's speed, token
> efficiency, and output quality across versions — using a Go REST API benchmark
> repo with a hidden test suite, frozen issue snapshots, and a scoring pipeline.
>
> **Companion:** `docs/planning/phase-37-metric-gaming-pre-mortem.md`
> enumerates how a meta-agent editing godark's prompts could game the score
> and lists the tripwires, hard constraints, and holdouts that the issues
> below implement.

## Milestone

`Phase 37: Benchmarking Framework`

---

## Issue 778: Add model field to step tracking pipeline

### Description

Every step in the agent pipeline resolves a model from `cfg.Model` and
`cfg.ModelOverrides`, but that information is never persisted. Cost comparisons
across runs are meaningless without knowing which model produced the cost. Thread
the resolved model string from the agent through rundata into the stats DB.

This is purely additive — a new string field at each layer of the existing data
pipeline. No existing callers break. Despite touching ~6 files, each change is
1-2 lines following the established pattern.

Data flow: `agent.Result.Model` → `ResultToStep()` → `rundata.StepResult.Model`
→ `stepToRecord()` → `stats.StepResultRecord.Model` → `step_results.model`
column.

### Key constraints

- Add `Model string` field to `agent.Result` in `internal/agent/launcher.go`
- Set `Result.Model` from `RunOpts.Model` wherever `Result` is populated after
  an agent run (look for where `CostUSD` is set on Result — model goes there)
- Add `Model string` to `rundata.StepResult` in `internal/rundata/writer.go`
- Copy model in `ResultToStep()` in `internal/agent/rundata.go`
- Add `Model string` to `stats.StepResultRecord` in `internal/stats/types.go`
- Add idempotent migration in `internal/stats/schema.go`:
  `ALTER TABLE step_results ADD COLUMN model TEXT DEFAULT ''`
- Include model in INSERT in `doWriteStepResult()` in `internal/stats/write.go`
- Copy model in `stepToRecord()` in `internal/orchestrator/statswriter.go`

### Acceptance criteria

- [ ] `step_results` table has a `model` column
- [ ] After a run, each step record contains the resolved model name (e.g.
      "opus", "sonnet")
- [ ] Model overrides are reflected — a step using `model_overrides.recon:
      sonnet` shows "sonnet", not the default model
- [ ] Existing databases migrate without error (idempotent ALTER)

### Test cases

- **Model persisted**: Run with default model "opus"; verify step record has
  `Model == "opus"`
- **Override persisted**: Run with `model_overrides.recon: sonnet`; verify recon
  step has `Model == "sonnet"` while implement step has `Model == "opus"`
- **Migration idempotent**: Call migrate twice on the same DB; no error on second
  call

---

## Issue 779: Add sandbox.exclude config and sandbox directory exclusion

### Description

Add a `sandbox.exclude` list to `godark.yaml` that prevents specified
directories from being visible to agents inside the sandbox. After the repo is
cloned inside the container, excluded paths are removed from the working tree.

This is needed for the benchmarking framework (hide `eval/` from agents) but is
independently useful for any project with directories agents shouldn't see
(proprietary test suites, large data fixtures, sensitive configs).

### Key constraints

- Add `Exclude []string` field to the Docker config struct in
  `internal/sandbox/config.go`
- Parse from `godark.yaml` under `sandbox.exclude` (or `docker.exclude` —
  match whichever nesting the Docker config already uses)
- Implement exclusion in `CloneScript()` in `internal/sandbox/clone.go` by
  appending `rm -rf <path>` commands after the clone step for each excluded
  directory
- Validate that exclude paths are relative, don't escape the workspace (no
  `..`), and don't target critical files (`.git/`, `go.mod`, etc.)
- Exclusion happens inside the container — the host repo is unaffected

### Acceptance criteria

- [ ] `sandbox.exclude` is parsed from `godark.yaml`
- [ ] Excluded directories are not present in the sandbox workspace after clone
- [ ] Validation rejects paths containing `..` or absolute paths
- [ ] Omitting `sandbox.exclude` preserves current behavior (full clone)

### Test cases

- **Single exclusion**: Config with `exclude: ["eval/"]`; verify clone script
  contains `rm -rf` for eval directory
- **Multiple exclusions**: Config with `exclude: ["eval/", "fixtures/"]`; verify
  both are removed
- **Path traversal rejected**: Config with `exclude: ["../secret"]`; verify
  validation error
- **Absolute path rejected**: Config with `exclude: ["/etc/passwd"]`; verify
  validation error
- **Empty list**: Config with no `exclude` key; verify clone script is unchanged

---

## Issue 780: Build issue snapshot export and import tooling

### Description

Build a `godark bench snapshot` subcommand (or similar) that exports the current
GitHub issues for a milestone to a JSON file, and a `godark bench restore`
command that recreates those issues from a snapshot. This enables reproducible
benchmark runs — freeze the issues once, reuse across many runs.

The snapshot captures everything needed to recreate issues: title, body, labels,
milestone assignment, and dependency relationships. Restore creates fresh issues
and rewrites `Blocked by` references to point to the new issue numbers.

### Key constraints

- New package `internal/bench/` in the domain layer for snapshot types and logic
- Export: use `gh issue list` + `gh issue view` (or GitHub API via existing
  `internal/github/` client) to fetch issues for a milestone
- Snapshot JSON format:
  ```json
  {
    "repo": "owner/repo",
    "milestone": "Phase 37: Benchmarking Framework",
    "godark_version": "v0.23.0",
    "created_at": "2026-04-12T...",
    "issues": [
      {
        "number": 100,
        "title": "...",
        "body": "...",
        "labels": ["phase-37"],
        "depends_on": [99]
      }
    ]
  }
  ```
- Restore: create issues via `gh issue create`, then update bodies to rewrite
  `Blocked by #OLD` → `Blocked by #NEW` using the number mapping
- Store snapshots in the benchmark repo under `eval/snapshots/`
- Add `internal/bench/` to the domain layer in `docs/architecture.json`

### Acceptance criteria

- [ ] `godark bench snapshot` exports issues for a milestone to JSON
- [ ] `godark bench restore` creates issues from a snapshot JSON file
- [ ] Dependency references (`Blocked by`) are remapped to new issue numbers
- [ ] Snapshot includes godark version and timestamp metadata
- [ ] Restore is idempotent-safe — warns if issues with matching titles already
      exist

### Test cases

- **Round-trip**: Export issues, delete them, restore from snapshot; verify
  titles, bodies, and labels match
- **Dependency remapping**: Export two issues where B depends on A; restore;
  verify B's body references A's new issue number
- **Duplicate detection**: Restore the same snapshot twice; verify warning on
  second run, no duplicates created
- **Empty milestone**: Export a milestone with no issues; verify empty snapshot
  with valid metadata

---

## Issue 781: Add benchmark comparison report command

### Description

Add a `godark bench compare` command that queries `stats.db` to compare metrics
across runs. Given two or more run IDs (or a repo + milestone filter), produce a
tabular report showing per-step cost, duration, and model, plus totals,
secondary metrics, and tripwire flags.

This is the primary tool for evaluating whether a godark change improved
performance. The metric-gaming pre-mortem
(`docs/planning/phase-37-metric-gaming-pre-mortem.md`) establishes that raw
score deltas are not sufficient — a run that improved score while doubling
cost or tripping reviewer approval rate is not a clean win. `bench compare`
must surface tripwires prominently so improvement claims can be evaluated
against the full picture.

Output should be terminal-friendly (aligned columns) with an optional `--json`
flag for programmatic consumption.

### Key constraints

- New command in `internal/cmd/bench.go` (parent `bench` command with
  `snapshot`, `restore`, `compare` subcommands)
- Query logic in `internal/stats/query.go` — add functions to fetch step results
  by run ID and aggregate totals, plus aggregate functions for secondary metrics
  below
- Report formatting in `internal/report/` or inline in the command
- Primary comparison table columns: step name, model, cost (USD), duration (s),
  and delta (%) between runs
- **Secondary metrics section** displays per run and per delta:
  - Cost per merged issue (USD / implemented count)
  - Reviewer approval rate (APPROVED verdicts / all review verdicts)
  - Retry rate (retry steps / issue count)
  - Flag rate (total reviewer flags / review count)
  - Model mix (fraction of steps per model tier)
- **Tripwires section** evaluates against a baseline run and displays PASS/FAIL
  for each:
  - Cost cap: mean cost per merged issue within 2x of baseline
  - Approval ceiling: reviewer approval rate ≤ 95%
  - Model mix pin: model mix matches baseline (no silent downgrades)
- Tripwire thresholds are configurable via flags; defaults documented inline
- Runs that trip any tripwire display a visible warning in the terminal output
  and a `tripwires_failed` array in the `--json` output
- Support comparing 2+ runs side-by-side
- `--json` flag outputs structured JSON instead of table

### Acceptance criteria

- [ ] `godark bench compare <run-id-1> <run-id-2>` produces a tabular comparison
- [ ] Table shows per-step cost, duration, and model for each run
- [ ] Delta column shows percentage change between runs
- [ ] Secondary metrics section shows cost-per-merge, approval rate, retry rate,
      flag rate, model mix for each run
- [ ] Tripwires section evaluates cost cap, approval ceiling, and model mix pin
      and marks runs that fail them
- [ ] `--json` flag outputs machine-readable JSON including `secondary_metrics`
      and `tripwires_failed` arrays
- [ ] Graceful error when run IDs don't exist in stats.db

### Test cases

- **Two-run comparison**: Insert two run records with different costs; verify
  table shows correct deltas
- **Model difference highlighted**: Two runs where recon uses different models;
  verify both models appear in output
- **Cost cap tripwire**: Construct a run with cost per merge > 2x baseline;
  verify the cost tripwire fires and is listed in `tripwires_failed`
- **Approval ceiling tripwire**: Construct a run with 100% reviewer approval
  rate; verify the approval tripwire fires
- **Model mix tripwire**: Construct a run whose model mix differs from baseline;
  verify the mix tripwire fires
- **Clean comparison**: Two runs with similar costs and normal approval rates;
  verify no tripwires fire
- **Missing run**: Compare with a nonexistent run ID; verify error message
- **JSON output**: Compare two runs with `--json`; verify valid JSON with
  `secondary_metrics` and `tripwires_failed` fields

---

## Issue 782: Create benchmark Go REST API repo with starter scaffolding

### Description

Create a new GitHub repo (`peter-stratton/godark-bench-api` or similar) with a
minimal Go REST API that serves as the starting point for benchmark runs. The
scaffolding should establish clear conventions so agents have patterns to follow,
but leave enough unbuilt that the benchmark is meaningful.

### Key constraints

- Go module, standard library `net/http` (or a lightweight router like
  `chi` — pick one and commit to it in the conventions)
- Starter structure:
  ```
  main.go
  internal/server/server.go     # HTTP server setup, middleware chain
  internal/server/routes.go     # route registration
  internal/handler/health.go    # GET /health
  internal/handler/books.go     # full CRUD for Books (starter resource)
  internal/model/book.go        # Book struct
  internal/store/memory.go      # in-memory store (slice/map based)
  go.mod
  CLAUDE.md                     # project conventions for the agent
  godark.yaml                   # standard godark config
  ```
- Books CRUD already working: `GET /books`, `GET /books/{id}`,
  `POST /books`, `PUT /books/{id}`, `DELETE /books/{id}`
- In-memory store (no database) — keeps the benchmark focused on API logic
- CLAUDE.md describes conventions: handler patterns, error response format,
  store interface, naming
- godark.yaml configured with `sandbox.exclude: ["eval/"]`

### Acceptance criteria

- [ ] Repo exists on GitHub with the starter scaffolding
- [ ] `go build ./...` succeeds
- [ ] Books CRUD endpoints work (manually testable with curl)
- [ ] CLAUDE.md describes project conventions clearly
- [ ] godark.yaml is valid and configured for the benchmark repo

### Test cases

- **Build passes**: `go build ./...` exits 0
- **Health endpoint**: `GET /health` returns 200 with `{"status": "ok"}`
- **Books CRUD**: Create a book, fetch it, update it, delete it, verify 404
  after delete

---

## Issue 783: Write feature spec for benchmark API

### Description

Write the feature specification document that describes the target API surface
the benchmark should produce. This spec is the input to godark's skill pipeline
(`create-milestone` → `create-planning-doc` → `create-issues` →
`create-scenarios`). It lives in the benchmark repo under `eval/spec.md`.

The spec should describe features that exercise a realistic range of agent tasks:
greenfield resources, relationships between resources, query parameters,
middleware, input validation, and error handling.

### Key constraints

- Target features (building on the Books starter):
  - **Authors** CRUD (`GET/POST/PUT/DELETE /authors`, `/authors/{id}`)
  - **Books-Authors relationship** — each book has an `author_id` field;
    `GET /books` can filter by `?author_id=N`; author deletion blocked if books
    reference them
  - **Pagination** — `GET /books` and `GET /authors` support `?page=N&per_page=N`
    with pagination metadata in response
  - **Input validation** — required fields, string length limits, proper 400
    responses with field-level error messages
  - **API key auth middleware** — `X-API-Key` header required on mutating
    endpoints (POST/PUT/DELETE); reads are public
- Spec format: narrative markdown describing endpoints, request/response schemas,
  behavior, and error cases
- Spec must be detailed enough to produce good issues but not so prescriptive
  that it constrains implementation approach

### Acceptance criteria

- [ ] `eval/spec.md` exists with complete API surface description
- [ ] All five feature areas are specified with endpoints, schemas, and behavior
- [ ] Error cases and edge cases are documented
- [ ] Spec is self-contained — readable without access to the starter code

### Test cases

- N/A — this is a document, not code. Quality is validated by the hidden test
  suite covering all specified features.

---

## Issue 784: Build hidden eval test suite and scoring harness

### Description

Build the integration test suite and scoring harness that evaluates agent output.
This lives in `eval/` in the benchmark repo and is excluded from the sandbox via
`sandbox.exclude`. After a benchmark run, the eval harness starts the built API
server and runs HTTP tests against it.

The metric-gaming pre-mortem
(`docs/planning/phase-37-metric-gaming-pre-mortem.md`, vector #1) flags
hidden-test-name leakage as the sharpest gaming vector: if named failures
like `TestAuthorDelete_BlockedByBooks` flow into any artifact that a
meta-agent reads when optimizing prompts, the meta-agent can reverse-engineer
the hidden suite over many iterations. The scoring harness must keep raw
test names confined to the human-facing JSON report and never write them to
`stats.db` or any per-run artifact in `~/.godark/runs/`.

### Key constraints

- Test runner: Go test binary in `eval/tests/` using standard `testing` +
  `net/http` client
- Tests cover all features from the spec:
  - Authors CRUD (create, read, update, delete, list)
  - Books-Authors relationship (filter by author, prevent orphaning)
  - Pagination (page size, page number, metadata)
  - Input validation (missing fields, bad values, error format)
  - Auth middleware (rejected without key, accepted with key, reads public)
- Scoring harness (`eval/score.go` or `eval/cmd/score/main.go`):
  - Build the API server (`go build ./...` from the workspace root)
  - Start the server on a random port
  - Run the test suite against it
  - Collect: total tests, passed, failed, build success/failure, lint results
  - Output a JSON score report with raw names (human-facing only)
  - Output a secondary JSON record with hashed failure codes for ingestion
    into `stats.db` — raw test names never leave the `eval/` directory
- Score report format (human-facing, written to `eval/reports/`):
  ```json
  {
    "build_ok": true,
    "lint_ok": true,
    "total_tests": 25,
    "passed": 22,
    "failed": 3,
    "score": 0.88,
    "failures": ["TestAuthorDelete_BlockedByBooks", ...]
  }
  ```
- Redacted record (ingestion format, written to the run directory):
  ```json
  {
    "build_ok": true,
    "lint_ok": true,
    "total_tests": 25,
    "passed": 22,
    "failed": 3,
    "score": 0.88,
    "failure_hashes": ["a1b2c3d4", "e5f6a7b8", ...]
  }
  ```
- Failure hashes are 8-character SHA256 prefixes of the test name. Stable
  per binary, not reversible in practice, sufficient for counting and
  correlation.
- Test names must not appear in any `~/.godark/runs/<run>/` artifact,
  `stats.db` row, dashboard output, or container stdout that the agent
  could read.

### Acceptance criteria

- [ ] Test suite covers all five feature areas from the spec
- [ ] Scoring harness builds the server, runs tests, and produces both the
      human-facing JSON report and the redacted ingestion record
- [ ] Tests are independent (no ordering dependency between test cases)
- [ ] Score is a simple pass ratio: `passed / total`
- [ ] Build or lint failure is captured in the report (not a crash)
- [ ] Grep for raw test names in a sample run's artifacts produces zero hits
      outside `eval/reports/`

### Test cases

- **Perfect implementation**: Run eval against a hand-crafted golden
  implementation; verify 100% pass rate
- **Partial implementation**: Run eval against starter scaffolding (no new
  features); verify score reflects only Books CRUD passing
- **Build failure**: Run eval against code that doesn't compile; verify
  `build_ok: false` in report
- **Server start failure**: Run eval against code that builds but crashes on
  start; verify graceful failure in report
- **Name redaction**: After a run, grep `~/.godark/runs/` for a known test
  name from the suite; verify zero matches
- **Hash stability**: Hash the same failing test twice; verify identical
  8-char prefix

---

## Issue 786: Add held-out secondary test suite for generalization check

### Description

Build a second, disjoint test suite in the benchmark repo that is run by the
scoring harness but whose failure information is never surfaced in any form
— no names, no hashes, no counts per test. Only a single aggregate pass
ratio is emitted.

The metric-gaming pre-mortem
(`docs/planning/phase-37-metric-gaming-pre-mortem.md`, vector #1 and the
holdout scenarios section) establishes that the primary suite, even with
name redaction, still provides signal per-test (via failure hashes) that a
meta-agent can correlate over many runs. A held-out secondary suite whose
results are emitted only as a single number closes that channel and makes
overfitting detectable: if the primary score climbs but the secondary score
stagnates or drops, the meta-agent has learned shortcuts specific to the
primary suite rather than the underlying capability.

### Key constraints

- Second suite lives at `eval/holdout/` in the benchmark repo, excluded from
  the sandbox alongside the primary `eval/` via `sandbox.exclude`
- Tests cover the same five feature areas from the spec but with different
  fixtures, edge cases, and assertion phrasing — not a copy
- Scoring harness runs both suites in sequence
- Score report gains one new field: `holdout_score` (float, 0 to 1)
- No per-test information for holdout appears anywhere — no `holdout_failures`
  array, no `holdout_failure_hashes`, no counts. A run either has a
  `holdout_score` of 0.72 or doesn't; the meta-agent cannot tell which
  specific tests failed
- `bench compare` (Issue 781) treats `holdout_score` as a secondary metric
  and shows divergence from primary as a diagnostic

### Acceptance criteria

- [ ] `eval/holdout/` exists with a second test suite covering the five
      feature areas
- [ ] Scoring harness runs both suites and emits `holdout_score` in both
      the human-facing and ingestion records
- [ ] Holdout suite contains at least 15 tests (enough to be statistically
      meaningful, not so many that flakes dominate)
- [ ] No per-test details for the holdout suite appear in any artifact
- [ ] `bench compare` displays `holdout_score` alongside primary score and
      flags divergence (primary improved by > 5%, holdout did not)

### Test cases

- **Both suites run**: Run against the golden implementation; verify both
  `score` and `holdout_score` are present and near 1.0
- **Holdout opacity**: After a run, grep the run directory and `stats.db`
  for any holdout test name; verify zero matches
- **Divergence detected**: Construct two runs where primary score diverges
  from holdout; verify `bench compare` flags the divergence

---

## Issue 787: Add `godark bench audit` command with known-bad reviewer fixture

### Description

Build a `godark bench audit` subcommand that verifies the current reviewer
prompts still catch intentionally-bad implementations. It runs one or more
hand-crafted bad implementations past the quality and functional reviewers
in isolation and asserts each reviewer returns `CHANGES_REQUESTED`.

The metric-gaming pre-mortem
(`docs/planning/phase-37-metric-gaming-pre-mortem.md`, vector #2)
establishes reviewer rubber-stamping as the most insidious gaming vector:
a meta-agent that rewrites reviewer prompts to approve everything will
trip the approval-rate tripwire only if real task agents are producing
genuinely-bad output at scale. A direct audit against a known-bad fixture
catches the regression immediately, not two benchmark runs later.

This is a harness-validation tool, not part of the normal benchmark run.

### Key constraints

- New `audit` subcommand under `bench` in `internal/cmd/bench.go`
- Fixtures live at `eval/audit/fixtures/` in the benchmark repo — each
  fixture is a directory containing:
  - A flawed implementation (e.g. auth middleware that accepts any API key,
    a DELETE endpoint that ignores ownership, a pagination bug that returns
    all rows)
  - A `fixture.json` describing the flaw and the expected reviewer verdict
    (always `CHANGES_REQUESTED`), plus the expected flag codes
- Audit runs each fixture through the quality reviewer and the functional
  reviewer independently (not the full pipeline)
- Agent invocation reuses `internal/agent/` runner with the reviewer prompt
  but skips the implementer, tests, and merge-coordinator steps entirely
- Exit code 0 if every reviewer bounced every fixture; non-zero otherwise
- `--json` flag outputs structured audit results
- Audit result format:
  ```json
  {
    "passed": true,
    "fixtures": [
      {
        "name": "auth-accepts-any-key",
        "quality_verdict": "CHANGES_REQUESTED",
        "functional_verdict": "CHANGES_REQUESTED",
        "expected": "CHANGES_REQUESTED",
        "pass": true
      }
    ]
  }
  ```
- Intended to run before declaring any new harness a winner. Not scheduled
  automatically; an operator runs it.

### Acceptance criteria

- [ ] `godark bench audit` runs one or more audit fixtures through the
      quality and functional reviewers
- [ ] Exit code 0 iff every reviewer bounced every fixture
- [ ] At least three audit fixtures exist in `eval/audit/fixtures/`
- [ ] `--json` flag produces machine-readable output
- [ ] Fixtures run in isolation without touching GitHub or a real repo

### Test cases

- **All fixtures bounce**: Run audit with current reviewers; verify exit 0
- **Rubber-stamp reviewer fails audit**: Manually override the reviewer
  prompt to "always approve"; verify audit exits non-zero with clear
  messaging about which fixture was missed
- **JSON output**: Run audit with `--json`; verify schema matches the
  documented format

---

## Issue 785: Run first baseline benchmark with v0.23.0

**Blocked by**: Add model field to step tracking pipeline, Add sandbox.exclude
config and sandbox directory exclusion, Build issue snapshot export and import
tooling, Add benchmark comparison report command, Create benchmark Go REST API
repo with starter scaffolding, Write feature spec for benchmark API, Build
hidden eval test suite and scoring harness, Add held-out secondary test suite
for generalization check, Add `godark bench audit` command with known-bad
reviewer fixture

### Description

Execute the full benchmark pipeline end-to-end for the first time using godark
v0.23.0. This validates the entire framework and produces the baseline metrics
that all future comparisons will reference.

Steps:
1. Run godark skill pipeline against the feature spec to produce issues
2. Snapshot the issues as `v0.23.0-baseline.json`
3. Run `godark run` against the benchmark repo
4. Run the eval harness against the output (primary + holdout suites)
5. Run `godark bench audit` to validate reviewers still bounce bad fixtures
6. Record metrics (cost, duration, score, holdout score, secondary metrics)
   and note the baseline

This is a manual/operational task, not a code issue. It validates the framework.

### Key constraints

- Use godark v0.23.0 (the just-tagged release with bounded concurrency)
- Run the full skill pipeline: `create-milestone` → `create-planning-doc` →
  `create-issues` → `create-scenarios`
- Snapshot the generated issues before running
- Baseline documents the numbers that future runs' tripwires measure against:
  - Cost per merged issue (seeds the 2x cost cap)
  - Reviewer approval rate (seeds the 95% ceiling)
  - Model mix (seeds the model-pin tripwire)
- Document the baseline results somewhere accessible (benchmark repo README or
  a results file)

### Acceptance criteria

- [ ] Skill pipeline runs successfully against the benchmark repo
- [ ] Issue snapshot saved as `eval/snapshots/v0.23.0-baseline.json`
- [ ] godark run completes (success or failure — both are valid baseline data)
- [ ] Eval harness produces a score report with `score` and `holdout_score`
- [ ] `godark bench audit` exits 0 against v0.23.0 reviewers
- [ ] Baseline metrics documented: total cost, wall time, primary score,
      holdout score, cost per merged issue, reviewer approval rate, retry rate,
      flag rate, model mix

### Test cases

- N/A — this is an operational task, not testable code. Success is defined by
  the acceptance criteria.
