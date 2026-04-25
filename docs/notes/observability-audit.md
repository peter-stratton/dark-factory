# Observability & Telemetry Audit

An inventory of existing features in `godark` that support debugging failed runs,
auditing development session loops, and tracing agent execution. Intended as
source material for a new "Observability" sidebar section on the docs site.

**Scope:** what exists today, where it lives in the codebase, and where the
meaningful gaps are.

---

## 1. Logging

Mature. Structured logs are written per-run and per-issue.

- **Run-level:** `~/.godark/runs/<owner>/<repo>/<YYYYMMDD-HHMMSS>/debug.log`
- **Issue-level (Phase 14):** `~/.godark/runs/.../issues/<N>/debug.log`
- **Format:** JSON via `slog.JSONHandler`, plus colored text to stdout
- **Logger variants:** full / file-only / append
- **Features:** ANSI highlighting for verdicts and timeouts

**Gaps:** no log rotation, no remote shipping, no sampling. Unbounded disk
growth under `~/.godark/runs/`.

---

## 2. Tracing (Phase 32)

Mature. Trace IDs thread through the entire pipeline.

- **Generated:** `internal/agent/loop.go:75` via UUID
- **Propagation:** through `StepResult` → `VerifyStepResult` → `Outcome` → JSON → SQLite
- **Schema:** `trace_id` columns added to `step_results` and `issue_outcomes`
- **Query API:** `QueryStepsByTraceID()`, `QueryOutcomeByTraceID()`, `QueryLatestTraceForIssue()`
- **CLI:** `godark trace <issue|trace-id>` with text / JSON / detail modes
- **TUI:** shows first 8 chars of trace ID on issue completion
- **Dashboard:** renders trace ID in issue detail header with copy button

**Gaps:** no OpenTelemetry / OTLP export, no parent-child span hierarchy.

---

## 3. Run Analytics (SQLite)

Mature. Central source-of-truth for historical analysis.

- **Path:** `~/.godark/stats.db` (created at orchestrator startup)
- **Tables:**
  - `runs` — run metadata and outcome counts
  - `issue_outcomes` — per-issue status, error, PR number, `trace_id`, `clone_sha`
  - `step_results` — cost, duration, flags, resources, `trace_id`, prompt (capped at 32KB)
- **Record types:** `RunRecord`, `IssueOutcomeRecord`, `StepResultRecord`
- **Write path:** `WriteRunStats()` called post-run; failures are non-fatal
- **Filters:** repo, milestone, date range, exclusions

**Gaps:** no indexes, no cascading deletes, no retention policy, no error classification.

---

## 4. Cost / Duration / Outcome Tracking

Mature.

- **Per-step:** `cost_usd`, `duration_seconds`, `started_at`, `finished_at`
- **Resources (Phase 30):** `peak_memory_bytes`, `cpu_nanoseconds`
- **Aggregation:** `Aggregate()` computes cost totals, duration stats, flag frequencies, retry stats
- **Trends:** `ComputeTrends()` produces historical line-chart data
- **Per-issue cost:** `IssueCostUSD()` sums all step JSONs
- **Outcome values:** `implemented`, `ready-to-merge`, `needs-human-review`, `failed`

**Gaps:** no per-model breakdown, no anomaly detection, no burn-down charts, no
real-time cost during an active run.

---

## 5. Agent Dialogue / PR Comments

Mature.

- **Struct:** `DialogueEntry` (role, round, body, outcome)
- **Storage:** `issues/<N>/dialogue.json` (ordered array)
- **Capture:** GitHub PR API + direct agent outputs
- **Surface:** dashboard renders dialogue as a timeline in issue detail

**Gaps:** no real-time streaming, no comment threading, no explicit
agent-vs-human distinction.

---

## 6. CLI Inspection Commands

Complete coverage across the four main use cases.

| Command | Purpose |
|---------|---------|
| `godark trace <issue\|trace-id>` | Decision flow per issue (text / JSON / detail) — Phase 32 |
| `godark analyze` | Aggregate report: flags, retries, cost, gaps |
| `godark report` | Sprint summary (terminal / markdown / HTML) with LLM-written narrative |
| `godark status` | Web dashboard at `http://localhost:8374` — run list, issue detail, logs, analysis |

---

## 7. Error Handling & Failure Modes

Good.

- **`failure-analysis.json`** — pattern codes, counts, severity
- **`Outcome.error`** — final error message (searchable in SQLite)
- **`judge-interventions.json`** — per-rule judge intervention records
- **Retries** — captured as separate step records (`retry-N`, `retry-N-quality-review`)
- **`container-log.txt`** — raw container stdout/stderr
- **Resilience** — stats writes are non-fatal; run data is never corrupted on analytics failure

**Gaps:** no retry-budget alerts, no error cascade tracking, no deduplication.

---

## 8. Planning-Doc / Skill Outputs

Comprehensive per-step artifact capture.

- **Step outputs:** `recon.json`, `spec-generator.json`, `planner.json`, `implement.json`, `quality-review.json`, `functional-review.json`, `merge_coordinator.json`, `verify-N.json`, `risk-assessment.json`
- **Analysis artifacts:** `dialogue.json`, `failure-analysis.json`, `judge-interventions.json`, `container-log.txt`, `punchlist.json`, `spec-delta.json`
- **Wave tracking:** `waves/<N>.json`
- **Run-level:** `run.json` (metadata, dependencies, rate-limit state)
- **Prompts:** stored in `step_results.prompt` (32KB cap in SQLite) and in full in per-step JSONs
- **Punchlist:** verification steps, scenario cases, acceptance tests, changed files, enrichment status
- **Spec Delta:** before/after specs, added/removed/changed cases, setup changes

**Gaps:** no deduplication, no version control of artifacts, no compression.

---

## 9. Real-Time Monitoring

Good terminal + web coverage.

- **TUI (Bubble Tea):** live issue table, spinner-based progress, stage transitions, trace ID display, cost/retries per issue
- **Dashboard:** HTML + HTMX for partial updates, paginated log viewer, filters
- **Progress events:** `RunStarted`, `IssueStarted`, `IssueStageChanged`, `IssueCompleted`, `WaveStarted`, `RollupCreated`, `RunFinished`, `JudgeIntervention`, `RateLimited`, `WorkersActive`

**Gaps:** no WebSocket / SSE, no dashboard auto-refresh, no real-time log
streaming, no live cost counter during a run.

---

## 10. Existing Documentation

- **`docs/architecture.md`** — layer definitions; treats logging as foundational
- **Phase 32 planning doc** — full trace-propagation design (schema, CLI, dashboard, TUI)
- **Phase 14 planning doc** — per-issue logs, wave metadata
- **Phase 31 planning doc** — planner cost aggregation
- **`godark.yaml`** — no observability-specific config entries yet

---

## Summary

### Strengths

- Trace IDs thread end-to-end (Phase 32)
- SQLite analytics enable cost and duration trending
- Dual monitoring surfaces: TUI and web dashboard
- Rich artifact capture (prompts, plans, dialogue, failure analysis)
- Non-fatal analytics writes keep runs resilient
- Clear layer separation makes the telemetry path easy to reason about

### High-priority gaps

1. No OpenTelemetry export — can't plug into external APM
2. No Prometheus `/metrics` endpoint
3. No log rotation or retention — unbounded disk growth
4. No real-time cost tracking during a run
5. No failure-pattern-aware retry logic

### Medium-priority gaps

6. No error deduplication
7. No dashboard auto-refresh
8. No per-model cost breakdown
9. No anomaly detection
10. No artifact versioning

---

## Proposed Docs Sidebar Structure

```
Observability
├── Overview
├── Run Directories (~/.godark/runs/ layout)
├── Logs (debug.log, levels, slog format)
├── Tracing (trace IDs, godark trace)
├── Analytics Database (~/.godark/stats.db schema)
├── CLI Commands (trace, analyze, report, status)
├── TUI & Dashboard
├── Artifacts Reference (per-step JSON files)
├── Failure Analysis & Judge Interventions
└── Roadmap / Known Gaps
```
