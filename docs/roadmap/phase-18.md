## Phase 18: Adaptive Agent Loop ✅

**Goal**: The agent loop adapts to codebase drift within a run, recovers
intelligently from stuck retries, and produces better-informed implementations.
Issues late in a milestone execute as reliably as early ones because the system
accounts for changes made by prior issues.

**Milestone**: `Phase 18` | **Label**: `phase-18`

### Recon agent
- Recon agent prompt template and role — `recon.txt` prompt template, `recon`
  role with read-only permissions (`Read, Glob, Grep`), structured output
  format for supplemental implementation brief
- Recon config and prompt data wiring — `prompts.recon` config field,
  `ReconBrief` template variable on `PromptData`, implementer prompt updated
  to include the brief when present
- Recon orchestrator integration — invoke recon agent before `Implement()` in
  `ProcessIssue()`, pass output as implementer context, skip if not configured
- Recon run data and dashboard — persist recon brief to
  `~/.godark/runs/<run>/recon/` alongside other run data, write recon result
  (duration, cost, session ID) to run data, surface brief in issue detail view

### Hybrid retry strategy
- Fresh agent with structured handoff — on retry 3+, start a fresh agent
  session instead of resuming, pass PR comment dialogue (Implementation Notes
  / Review Notes) as structured handoff context
- Hybrid retry config — `max_resume_retries` config field (default 2), beyond
  which retries use fresh sessions with handoff artifact

**Issues**: #367–#372

**Planning doc**: `docs/planning/phase-18-adaptive-agent-loop.md`

