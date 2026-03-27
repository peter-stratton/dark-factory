### Future considerations (not yet scoped)
- Acceptance criteria coverage validation - cross-reference issue ACs against
  scenario cases to verify coverage. Tricky because some ACs are inherently
  untestable via scenarios (e.g. "code compiles", "no unused imports"). Could
  flip the direction and validate that every scenario case traces back to an
  AC rather than requiring every AC to have a scenario case. Deferred from
  Phase 30 because GIVEN/WHEN/THEN enforcement and spec deltas provide most
  of the value without the annotation overhead
- Planner skip heuristic — skip the planner step for issues labeled
  `skip-planner` or when the issue body contains a pre-written plan section
  (e.g., detailed key constraints with file paths). Deferred from Phase 31
  because the planner is non-blocking and fast (~2-3 min), so the cost of
  always running it is low
- Planner complexity signal — parse planner output for a complexity assessment
  (simple/moderate/complex/needs-splitting) and auto-label issues that are too
  large. Deferred from Phase 31 because the splitting decision belongs in the
  planning workflow (/godark-create-planning-doc) where the user has context,
  not in an automated runtime check
- Configurable retry on judge Kill — currently only transport_failure retries
  the container; idle_timeout/no_progress/tool_thrash kills fail the step with
  no automatic retry (falls through to the normal max_retries review loop)
- Linter config generation from `architecture.json` (per-language)
- Multi-cluster deployment and geographic distribution
- Cost allocation and chargeback per team/repo
- Per-module change detection — diff PR changed files against module paths
  and only run build/test for affected modules and their dependents (currently
  all modules are built/tested unconditionally)
- Compose-based test infrastructure — `test_infra` config block for managing
  docker-compose lifecycle (setup/teardown) around the verify pipeline;
  deferred because `wait_for_checks` covers integration testing via CI
- Landing page and docs site
- Demo / example repo that people can point godark at to try it out
- GitLab support — godark currently assumes GitHub for everything: issue
  fetching, PR creation, review detection, label management, merge operations,
  and the `gh` CLI. Adding GitLab would require abstracting the VCS provider
  behind an interface (`internal/vcs/` or similar), implementing a GitLab
  client (likely using `glab` CLI or the GitLab API directly), and updating
  prompt templates that reference `gh` commands. Config would add a `provider:`
  field (`github` default, `gitlab` opt-in). Scope is significant — touches
  infrastructure, orchestration, and prompt layers — but the architecture
  already isolates GitHub calls in `internal/github/`
- Expanded distribution — add Windows builds, remove Linux arm64 ignore,
  add Scoop (Windows package manager), publish Docker images to GHCR, and
  optionally add AUR/Snap/DEB/RPM for broader Linux reach. All supported
  natively by GoReleaser except Winget (manual PR to winget-pkgs repo)
- Homebrew core inclusion (`brew install godark` without tap prefix)
- README badges — license, latest release, CI build status, test coverage, Go Report Card
- Quality review ROI evaluation — instrument overlap between quality reviewer
  and functional reviewer catches; consider merging into a single review pass
  if overlap is high
- Strategy agent for stuck retry loops — read-only LLM agent (distinct from
  the Go-side judge in Phase 28) that evaluates whether an implementation
  approach is working or going in circles; decides to retry with a different
  strategy, restart fresh, or escalate; implement only if retry data shows a
  persistent pattern of stuck loops that strictness decay doesn't resolve
- Docs site: add macOS `caffeinate` guidance — recommend `caffeinate -s godark
  run ...` for long/overnight runs to prevent macOS sleep from suspending
  Docker containers, dropping network connections, and stalling agent processes
- Golden evaluation dataset for prompt regression testing — a curated set of
  canonical issues (real or synthetic) with known-good implementations, stored
  in `tests/eval/`. A `godark eval` command runs agents against these issues
  and scores the results against baseline expectations. Primary use case is
  catching prompt regressions: run `godark eval` in CI after any change to
  `prompts/` and block the PR if scores drop below thresholds. Requires a new
  `internal/eval/` package, a scoring rubric (acceptance criteria coverage,
  verify pass rate, cost/duration within bounds), and a baseline snapshot to
  compare against. Start small — 20–30 issues spanning simple wiring, moderate
  features, and complex cross-cutting changes — and grow the dataset over time.
  Inspired by offline evaluation frameworks for LLM agents (three pillars:
  routing evaluation, LLM-as-judge scoring, and context grounding verification)
- Specification quality gate — a pre-implementation gate that scores issue
  readiness before spending compute. An agent evaluates whether the issue has
  sufficient detail, explicit acceptance criteria, and testable requirements.
  Weak specs get rejected or enriched before entering the pipeline. Could
  enforce structured issue templates that require testable acceptance criteria.
  Complements the existing `godark vet` validation but operates at runtime on
  individual issues rather than in bulk
- Eval as first-class contract — define issue-specific acceptance tests
  upfront (before or alongside spec generation) that the system explicitly
  targets, rather than relying on emergent evals from the reviewer agent.
  The eval becomes a hard contract: implementation iterates until the
  pre-defined acceptance test passes. Requires design work around authoring
  format, storage, and how it integrates with the existing verify and review
  pipeline. Distinct from scenario specs (which describe behavior) in that
  evals are executable pass/fail gates defined before implementation begins
- Production monitoring feedback loop — post-merge observation of deployed
  code to detect regressions and feed outcomes back into the pipeline. Could
  integrate with external metrics systems (Prometheus, Grafana, Datadog) to
  track whether dark-factory-built features cause production incidents, error
  rate spikes, or performance degradation. Enables quantifiable proof that
  the system produces production-quality code. Implementation depends heavily
  on the target deployment environment and observability stack. Could start
  simple (poll a health endpoint or error rate after merge) and grow toward
  richer integrations. Valuable for building organizational confidence in
  autonomous code generation
- Self-optimization from historical data — use accumulated run analytics
  (stats.db) to tune pipeline behavior automatically. Examples: issues of a
  certain type or complexity that fail review frequently get more recon/spec
  effort; prompts that correlate with higher success rates get preferred;
  retry budgets adjust based on historical pass rates. Requires server mode
  (Phase 15) for persistent cross-run state and enough data volume to draw
  meaningful conclusions. Likely a full milestone given the breadth of
  tuning surfaces and the need for guardrails against over-fitting to
  historical patterns
