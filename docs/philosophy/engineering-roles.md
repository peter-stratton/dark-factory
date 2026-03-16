# Engineering Roles in a Godark Project

Godark changes what engineers do, not whether they're needed. The fundamental
shift: **humans control intent, constraints, and quality -- agents handle
implementation.** The traditional "write code, get reviewed" loop becomes
"define what to build, verify it was built correctly."

Every engineering level still has meaningful, high-leverage work. It's just
different work.

---

## Junior Engineer

**Traditional role:** Write code under supervision, learn patterns by doing.

**Godark role: Spec Writer and Verification Tester**

- **Write issue specs** -- translate planning doc descriptions into the
  structured format godark agents consume (Description, Acceptance Criteria,
  Test Cases). This is where juniors learn to think precisely about requirements
  -- a skill that transfers regardless of whether an agent or human writes the
  code.
- **Write scenario specs** -- create the `tests/scenarios/` files that define
  what "correct" looks like. Forces them to think about edge cases and expected
  behavior before any code exists.
- **Run punchlist verification** -- after agents complete PRs, juniors work
  through the generated punchlist, manually testing each item. This is where
  they learn the codebase -- by verifying behavior, not by writing boilerplate.
- **Review agent PRs with training wheels** -- review agent-generated diffs
  with `auto_merge: none`, learning to read code critically. They build code
  review skills on real diffs without the pressure of writing the code
  themselves.
- **Triage quality flags** -- when the dashboard shows flags like
  `no_diff_read` or `no_tests_run`, juniors investigate and file follow-up
  issues.

**What they learn:** Requirements thinking, test design, code reading,
architecture awareness (from reviewing diffs against `architecture.json`).
These are the foundations that make them mid-level.

---

## Mid-Level Engineer

**Traditional role:** Independently deliver features, participate in design.

**Godark role: Run Operator and Issue Designer**

- **Break milestones into issues** -- take a phase goal and decompose it into
  agent-sized work items. This is the `/godark-create-planning-doc` workflow:
  understanding the change surface, sizing tasks, defining dependencies.
  Requires knowing the codebase well enough to predict what the agent will
  touch.
- **Operate runs** -- kick off `godark run`, monitor the TUI and dashboard,
  handle the human-in-the-loop review cycle. When an agent's PR needs changes,
  write clear review feedback that the agent can act on. Knowing how to
  communicate intent to an agent is a distinct skill.
- **Tune prompts** -- when agents make recurring mistakes (visible via
  analytics: retry rates, quality flags), modify the prompt templates in
  `prompts/` to fix the pattern. This is iterative and data-driven -- use
  `godark analyze` to find the gap, adjust the prompt, run again, measure.
- **Configure project harnesses** -- run `/godark-define-architecture` and
  `/godark-define-conventions` for new projects the team is onboarding. Set up
  `godark.yaml` with the right runtime, build commands, and verification
  pipeline.
- **Write and maintain CLAUDE.md** -- keep the control document current as the
  project evolves. This is the highest-leverage file in a godark project -- it
  shapes every agent invocation.

**What they learn:** System design at the specification level, prompt
engineering, operational thinking (monitoring, analytics, feedback loops).
These are the foundations that make them senior.

---

## Senior Engineer

**Traditional role:** Own architecture, mentor, make technical decisions.

**Godark role: Architect and Quality System Designer**

- **Design architecture layers** -- define `architecture.json`, decide what can
  depend on what, draw the boundaries that agents must respect. This is the
  highest-leverage work in a godark project. Good layers mean agents produce
  clean code. Bad layers mean every PR fights the structure.
- **Write conventions** -- define `conventions.md` with patterns that are
  explicit enough for agents to follow. The godark principle applies: "never
  send an LLM to do a linter's job." Seniors decide what goes in conventions
  (explicit patterns), what goes in linters (automated enforcement), and what
  goes in prompts (behavioral guidance).
- **Design the verification pipeline** -- configure the `verify:` block in
  `godark.yaml`, decide which checks run, set failure thresholds, design the
  auto-fix cycle. This is the quality system -- the deterministic backstop
  behind the probabilistic agents.
- **Set auto-merge policy** -- decide the `auto_merge` strategy per repo. This
  is a trust calibration: `none` for new projects, `low_risk` once the
  pipeline proves reliable, `all` when the harness is mature. Seniors own this
  graduation.
- **Evaluate agent effectiveness** -- use `godark analyze` trends to assess
  whether the agent pipeline is improving or degrading. When retry recovery
  rate drops or cost per issue spikes, diagnose whether it's a prompt problem,
  a spec problem, or an architecture problem.
- **Design roadmap phases** -- the `/godark-create-milestone` workflow. Seniors
  decide what gets built, in what order, with what dependencies. The strategic
  layer that everything else flows from.
- **Own risk assessment** -- configure risk thresholds (`max_lines`,
  `max_files`, `protected_paths`), define which paths are protected, decide
  what constitutes "low risk" for auto-merge.

**What they provide:** The constraints that make agents effective. An agent
with a well-designed architecture, clear conventions, and a solid verification
pipeline produces better code than most humans writing under time pressure.

---

## QA Engineer

**Traditional role:** Test planning, manual testing, automation, bug triage.

**Godark role: Scenario Engineer and Quality Analyst**

- **Own scenario specs** -- write and maintain the `tests/scenarios/` corpus.
  These are not just test cases -- they are the behavioral contract that the
  functional reviewer agent validates against. Good scenarios catch bugs before
  humans ever see the PR.
- **Design verification commands** -- define `build_command`, `test_command`,
  `lint_command` in `godark.yaml`. Ensure the deterministic pipeline catches
  what agents miss. Evaluate whether existing tests have sufficient coverage
  for agent-generated code.
- **Monitor quality analytics** -- own the dashboard's analysis page. Track
  flag frequencies, retry rates, verify check failures. When
  `no_review_tests_written` spikes, investigate whether scenario specs are
  missing or the reviewer prompt needs adjustment.
- **Validate auto-merge candidates** -- for repos using `low_risk` auto-merge,
  QA reviews the risk assessment criteria. Are the thresholds right? Are edge
  cases slipping through? This is the human audit of the automated trust
  system.
- **Run acceptance testing from punchlists** -- the generated punchlists are
  QA's checklist. For PRs that need human review, QA works through the
  verification steps, scenario cases, and acceptance tests.
- **Evaluate review quality** -- are quality reviews and functional reviews
  catching different things, or is there overlap? Should they be merged into
  one pass? QA owns this analysis.

**What they provide:** The quality signal that the system learns from. Good
scenario specs lead to better functional reviews, fewer retries, and lower
cost. QA's work directly improves agent effectiveness over time.

---

## The Shift

In a traditional engineering team, code writing is distributed across all
levels. In a godark team, **code writing is centralized in agents**, and human
work shifts upstream (specification, architecture, constraints) and downstream
(verification, monitoring, tuning).

This is not about replacing engineers. It is about letting each level focus on
the work that requires their judgment:

| Level | Traditional focus | Godark focus |
|-------|------------------|--------------|
| Junior | Write simple code | Write specs, verify behavior |
| Mid | Write complex code | Design issues, operate runs, tune prompts |
| Senior | Design systems | Design constraints that make agents effective |
| QA | Test after the fact | Define correctness before agents start |

The common thread: **every role is about making the agent more effective**, not
about doing the agent's job. The better the specs, architecture, conventions,
and scenarios -- the fewer retries, the lower the cost, the higher the
auto-merge rate.
