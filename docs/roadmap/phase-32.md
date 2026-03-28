## Phase 32: Decision Flow Tracing

**Goal**: Every issue processed by Dark Factory gets a unique trace ID that
threads through all stages, making the full lifecycle — from recon through
merge — queryable and reconstructible as a single unit.

**Milestone**: `Phase 32: Decision Flow Tracing` | **Label**: `phase-32`

- Trace ID generation and propagation through ProcessIssue and hook/writer
- Schema extension — trace_id on step_results and issue_outcomes, new traces table
- JSON artifact annotation with trace_id and span_id fields
- `godark trace` CLI command for querying issue lifecycle by issue number or trace ID
- Dashboard trace view — trace ID header and copy action on issue detail page
- TUI trace column — truncated trace ID in the live issue table

**Issues**: #700–#704

**Planning doc**: `docs/planning/phase-32-decision-flow-tracing.md`
