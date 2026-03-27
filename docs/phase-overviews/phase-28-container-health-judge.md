# Phase 28: Container Health Judge

When an agent stalls mid-run, the old behavior was to wait out the full 30-minute timeout -- burning API credits and clock time on a session that stopped making progress minutes ago. Phase 28 adds a Go-side health monitor that watches container log streams in real-time and intervenes in seconds. Four heuristic rules detect idle agents, tool search loops, transport failures, and no-progress streaming. No LLM calls involved -- pure pattern matching on structured log output, with per-role thresholds configurable in `godark.yaml`.

---

## Real-Time Log Streaming

**What it does:** `RunContainer` now streams `docker logs --follow` via a background goroutine instead of only reading logs after container exit. Lines are fed to a callback as they arrive, enabling the judge to react while the agent is still running.

**Example:** The `RunOpts` struct in `internal/sandbox/container.go` gained a `LogCallback` field:

```go
type RunOpts struct {
    Image             string
    Cmd               []string
    Env               map[string]string
    Timeout           time.Duration
    LogCallback       func(line string)
    // ...
}
```

When `LogCallback` is set, `startLogStreamer` spawns two goroutines -- a producer tailing `docker logs --follow --timestamps` and a consumer delivering lines to the callback. The channel buffer holds 256 lines; if the consumer falls behind, lines are dropped rather than backpressuring the container. A 1 MB max line length handles realistic Docker output. When `LogCallback` is nil, no streaming goroutine starts and behavior is unchanged from before.

```
Container starts
  |
  +-- Producer goroutine: docker logs --follow --timestamps <name>
  |     \-- writes to buffered channel (cap 256, drops on full)
  |
  +-- Consumer goroutine: reads channel, calls LogCallback(line)
  |
  +-- Container exits
  |     \-- context cancelled, both goroutines exit
  |
  +-- Post-exit: docker logs <name> populates RunResult.Stdout/Stderr (unchanged)
```

---

## Judge Package and Detection Rules

**What it does:** The `internal/agent/judge/` package implements four detection rules, each watching the log stream for a specific failure pattern. Every log line passes through `Judge.ProcessLine()`, which returns an `*Intervention` when a rule fires.

**Example:** A recon agent working on issue #450 goes silent after reading a few files. The judge's idle timeout fires after 180 seconds (the configured threshold for the `recon` role):

```go
j := judge.NewJudge("recon", judge.Config{
    IdleTimeoutByRole:  map[string]int{"recon": 180, "implementer": 300},
    DefaultIdleTimeout: 300,
})

// Lines arrive during normal operation...
j.ProcessLine(`{"tool": "Read", "input_summary": "main.go"}`, now)

// 3 minutes of silence, heartbeat ticks empty lines every 30s...
intervention := j.ProcessLine("", now.Add(181*time.Second))
// intervention.Rule     = "idle_timeout"
// intervention.Judgment = judge.Kill
// intervention.Detail   = `no output for 3m1s (threshold 3m0s, role "recon")`
```

The four rules and their judgments:

| Rule | Fires when | Judgment | Default threshold |
|------|-----------|----------|-------------------|
| `idle_timeout` | No log output for N seconds | `Kill` | 300s (per-role override) |
| `no_progress` | Log output streaming but no tool calls for N seconds | `Kill` | 600s (per-role override) |
| `tool_thrash` | Same ToolSearch query repeated N times within window | `Kill` | 3 queries in 60s |
| `transport_failure` | N stream-closed/error lines with zero tool calls | `RetryContainer` | 10 errors |

The `no_progress` rule distinguishes "agent is thinking" (text streaming, no tools) from "agent is working" (tool calls happening). It detects tool calls from both SDK audit format (`{"tool": "Read", ...}`) and CLI stream-json format (`{"type":"tool_use","name":"Read", ...}`). The `tool_thrash` rule extracts the query string from ToolSearch JSON and tracks per-query frequency in a sliding window. The `transport_failure` rule is the only one that returns `RetryContainer` instead of `Kill` -- the agent itself isn't broken, just the connection.

---

## Judge Configuration

**What it does:** A `judge:` block in `godark.yaml` controls all thresholds. When absent, the judge runs with sensible defaults. Setting `enabled: false` disables it entirely.

**Example:** A project where the implementer needs longer to think but the recon agent should fail fast:

```yaml
judge:
  enabled: true
  default_idle_timeout: 300
  idle_timeout_by_role:
    recon: 180
    implementer: 300
    reviewer: 600
  default_no_progress_timeout: 600
  no_progress_timeout_by_role:
    implementer: 1200
  tool_thrash_threshold: 3
  tool_thrash_window_secs: 60
  transport_failure_threshold: 10
  container_retry_limit: 2
```

The `container_retry_limit` field controls how many times a container is restarted on `RetryContainer` judgments. A value of 2 means 3 total attempts (1 initial + 2 retries). The config struct in `internal/config/config.go`:

```go
type Judge struct {
    Enabled                   *bool          `yaml:"enabled"`
    IdleTimeoutByRole         map[string]int `yaml:"idle_timeout_by_role"`
    DefaultIdleTimeout        int            `yaml:"default_idle_timeout"`
    NoProgressTimeoutByRole   map[string]int `yaml:"no_progress_timeout_by_role"`
    DefaultNoProgressTimeout  int            `yaml:"default_no_progress_timeout"`
    ToolThrashThreshold       int            `yaml:"tool_thrash_threshold"`
    ToolThrashWindowSecs      int            `yaml:"tool_thrash_window_secs"`
    TransportFailureThreshold int            `yaml:"transport_failure_threshold"`
    ContainerRetryLimit       int            `yaml:"container_retry_limit"`
}
```

---

## Container Retry for Transport Failures

**What it does:** When the judge detects a transport failure (dead SDK connection), the container is automatically restarted instead of failing the step. This is distinct from the agent retry loop -- container retry restarts the same step fresh, while agent retry runs the implementer again with reviewer feedback.

**Example:** An implementer hits a flaky network connection. The log stream shows repeated `stream closed` lines with no tool calls. After the 10th error line, the judge fires `transport_failure` with `RetryContainer`. The launcher in `internal/agent/launcher.go` catches this:

```go
for attempt := 0; attempt <= containerRetryLimit; attempt++ {
    res, err := runSandboxOnce(ctx, opts, sandboxOpts, startedAt, logger)

    if res.JudgeIntervention != nil &&
       res.JudgeIntervention.Judgment == judge.RetryContainer &&
       attempt < containerRetryLimit {
        continue  // Restart container, same step
    }
    break
}
```

If all retries are exhausted, the result propagates up as a failure. The implementer retry loop may then kick in with a fresh session, but the transport issue is usually transient and resolves on the first container retry.

---

## Intervention Flow: Detection to Display

**What it does:** When the judge intervenes, the event flows through run data, the dashboard, the TUI, and notifications -- giving visibility into what happened and why.

**Example:** An implementer on issue #512 stalls after 5 minutes. The judge kills the container. Here's the event flow:

1. **Detection**: `judge.ProcessLine()` returns an `Intervention` with rule `idle_timeout`
2. **Container kill**: The callback stores the intervention in an atomic pointer and calls `cancel()` to stop the container immediately
3. **Result propagation**: `Result.JudgeKilled = true` and `Result.JudgeIntervention` are set
4. **Run data**: Written to `issues/512/judge-interventions.json` as a JSON array:

```json
[{
  "rule": "idle_timeout",
  "judgment": "kill",
  "detail": "no output for 5m1s (threshold 5m0s, role \"implementer\")",
  "counts": {"idle_seconds": 301},
  "detected_at": "2026-03-15T14:22:33Z",
  "step": "implement"
}]
```

5. **TUI**: The text reporter prints:
```
#512 -- judge kill: idle_timeout (no output for 5m0s...) [implement]
```

6. **Dashboard**: The issue detail page shows the intervention as a card with a red "danger" badge for `kill` judgments (yellow "warning" for `retry_container`), displaying the rule name, detail, and timestamp.

---

## Heartbeat Tick

**What it does:** A background goroutine ticks the judge with empty lines every 30 seconds, so idle timeout detection works even when the container produces zero output.

**Example:** Without the heartbeat, an agent that goes completely silent would never trigger the idle rule -- there would be no lines to process. The heartbeat in `buildJudgeCallback` (in `internal/agent/launcher.go`) solves this:

```go
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            mu.Lock()
            if iv := j.ProcessLine("", time.Now()); iv != nil {
                // Store intervention, cancel context
            }
            mu.Unlock()
        case <-ctx.Done():
            return
        }
    }
}()
```

The mutex serializes access between the heartbeat goroutine and the log callback goroutine. The 30-second interval means idle detection fires within 30 seconds of the actual threshold being crossed.

---

## Benign Kill Handling

**What it does:** If the judge kills a container but the agent already produced useful output (`ResultText` is non-empty), the kill is treated as benign and the result is used normally. This handles the common case where an agent finishes its work, outputs the result, then idles while the container winds down.

**Example:** An implementer finishes writing code and outputs its result text, then goes idle for 5 minutes while the container is still running. The judge kills it for idle timeout, but `handleJudgeIntervention` in `internal/agent/loop.go` checks:

```go
if result.ResultText != "" {
    return false  // Benign kill -- proceed with result
}
return true  // Terminal kill -- no usable result
```

The implementation continues to the review phase with the agent's output intact. The intervention is still recorded in run data for visibility, but it doesn't trigger a retry or failure.
