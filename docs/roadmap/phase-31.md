## Phase 31: Planner Agent

**Goal**: A new agent sits between recon and implementation, producing a
design document and task breakdown before any code is written. The
implementer receives both the recon brief (what code exists) and the plan
(how to implement), reducing wasted iterations from wrong approaches.

**Milestone**: `Phase 31: Planner Agent` | **Label**: `phase-31`

### Prompt template and agent role
- `prompts/planner.txt` with template variables for issue body, recon brief,
  architecture doc, and conventions doc
- Agent permissions: read-only (`Read`, `Glob`, `Grep`) - the planner reasons
  about code but does not modify it
- Role name: `"planner"`, configurable via `prompts.planner` in `godark.yaml`

### Structured output
- Planner produces a structured brief with defined sections:
  - **Approach** - high-level implementation strategy (1-2 paragraphs)
  - **Key decisions** - architectural choices and trade-offs
  - **Task breakdown** - ordered implementation steps with file paths
  - **Risk flags** - anything surprising, ambiguous, or likely to cause
    review failures

### Pipeline integration
- New step in `ProcessIssue()` between recon and implementation:
  `recon -> planner -> implementer`
- Planner is a **non-blocking step** - if it fails or times out, a warning
  is logged and implementation proceeds without a plan (same pattern as recon)
- Planner brief injected into implementer prompt via `{{.PlannerBrief}}`
  template variable, alongside existing `{{.ReconBrief}}`

### Run data and visibility
- `planner.json` per issue recording duration, cost, session ID, and the
  full planner brief
- Planner appears as a dedicated step in the dashboard issue detail timeline
  and TUI stage transitions: recon -> plan -> implement -> verify -> review
- Planner cost aggregated in run reports and `godark analyze`
- Warning surfaced in dashboard log view when planner fails

**Issues**: #693–#695

**Planning doc**: `docs/planning/phase-31-planner-agent.md`
