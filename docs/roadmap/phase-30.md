## Phase 30: Spec Tightening

**Goal**: The specification layer is strict enough that agents receive
unambiguous, testable requirements - and the system can detect what changed
at the requirements level when code lands. Hardens specs before Phase 31
introduces a planner that depends on spec quality.

**Milestone**: `Phase 30: Spec Tightening` | **Label**: `phase-30`

### Scenario spec structure enforcement
- Extend `godark vet scenarios` to require GIVEN/WHEN/THEN structure in
  scenario cases (currently only checks for `### <Case>` headings and bullet
  outcomes)
- Validate that every scenario case has at least one GIVEN, one WHEN, and
  one THEN clause
- Clear error messages with line numbers when structure is missing or malformed
- Update `/godark-create-scenarios` skill to emit GIVEN/WHEN/THEN format by
  default

### Spec delta generation
- After a PR merges, diff the scenario specs touched by the PR against their
  pre-merge state
- Generate a requirements-level delta: which requirements were added, changed,
  or removed
- Post the spec delta as a PR comment (alongside existing punchlist output)
- Store spec deltas in run data for dashboard display

**Issues**: #678-#682

**Planning doc**: `docs/planning/phase-30-spec-tightening.md`
