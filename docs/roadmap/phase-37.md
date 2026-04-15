## Phase 37: Benchmarking Framework

**Goal**: A repeatable, automated way to measure godark's speed, token efficiency,
and output quality across versions — using a Go REST API benchmark repo with a
hidden test suite, frozen issue snapshots, and a scoring pipeline.

**Milestone**: `Phase 37: Benchmarking Framework` | **Label**: `phase-37`

**Issues**: #778–#785

- Create benchmark Go REST API repo with starter scaffolding (server, health endpoint, one CRUD resource, conventions, CLAUDE.md)
- Write the feature spec (API surface, endpoints, behavior) that feeds into godark's skill pipeline
- Build the hidden integration test suite (lives in `eval/`, excluded from sandbox)
- Run godark skill pipeline against the spec to produce the first issue snapshot
- Build issue snapshot tooling — export issues to JSON, recreate from snapshot via `gh`
- Build the scoring harness — run hidden tests against agent output, collect pass rate, lint/vet results
- Build the metrics collector — capture wall time, token usage, cost per run
- Build the comparison reporter — tabular output comparing runs across snapshots and godark versions
- First baseline run with v0.23.0
