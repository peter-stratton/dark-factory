## Phase 9: Harness-Aware Agent Execution ✅

**Goal**: The harness files scaffolded and validated in Phase 8 are wired
into actual agent runs. Agents read architecture and conventions docs,
post structured dialogue on PRs, and the reviewer checks layer compliance
— all driven by the orchestrator, not just prompt template text.

**Milestone**: `Phase 9` | **Label**: `phase-9`

- Update architecture.json for dialogue package — add `internal/dialogue/`
  to domain layer paths
- Populate harness template variables in launcher — add `architecture_doc`
  and `conventions_doc` config fields with defaults; read file contents in
  `newPromptData()`; empty string for missing files (graceful degradation)
- Structured PR comment parser — new `internal/dialogue/` package; parse
  Implementation Notes and Review Notes from PR comment text into typed
  structs
- Wire agent dialogue into run data — `DialogueEntry` struct in rundata;
  orchestrator fetches PR comments after review cycles and writes
  `dialogue.json` per issue
- Surface agent dialogue in dashboard — dialogue timeline in issue detail
  view with expandable entries styled by role
- Architecture JSON context for reviewer — add `{{.ArchitectureJSON}}`
  template variable; reviewer gets structured layer definitions for
  compliance checking
- Configurable architecture enforcement — `enforce_architecture` config
  option; when enabled, reviewer must reject layer violations; when
  disabled (default), violations are informational only

**Issues**: #146–#152

**Planning doc**: `docs/planning/phase-9-harness-aware-agent-execution.md`

