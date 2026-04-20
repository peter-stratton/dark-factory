# Phase 37: Metric-Gaming Pre-Mortem

> **Purpose:** Before Phase 37 wires a scoring function into `bench compare`,
> adversarially attack it. Any metric that graduates to "primary" must survive
> the gaming vectors below, or be paired with a tripwire that catches them.
>
> **Threat model:** The adversary is a meta-agent editing prompts, skill
> configs, and harness templates in the godark repo between runs. The task
> agent in the sandbox is not the threat - it already runs against a hidden
> test suite. The meta-agent is the threat because it sees the score, and
> "the score" is whatever Phase 37 decides to emit.

---

## Proposed primary score

Phase 37 (Issue 784) outputs:

```json
{ "build_ok": true, "lint_ok": true, "total_tests": 25, "passed": 22,
  "failed": 3, "score": 0.88, "failures": [...] }
```

The default reading: `score = passed / total`, with `build_ok` and `lint_ok`
as hard gates that zero the score when false. Cost, duration, and retry
counts live in `stats.db` but are not part of the score.

This is a solid anti-gaming choice *for the task agent*, because the hidden
test suite is excluded via `sandbox.exclude`. It is not automatically safe
for the meta-agent, because the meta-agent sees the score, the failure
names, and the trajectory of many runs.

---

## Gaming vector table

| # | Category | Vector | Tripwire |
|---|----------|--------|----------|
| 1 | Hidden-test leakage | Failure names in the report ("TestAuthorDelete_BlockedByBooks") slowly describe the test suite. Over many iterations a meta-agent can optimize prompts that target named failures specifically. | Keep a second, never-surfaced test suite. Rotate primary tests between iteration windows. Never include test names in training context. |
| 2 | Reviewer rubber-stamping | Meta-agent rewrites the quality or functional reviewer to approve everything. Score unaffected if tests still run, but cost per merged PR explodes and real regressions reach main. | Track reviewer approval rate per harness_hash. Flag when it drifts above a ceiling (say, 95%). Inject known-bad PR fixtures and require the reviewer to bounce them. |
| 3 | Prompt bloat | Meta-agent adds "verify twice, write tests, run linters" everywhere. Score goes up; tokens/run triples. | Hard-cap total cost per issue at 2x baseline. Runs that exceed it are excluded from score improvement claims. |
| 4 | Model downgrade | Meta-agent rewrites `model_overrides` so every step uses the cheapest model. Score probably drops, but if cost becomes a metric later, this becomes attractive. | Pin the model mix as part of the harness hash inputs, or score every run at a fixed model config and evaluate cost separately. |
| 5 | Issue pipeline gaming | Meta-agent edits `create-issues` / `create-scenarios` prompts to emit simpler, more unambiguous issues from the same spec. Downstream implementation is easier, score rises, but the benchmark drifted. | Separate "skill pipeline hash" from "execution harness hash." Pin generated issues to a snapshot (Phase 37 already plans this) and forbid regeneration inside iteration windows. |
| 6 | Retry milking | If retries are cheap relative to first-try cost, meta-agent trains the implementer to produce lazy first attempts and rely on retry. Score holds, cost rises, wall time rises. | Track retry rate per harness. Cost cap (#3) also backstops this. |
| 7 | Flag suppression | Meta-agent rewrites reviewers to stop emitting `flags` on marginal code. Downstream quality-gating logic sees a cleaner stream than reality. | Flag counts should be tracked independently and compared across harnesses. Abrupt drops are suspicious. |
| 8 | Two-reviewer division of labor | Quality and functional reviewers were designed as independent perspectives. Meta-agent could specialize one into a compiler and the other into a rubber stamp, covering the score surface without matching intent. | Keep an independent human review sample. Track agreement rate between the two reviewers; near-zero disagreement or near-total disagreement are both red flags. |
| 9 | Dependency graph gaming | Meta-agent edits the dependency-resolution prompt to mark everything independent, enabling parallel wave execution that happens to fit the benchmark better than the sequential intent. | Pin issue dependency graph in the snapshot; compare the actual executed order against the snapshot order. |
| 10 | Build-ok / lint-ok pinpointing | `build_ok` and `lint_ok` as hard gates create a discontinuous reward. Meta-agent learns to prioritize "make it compile, make lint pass" over correctness once those gates are at risk. | Deliberate. This is arguably what we want. But track the ratio of "compiles-and-lints but fails many tests" vs "compiles-and-lints and passes many tests" to detect if prompts are drifting toward surface compliance. |
| 11 | Spec-delta cheating | Phase 32's `spec-delta.json` captures scenario changes. If spec-delta is used for scoring, meta-agent could game it by rewriting scenarios in the spec rather than the code. | Spec delta is diagnostic, not scoring. Keep it that way. |
| 12 | PR description stuffing | If any reviewer or merge coordinator reads the PR body for signal, meta-agent learns to include reassuring text there. | Reviewers should read code and tests only. Audit reviewer prompts for PR-body references. |

---

## Hard constraints (non-negotiable gates)

Score improvements that violate these are thrown out regardless of numbers:

1. **Cost cap.** Mean cost per merged issue must stay within 2x of baseline.
   Phase 37's `bench compare` already surfaces this; it needs to become a
   filter, not just a column.
2. **Reviewer approval ceiling.** Reviewer approval rate across a harness
   cannot exceed 95% without human audit of a random sample.
3. **Model mix pin.** The active model for each step is part of what's being
   optimized-against, not a free optimization variable. Changing `model_overrides`
   between iterations produces a new baseline, not an improvement claim.
4. **Snapshot lock.** Issues, dependency graph, and spec text are immutable
   per iteration window. Only prompts, harness scaffolding, and godark code
   are editable.

---

## Holdout scenarios

Phase 37 plans one benchmark repo with one test suite. The pre-mortem
recommends three additional held-out artifacts before declaring any
score improvement credible:

1. **Known-bad implementation fixture.** A hand-crafted implementation with
   a subtle bug (e.g., auth middleware that accepts any API key). Runs the
   reviewer in isolation and asserts `CHANGES_REQUESTED`. A reviewer that
   passes this without flagging is compromised. Not part of the main score;
   run on demand to audit reviewers.
2. **Secondary test suite, never surfaced.** A disjoint set of tests in a
   separate file, never included in failure names shown to the agent or
   logged to `~/.godark/stats.db`. Used to verify that score improvements
   on the primary suite generalize, not overfit to named failures.
3. **Adversarial issue set.** A handful of issues designed to tempt the
   specific gaming vectors above - a deliberately ambiguous spec (tests
   #5 and #9), an issue with an intentional dependency trap (#9), an issue
   whose best implementation requires flagging a scenario bug (#7).

---

## Metrics taxonomy

Three tiers with different roles. Phase 37 should emit all three, but only
one is the optimization target:

**Primary (what the loop optimizes):**
- `score = passed / total` on the primary hidden suite, subject to
  `build_ok && lint_ok`.

**Secondary (tracked per run, not optimized):**
- Cost per merged issue (USD)
- Reviewer approval rate (quality, functional, both)
- Retry rate
- Flag rate per review
- Model mix (fraction of steps per model tier)

**Tripwires (runs that hit these are excluded from improvement claims):**
- Cost > 2x baseline
- Reviewer approval rate > 95%
- Model mix drifted from pinned config
- Primary score improved but held-out secondary score did not

---

## Implications for Phase 37

The current plan is adequate as a v1 scoring infrastructure. The pre-mortem
surfaces the following additions that should land in Phase 37 rather than
bolted on later:

1. **Issue 781 (`bench compare`) should compute tripwire flags, not just
   deltas.** A run that improved the primary score but tripped a cost cap
   should be displayed differently from one that improved cleanly.
2. **Issue 784 (eval harness) should suppress test names in any artifact
   that could flow back to training.** The JSON score report can keep them;
   `stats.db`, `failure-analysis.json`, and the dashboard should redact or
   hash them.
3. **Add a secondary, never-surfaced test suite in `eval/` that the scoring
   harness runs but does not emit failure names for.** Used as a
   generalization check.
4. **Track `reviewer_approval_rate` per harness_hash in `stats.db` as part
   of the summary**, not derived at query time. Same for cost-per-merge
   and retry rate. These become first-class secondary metrics.
5. **A `godark bench audit` subcommand** (not yet planned) that runs the
   known-bad implementation fixture against the current reviewer prompts
   and asserts bounce. Called before any new harness is declared a winner.

Items 1 and 4 are small additions to existing issues. Items 2, 3, and 5
are new issues worth adding to Phase 37's scope.

---

## What this document is not

It is not a security analysis. The threat is a meta-agent optimizing for
a score, not an attacker exfiltrating data. Tripwires here exist to
preserve the *validity* of score claims, not to prevent a motivated
adversary from cheating. A human reviewing improvement claims remains
necessary.

It is also not exhaustive. New gaming vectors will surface as soon as
real meta-agent runs produce unexpected score jumps. This document should
be updated when that happens - a gaming vector caught in practice is
worth more than any number predicted on paper.
