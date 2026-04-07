## Phase 36: Structured Frontend Issue Decomposition

**Goal**: Frontend issues carry explicit, machine-readable UI requirements (screens, interactions, data bindings, navigation) so that agents and verification steps have concrete specs to build and audit against.

**Milestone**: `Phase 36: Structured Frontend Issue Decomposition` | **Label**: `phase-36`

- UI requirement schema — define a structured format for frontend issue requirements: screens/components, user interactions, data sources, navigation flows
- Issue template extensions — add optional UI-specific sections to issue templates that planner and spec generator can populate
- Planner agent UI awareness — planner recognizes when an issue has frontend implications and decomposes it with explicit wiring requirements
- Wiring checklist generation — automatically produce a checklist of expected connections (event handlers, API calls, state bindings) from structured requirements, used by both the implementing agent and the completeness auditor
