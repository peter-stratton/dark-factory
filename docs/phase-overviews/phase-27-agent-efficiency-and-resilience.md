# Phase 27: Agent Efficiency & Resilience

Phase 27 was originally scoped as a set of prompt-level and timeout improvements to make agent steps more efficient and resilient. It was deferred when the Container Health Judge (Phase 28) solved the core timeout problem from the infrastructure side, and subsequent phases addressed the remaining goals through different mechanisms. This overview documents how each Phase 27 goal was ultimately fulfilled across the codebase, since no single phase landed these features as a unit.

---

## Per-Role Timeout and Health Monitoring

**What it does:** Rather than a single global timeout, each agent role gets tailored health thresholds. The judge's `idle_timeout_by_role` and `no_progress_timeout_by_role` maps in `godark.yaml` let you set different patience levels per role - a recon agent that goes silent for 3 minutes is likely stuck, but an implementer might legitimately think for 5 minutes.

**Example:** The project's judge configuration gives the recon role a shorter leash than the implementer:

```yaml
judge:
  idle_timeout_by_role:
    recon: 180
    implementer: 300
    reviewer: 600
  no_progress_timeout_by_role:
    implementer: 1200
```

The config struct in `internal/config/config.go`:

```go
type Judge struct {
    Enabled                   *bool          `yaml:"enabled"`
    IdleTimeoutByRole         map[string]int `yaml:"idle_timeout_by_role"`
    DefaultIdleTimeout        int            `yaml:"default_idle_timeout"`
    NoProgressTimeoutByRole   map[string]int `yaml:"no_progress_timeout_by_role"`
    DefaultNoProgressTimeout  int            `yaml:"default_no_progress_timeout"`
    // ...
}
```

This replaced the original Phase 27 plan for per-step timeout configuration. Instead of hard container timeouts per role, the judge provides real-time health monitoring with per-role sensitivity - a more granular solution that catches stalls in seconds rather than waiting for a timeout to expire.

---

## Partial Output Preservation on Timeout

**What it does:** When the judge kills a container but the agent has already produced useful output (`ResultText` is non-empty), the kill is treated as benign and the result is used normally. This handles the common case where an agent finishes work then idles during container wind-down.

**Example:** In `handleJudgeIntervention` in `internal/agent/loop.go`:

```go
// Kill with usable result: the agent completed work before going idle.
// Proceed normally - the kill was benign.
if result.ResultText != "" {
    logger.Info("judge killed container but result is usable, proceeding",
        "issue_number", issueNum, "step", step)
    return false
}
```

The original Phase 27 plan called for capturing partial recon stdout on timeout. The benign kill pattern achieves this naturally - if a recon agent outputs its brief before going idle, that output is preserved and passed to the implementer even though the container was killed. No special partial-output plumbing was needed.

---

## Model Overrides Per Role

**What it does:** The `model_overrides` map in `godark.yaml` lets you assign different models to different agent roles, controlling both cost and capability. Lightweight roles like recon and spec generation use a cheaper model, while the implementer uses the most capable one.

**Example:** The project's configuration uses Sonnet for reconnaissance and spec generation, keeping Opus for the implementer:

```yaml
model: opus
model_overrides:
  recon: sonnet
  spec_generator: sonnet
  quality_reviewer: sonnet
```

In `newRunOpts` in `internal/agent/implementer.go`, the override is applied when building run options:

```go
model := cfg.Model
if m, ok := cfg.ModelOverrides[role]; ok {
    model = m
}
```

This addresses the Phase 27 goal of adaptive recon depth by issue complexity - the recon agent runs on a faster, cheaper model because its job is gathering context, not writing code. The model selection applies to every agent function that calls `newRunOpts`, which is all of them.

---

## Handoff Context for Degraded Sessions

**What it does:** When session resumption retries are exhausted, the system assembles a handoff context - a synthesized summary of what was tried, what failed, and what the reviewer said - so a fresh agent session can pick up where the degraded session left off without re-reading the entire history.

**Example:** In the functional review retry loop in `internal/agent/loop.go`:

```go
var handoff string
if shouldHandoff(attempt, cfg.MaxResumeRetries) {
    handoff = assembleHandoffContext(cfg.Repo, prNum, logger)
}
```

The `shouldHandoff` function determines when to switch strategies:

```go
func shouldHandoff(attempt int, maxResumeRetries int) bool {
    return attempt >= maxResumeRetries
}
```

This was built as part of Phase 18 (Adaptive Agent Loop) and directly addresses the Phase 27 concern about useful output even when retries accumulate. Rather than trying to make individual steps more resilient, the system has a structured fallback path: session resumption first, then handoff context when sessions degrade.

---

## Generalized Recon Prompt

**What it does:** The recon prompt in `prompts/recon.txt` uses universal language that works across any project type - Go CLIs, web apps, mobile projects. It references architecture layers, dependency wiring, and shell/navigation structures in generic terms rather than framework-specific ones.

**Example:** The recon prompt instructs the agent to gather five categories of verbatim code snippets:

```
1. Domain models referenced by the issue
2. Repository interfaces / data-access classes
3. One example of the same type of artifact being built
4. Provider / dependency wiring
5. Shell / navigation wiring
```

Each category uses generic terminology ("domain models", "repository interfaces", "provider wiring") rather than framework-specific language. The prompt also references `docs/architecture.json` for layer membership, which is project-specific context injected via template variables rather than hardcoded assumptions.

The original Phase 27 plan called for removing Flutter/UI-specific language from the recon prompt. The current prompt has no such language - it was either cleaned up during Phase 18 when the recon agent was introduced, or was never framework-specific to begin with.

---

## Cost Control Through Role Architecture

**What it does:** The combination of model overrides, per-role judge thresholds, and the multi-agent pipeline itself creates layered cost control. Cheap models handle reconnaissance and spec generation. The judge kills stalled agents before they burn credits. The review loop caps retry cycles.

**Example:** A typical issue processed by the current pipeline shows the cost distribution across roles:

```
$ godark trace 42

Step              Duration  Cost     Started              Flags
recon             1m23s     $0.0845  2026-04-01 14:00:12
spec_generator    0m45s     $0.0312  2026-04-01 14:01:35
planner           1m02s     $0.0523  2026-04-01 14:02:20
implement         8m15s     $1.2340  2026-04-01 14:03:22
verify            0m38s     $0.0000  2026-04-01 14:11:37
quality_review    2m10s     $0.1890  2026-04-01 14:12:15
functional_review 1m55s     $0.1650  2026-04-01 14:14:25
```

Recon and spec generation together cost under $0.12 - roughly 7% of the total. The implementer dominates at $1.23. The judge's per-role idle thresholds (recon: 180s, implementer: 300s) prevent any single role from running away. And `MaxRetries` in config caps how many times the expensive implementer loop can repeat.

This is not a single feature but the emergent result of decisions spread across Phases 18 (model overrides, adaptive loop), 24 (resource tracking), 28 (judge thresholds), and 22 (cost analytics). Together they address Phase 27's efficiency goals more comprehensively than the originally planned prompt-level changes would have.
