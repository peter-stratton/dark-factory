## Phase 35: Issue Completeness Auditing

**Goal**: After implementation, a dedicated audit step cross-references issue requirements against what was actually built, catching missing features, unwired UI elements, and gaps before the review stage.

**Milestone**: `Phase 35: Issue Completeness Auditing` | **Label**: `phase-35`

- Post-implementation audit agent — reads issue requirements and compares against changed files to identify unimplemented features
- Missing feature detection — flag when an issue implies UI work (screens, interactions, navigation) but no corresponding UI files were created or modified
- Wiring verification assertions — check that UI elements have event handlers, navigation routes are registered, and data sources are connected
- Audit report integration — surface completeness findings as structured output that feeds into the review stage, with clear pass/fail per requirement
