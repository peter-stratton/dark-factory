## Phase 40: Meta-Agent Autonomous Mode

**Goal**: A `godark meta loop` command that iterates propose → apply →
benchmark → keep-or-revert on an isolated worktree, emitting a ranked branch of
winning experiments for human review. Rule-preserving by physical isolation, a
write allowlist, Phase 37 tripwires, and a final human promotion gate. The
meta-agent never commits to main; humans always own the merge step.

**Milestone**: `Phase 40: Meta-Agent Autonomous Mode` | **Label**: `phase-40`

**Depends on**: Phase 37 (Benchmarking Framework), Phase 39 (Meta-Agent Human Mode)

- Worktree isolation helper — `git worktree` create/clean for meta-agent experiments, never touching main
- Write allowlist enforcement — block writes outside a configured allowlist at the tool layer (inverted `ProtectedPaths`); default allowlist is `prompts/*.txt`
- `meta apply` subcommand — apply a proposal JSON to a worktree, produce a commit
- Experiment commit convention — structured commit trailers plus `experiment.json` sidecar per iteration
- `meta loop` subcommand — orchestrate propose, apply, benchmark, keep-or-revert; share the propose core from Phase 39
- Tripwire integration — consult Phase 37 `bench compare` and auto-revert experiments that trip cost cap, approval ceiling, model mix pin, or holdout-vs-primary divergence
- Budget and iteration caps — max iterations, max cumulative cost, max wall time; loop exits gracefully at any cap
- Audit log — per-loop structured log of every experiment (kept, reverted, tripped) for post-hoc review
- Ranked-branch output — final worktree branch with winning experiments ordered by score delta, formatted as a reviewable PR-style summary
- First autonomous loop run — against the Phase 37 benchmark repo; evaluate by eye before promoting anything
