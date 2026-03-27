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
  - **Complexity signal** - simple / moderate / complex / needs-splitting
- Output parsed by Go side to extract the complexity signal

### Pipeline integration
- New step in `ProcessIssue()` between recon and implementation:
  `recon -> planner -> implementer`
- Planner is a **blocking step** - if it fails or times out, implementation
  does not proceed (unlike recon which is non-blocking)
- "Needs-splitting" complexity signal pauses the pipeline and labels the
  issue `needs-human-review` with a comment explaining why
- Planner brief injected into implementer prompt via `{{.PlannerBrief}}`
  template variable, alongside existing `{{.ReconBrief}}`

### Agent function and config wiring
- `Plan()` function in `internal/agent/planner.go` following existing agent
  function pattern (`newRunOpts` + `Run()`)
- Add `Planner string` field to `Prompts` struct in config
- Timeout configurable via `timeouts.planner` in `godark.yaml` (default 5m)
- Optional: skip planner for issues labeled `skip-planner` or when issue
  body contains a pre-written plan section

### Run data and visibility
- `planner.json` per issue recording duration, cost, session ID, complexity
  signal, and the full planner brief
- Planner appears as a dedicated step in the dashboard issue detail timeline
  and TUI stage transitions: recon -> plan -> implement -> verify -> review
- Planner cost aggregated in run reports and `godark analyze`

