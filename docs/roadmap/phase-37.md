## Phase 37: Security-Aware Review & Prompt Auditability

**Goal**: The reviewer catches security anti-patterns before merge, every agent
prompt is captured for post-hoc audit, and `godark trace --detail` renders the
full decision chain for any issue as a single readable narrative.

**Milestone**: `Phase 37: Security-Aware Review & Prompt Auditability` | **Label**: `phase-37`

- Security trace in semi-formal reviewer - add a SECURITY TRACE section to the reviewer prompt that checks for hardcoded credentials, tokens without TTL, sensitive data in logs/caches, and unauthed new endpoints; extend CheckSemiformalConsistency to flag FLAGGED-but-APPROVED contradictions
- Prompt capture in StepResult - add a Prompt field to StepResult and the step_results DB table so rendered agent prompts are persisted alongside outputs and tool traces
- Detailed trace rendering - extend `godark trace` with a --detail flag that walks the run data directory and renders the complete chain per issue: prompt sent, agent output summary, tool calls, quality flags, verdict, risk gates, and final outcome as a single chronological narrative

**Issues**: #786-#788

**Planning doc**: `docs/planning/phase-37-security-aware-review-and-prompt-auditability.md`
