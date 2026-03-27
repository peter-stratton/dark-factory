## Phase 27: Agent Efficiency & Resilience ⏸️ DEFERRED

**Goal**: Every agent step completes within its time budget, produces useful
output even on timeout, and never wastes time on tools it can't access. Recon
adapts its depth to issue complexity and codebase size. Prompts are audited
for tool/permission alignment across all roles.

**Milestone**: `Phase 27: Agent Efficiency & Resilience` | **Label**: `phase-27`

- Multi-pass recon with progressive detail — restructure recon prompt into
  prioritized passes (file list + drift → key snippets → pattern examples)
  so partial output is always useful
- Partial recon brief on timeout — capture and pass partial stdout to
  implementer when recon times out instead of discarding all work
- Adaptive recon depth by issue type — lightweight recon for wiring/refactor
  issues, deep recon for feature issues; optionally skip recon for
  well-specified issues
- Per-step timeout configuration — allow `agent_timeout` overrides per role
  (e.g. recon: 5m, spec_generator: 3m, implementer: 30m, reviewer: 15m)
- Prompt/permission audit — systematic scan of all prompts vs role permissions,
  remove any remaining instructions for unavailable tools
- Recon prompt generalization — remove Flutter/UI-specific language in favor
  of universal patterns, or make project-type-aware via template variables

**Issues**: #631–#635

**Planning doc**: `docs/planning/phase-27-agent-efficiency-and-resilience.md`

