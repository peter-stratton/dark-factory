# Phase 28: Container Health Judge

> **Goal:** A Go-side health monitor watches container log streams in real-time
> and intervenes when agents stall, thrash, or hit transport failures — cutting
> losses in seconds instead of waiting for the 30-minute timeout. No LLM calls;
> pure heuristic pattern matching on structured log output.

## Milestone

`Phase 28: Container Health Judge`

---

## Issue 640: Real-time log streaming from RunContainer

### Description

Change `RunContainer` to stream `docker logs --follow` via a background
goroutine instead of only reading logs after container exit. Lines are fed to an
optional callback as they arrive. When no callback is provided, behavior is
unchanged — full stdout/stderr is still captured in `RunResult` after exit.

This is the foundation that the judge connects to. No pattern matching in this
issue — just the streaming infrastructure.

### Key constraints

- Modify `sandbox.RunOpts` in `internal/sandbox/container.go` to add an optional
  `LogCallback func(line string)` field
- After `docker start` (line ~194), launch a goroutine that runs
  `docker logs --follow --timestamps <name>` and feeds lines to `LogCallback`
- Use `exec.CommandContext` so the goroutine's `docker logs` process is killed
  when the context is cancelled (timeout or container exit)
- Line-buffer the output: split on `\n`, deliver complete lines only
- The goroutine must not block container lifecycle — if `LogCallback` is slow,
  drop lines rather than backpressuring `docker logs`
- Preserve existing behavior: step 5 (`docker logs` after exit) still runs and
  populates `RunResult.Stdout` / `RunResult.Stderr` as before
- When `LogCallback` is nil, skip launching the streaming goroutine entirely
- `SplitRunner` is not used for streaming — the follow goroutine uses its own
  `exec.CommandContext` call. `SplitRunner` continues to handle the post-exit
  log capture

### Acceptance criteria

- [ ] `LogCallback` field exists on `sandbox.RunOpts`
- [ ] When `LogCallback` is set, lines are delivered during container execution
- [ ] When `LogCallback` is nil, no streaming goroutine is started
- [ ] Full stdout/stderr still captured in `RunResult` after container exit
- [ ] Streaming goroutine exits cleanly on context cancellation

### Test cases

- **Callback receives lines**: Run container with a callback that collects
  lines into a slice. Verify lines are received and non-empty
- **Nil callback no-op**: Run container with nil `LogCallback`. Verify no
  panic and `RunResult` is populated normally
- **Goroutine exits on cancel**: Cancel context during container run. Verify
  the streaming goroutine does not leak (no goroutine leak in test)
- **RunResult still populated**: Run container with `LogCallback` set. Verify
  `RunResult.Stdout` is still populated after exit

---

## Issue 641: Judge package — core types and idle timeout rule

### Description

Create the `internal/agent/judge/` package with the core type system and the
first detection rule: idle timeout. The idle timeout fires when no tool call
audit lines appear in the log stream for a configurable duration, indicating
the agent has stalled.

### Key constraints

- New package `internal/agent/judge/`
- `Judgment` type: constants `Kill`, `RetryContainer`, `Warn`, `Ignore`
- `Intervention` struct: `Rule string`, `Judgment Judgment`,
  `Detail string` (human-readable), `Counts map[string]int`,
  `DetectedAt time.Time`
- `Config` struct: `IdleTimeoutByRole map[string]int` (seconds, keyed by role
  name), `DefaultIdleTimeout int` (fallback, default 300)
- `Rule` interface: `ProcessLine(line string, now time.Time) *Intervention`
  and `Name() string`
- `Judge` struct: holds `[]Rule`, role string, config. Method
  `ProcessLine(line string, now time.Time) *Intervention` iterates rules,
  returns first non-nil intervention
- Idle timeout rule: tracks last time a line containing `"tool":` was seen.
  When `now - lastToolCall > threshold`, returns `Kill` intervention. Must
  distinguish "no output at all" (also idle) from "streaming assistant text
  but no tool calls" (also idle). Both are idle
- The judge receives wall-clock time via the `now` parameter rather than
  calling `time.Now()` internally — this makes tests deterministic
- Role is passed at construction: `NewJudge(role string, cfg Config) *Judge`

### Acceptance criteria

- [ ] `Judgment` type with Kill, RetryContainer, Warn, Ignore constants
- [ ] `Intervention` struct captures rule name, judgment, detail, and timing
- [ ] `Judge` struct processes lines and delegates to rules
- [ ] Idle timeout fires after configured seconds with no tool call lines
- [ ] Idle timeout respects per-role threshold from config

### Test cases

- **Idle timeout fires**: Feed lines with no `"tool":` content, advance time
  past threshold. Verify Kill intervention returned
- **Idle timeout resets on tool call**: Feed non-tool lines, then a tool call
  line before threshold. Verify no intervention. Then go idle again past
  threshold. Verify intervention fires
- **Per-role threshold**: Configure recon at 180s, implementer at 300s. Verify
  each role uses its own threshold
- **Default threshold fallback**: Role not in `IdleTimeoutByRole` map. Verify
  `DefaultIdleTimeout` is used
- **First line starts the clock**: Verify that idle timeout is measured from
  the first `ProcessLine` call, not from judge construction

---

## Issue 644: Judge package — tool thrash and transport failure rules

**Blocked by**: #641

### Description

Add the remaining two detection rules to the judge package: tool thrash
(agent repeatedly searching for unavailable tools) and transport failure
(SDK connection dead on startup).

### Key constraints

- Add to `internal/agent/judge/` package
- Tool thrash rule:
  - Track ToolSearch queries by extracting the query argument from log lines
    containing `ToolSearch` (or the tool search audit pattern)
  - Fire when 3+ calls with the same query pattern occur within a 60-second
    sliding window
  - Judgment: `Kill` (agent is stuck searching for a tool that doesn't exist)
  - Config fields: `ToolThrashThreshold int` (default 3),
    `ToolThrashWindowSecs int` (default 60)
- Transport failure rule:
  - Fire when 10+ `stream closed` / `stream error` lines are seen AND zero
    tool call lines have been observed
  - Judgment: `RetryContainer` (SDK transport is dead, fresh container may fix)
  - Config fields: `TransportFailureThreshold int` (default 10)
- Both rules implement the existing `Rule` interface from the core issue
- Add rules to `defaultRules()` or equivalent constructor used by `NewJudge`

### Acceptance criteria

- [ ] Tool thrash rule detects repeated ToolSearch queries within window
- [ ] Tool thrash rule ignores different queries (no false positive)
- [ ] Transport failure rule fires on stream errors with zero tool calls
- [ ] Transport failure rule does not fire when tool calls are present
- [ ] Both rules produce correct Judgment values

### Test cases

- **Tool thrash fires**: Feed 3 ToolSearch lines with same query within 60s.
  Verify Kill intervention
- **Tool thrash different queries**: Feed 3 ToolSearch lines with different
  queries. Verify no intervention
- **Tool thrash outside window**: Feed 3 same-query ToolSearch lines spread
  over 120s. Verify no intervention
- **Transport failure fires**: Feed 10 stream-closed lines with no tool call
  lines. Verify RetryContainer intervention
- **Transport failure with tool calls**: Feed 10 stream-closed lines but also
  1 tool call line. Verify no intervention (transport recovered)

---

## Issue 642: Judge config block in godark.yaml

### Description

Add a `judge:` configuration block to `godark.yaml` so operators can tune
detection thresholds or disable the judge entirely.

### Key constraints

- Add `Judge` struct to `internal/config/config.go`:
  ```go
  type Judge struct {
      Enabled                   *bool          `yaml:"enabled"`
      IdleTimeoutByRole         map[string]int `yaml:"idle_timeout_by_role"`
      DefaultIdleTimeout        int            `yaml:"default_idle_timeout"`
      ToolThrashThreshold       int            `yaml:"tool_thrash_threshold"`
      ToolThrashWindowSecs      int            `yaml:"tool_thrash_window_secs"`
      TransportFailureThreshold int            `yaml:"transport_failure_threshold"`
      ContainerRetryLimit       int            `yaml:"container_retry_limit"`
  }
  ```
- Add `Judge Judge` field to `Config` struct with yaml tag `judge`
- Defaults when fields are zero/absent:
  - `Enabled`: true (use pointer to distinguish absent from explicit false)
  - `DefaultIdleTimeout`: 300
  - `IdleTimeoutByRole`: empty map (use DefaultIdleTimeout)
  - `ToolThrashThreshold`: 3
  - `ToolThrashWindowSecs`: 60
  - `TransportFailureThreshold`: 10
  - `ContainerRetryLimit`: 2
- Add a `JudgeConfig()` method on `Config` that returns the judge config with
  defaults applied (similar pattern to other config accessors)

### Acceptance criteria

- [ ] `Judge` struct exists in config with all threshold fields
- [ ] Absent `judge:` block yields sensible defaults (enabled, default thresholds)
- [ ] Explicit `enabled: false` disables the judge
- [ ] Per-role idle timeouts parseable from YAML
- [ ] `go test ./internal/config/` passes

### Test cases

- **Defaults applied**: Parse config with no `judge:` block. Verify
  `JudgeConfig()` returns enabled=true, default thresholds
- **Explicit disable**: Parse config with `judge: { enabled: false }`. Verify
  enabled=false
- **Custom thresholds**: Parse config with custom idle timeout and thrash
  threshold. Verify values propagated
- **Per-role idle timeout**: Parse config with
  `idle_timeout_by_role: { recon: 180, implementer: 300 }`. Verify map populated

---

## Issue 643: JudgeIntervention in run data

### Description

Add persistence for judge intervention records so they appear in the dashboard
and `godark analyze` output. When the judge intervenes during an agent run, the
intervention is written to the issue's run data directory.

### Key constraints

- Add `JudgeIntervention` type to `internal/rundata/writer.go`:
  ```go
  type JudgeIntervention struct {
      Rule       string         `json:"rule"`
      Judgment   string         `json:"judgment"`
      Detail     string         `json:"detail"`
      Counts     map[string]int `json:"counts,omitempty"`
      DetectedAt time.Time      `json:"detected_at"`
      Step       string         `json:"step"`
  }
  ```
- Add `WriteJudgeIntervention(issueNum int, intervention JudgeIntervention)`
  method to `Writer`
- Path: `issues/<issueNum>/judge-interventions.json` (JSON array, append if
  file exists — multiple interventions possible per issue across retries)
- Add `JudgeInterventions []JudgeIntervention` field to `IssueDetail` in
  `internal/rundata/reader.go`
- Load interventions in the reader's issue-loading path

### Acceptance criteria

- [ ] `JudgeIntervention` type defined in rundata
- [ ] `WriteJudgeIntervention` writes to correct path
- [ ] Multiple interventions append to the same file
- [ ] Reader loads interventions into `IssueDetail.JudgeInterventions`
- [ ] `go test ./internal/rundata/` passes

### Test cases

- **Write single intervention**: Write one intervention, read file, verify JSON
  structure
- **Append multiple**: Write two interventions for same issue. Verify file
  contains both
- **Reader loads interventions**: Write interventions, load via Reader. Verify
  `IssueDetail.JudgeInterventions` populated
- **Missing file**: Reader loads issue with no judge-interventions.json. Verify
  empty slice, no error

---

## Issue 645: Wire judge into launcher — kill and partial result

**Blocked by**: #640, #641, #642

### Description

Connect the judge to `RunContainer` via the log streaming callback in
`runSandbox()`. When the judge returns a Kill judgment, stop the container and
return the partial result. This is the happy-path wiring — container retry
for transport failures is a separate issue.

### Key constraints

- Modify `runSandbox()` in `internal/agent/launcher.go`:
  - Accept judge config (passed from caller or read from config)
  - If judge is enabled, create `judge.NewJudge(role, judgeConfig)`
  - Set `LogCallback` on `sandbox.RunOpts` to a closure that calls
    `judge.ProcessLine(line, time.Now())`
  - When `ProcessLine` returns an intervention: store it on the result,
    and for Kill judgments, cancel the context to stop the container
  - Use a `context.WithCancel` wrapping the timeout context so the judge
    can trigger early cancellation independently of the timeout
- Add `JudgeKilled bool` field to `agent.Result` — distinct from `TimedOut`
- Add `JudgeIntervention *judge.Intervention` field to `agent.Result`
- When judge-killed: `Result.JudgeKilled = true`, `Result.TimedOut = false`,
  partial stdout/stderr captured as usual
- The `Run()` function signature does not change — judge config flows through
  `RunOpts` or a new `JudgeConfig` field on `agent.RunOpts`

### Acceptance criteria

- [ ] Judge created from config in `runSandbox` when enabled
- [ ] LogCallback wired to judge's ProcessLine
- [ ] Kill judgment cancels context and stops container
- [ ] `Result.JudgeKilled` set to true on kill
- [ ] `Result.JudgeIntervention` populated with intervention details

### Test cases

- **Kill stops container**: Stub RunContainer to stream lines that trigger idle
  timeout. Verify result has `JudgeKilled: true` and partial output
- **Judge disabled**: Config with `enabled: false`. Verify no LogCallback set,
  container runs normally
- **No intervention**: Stream normal lines with tool calls. Verify result has
  `JudgeKilled: false` and no intervention
- **Intervention record populated**: Trigger a kill. Verify
  `Result.JudgeIntervention` contains rule name, detail, and timing

---

## Issue 646: Wire judge into agent loop — intervention events and run data

**Blocked by**: #643, #645

### Description

After each agent call returns in the orchestration loop, check whether the
judge intervened and handle it as a distinct outcome. Write interventions to
run data and surface them as a distinguishable event (not just "failed").

### Key constraints

- Modify `internal/agent/loop.go`:
  - After each agent `Run()` / `Retry()` / `VerifyFix()` call, check
    `result.JudgeKilled` and `result.JudgeIntervention`
  - When judge killed: write the intervention to run data via
    `hook.WriteJudgeIntervention(issueNum, intervention)`
  - Convert `judge.Intervention` to `rundata.JudgeIntervention` (add the
    `Step` field indicating which agent step was killed: "implement",
    "review", "verify_fix", etc.)
  - Judge kill is a terminal outcome for the current attempt — do not retry
    the step (the agent was unproductive). Log a clear message explaining why
  - Judge RetryContainer is handled by the launcher (issue #8), so the loop
    only sees the final result after container retries are exhausted
  - If the judge killed the step and max retries haven't been reached, the
    existing retry logic can still re-run the step as a new attempt (fresh
    container). The judge kill prevents retrying the _same_ container, not
    the whole step

### Acceptance criteria

- [ ] Judge-killed results detected in loop after agent calls
- [ ] Intervention written to run data with correct step name
- [ ] Judge kill logged with rule name and detail
- [ ] Judge kill does not trigger same-container retry
- [ ] Fresh-attempt retry still permitted after judge kill

### Test cases

- **Intervention written**: Mock agent returning judge-killed result. Verify
  `WriteJudgeIntervention` called with correct step and rule
- **Kill logged**: Trigger judge kill in loop. Verify log message contains
  rule name
- **No retry on kill**: Agent judge-killed on first attempt. Verify step is
  not retried in the same attempt cycle
- **Fresh attempt after kill**: Agent judge-killed, but max retries not
  reached. Verify a new attempt is started (new container)

---

## Issue 647: Container retry for transport failures

**Blocked by**: #645

### Description

When the judge detects a transport failure (RetryContainer judgment), retry the
container — not the whole agent step — up to a configurable limit. This is
distinct from the existing agent retry logic which re-runs the full step with
reviewer feedback.

### Key constraints

- Modify `runSandbox()` in `internal/agent/launcher.go`:
  - When judge returns `RetryContainer` intervention: stop the current
    container, log the retry, and start a new container with the same opts
  - Retry up to `ContainerRetryLimit` times (from judge config, default 2)
  - Each retry is a fresh container (new name, new `docker create/start`)
  - The prompt, env, and image are unchanged between retries
  - Refresh the GitHub App token before each retry (call `refreshGHToken`)
  - If all container retries are exhausted, return the last result with
    `JudgeKilled: true` and the transport failure intervention
  - Log each container retry attempt with attempt number and reason

### Acceptance criteria

- [ ] Transport failure triggers container retry (not step retry)
- [ ] Retry limit respected (default 2 retries)
- [ ] Each retry is a fresh container with same opts
- [ ] GitHub App token refreshed before each retry
- [ ] Exhausted retries return last result with JudgeKilled

### Test cases

- **Retry succeeds**: First container hits transport failure, second succeeds.
  Verify final result is from the successful container
- **Retry limit exhausted**: All containers hit transport failure. Verify
  result has `JudgeKilled: true` after N retries
- **Kill does not retry**: Judge returns Kill (not RetryContainer). Verify no
  container retry attempted
- **Token refreshed**: Verify `refreshGHToken` called before each retry
  attempt

---

## Issue 648: Surface judge interventions in TUI and notifications

**Blocked by**: #646

### Description

Display judge interventions in the TUI as they happen and send notifications
when the judge kills an agent step. This gives operators real-time visibility
into judge decisions.

### Key constraints

- Add a `JudgeIntervention` message type to `internal/tui/messages.go`
- Update the TUI model in `internal/tui/model.go` to handle the new message:
  display the intervention reason in the issue's status column (e.g.,
  "Killed: idle 300s", "Killed: tool thrash", "Retry: transport failure")
- Update `internal/tui/table.go` to render judge-killed status distinctly
  from normal failure (different color or marker)
- Add `judge_intervention` to notification events in `internal/notify/notify.go`
  — piggyback on the existing abort/failure notification path
- The progress reporter interface (`internal/progress/reporter.go`) may need
  a new method or the existing `StepFailed` method can carry the intervention
  detail — prefer reusing existing methods with additional context

### Acceptance criteria

- [ ] TUI displays judge intervention reason for affected issues
- [ ] Judge-killed status visually distinct from normal failure in TUI
- [ ] Notification sent on judge kill event
- [ ] TUI handles interventions without crashing on nil fields

### Test cases

- **TUI renders intervention**: Send JudgeIntervention message to TUI model.
  Verify the issue row shows the intervention reason
- **Notification dispatched**: Trigger judge kill event. Verify notification
  provider receives the event with intervention details
- **Nil intervention safe**: Send step result with nil JudgeIntervention to
  TUI. Verify no panic

---

## Issue 649: Dashboard rendering of judge interventions

**Blocked by**: #643

### Description

Render judge intervention records in the web dashboard so operators can see
which issues were affected by judge decisions and why. The data is already
loaded into `IssueDetail.JudgeInterventions` by the reader — this issue adds
the template rendering.

### Key constraints

- Modify dashboard templates in `internal/dashboard/` to display judge
  interventions on the issue detail view
- Show for each intervention: rule name, judgment, detail message, detected
  timestamp, and which step was affected
- On the run overview page, add a visual indicator (icon or badge) on issues
  that had judge interventions
- On the issue detail page, add a "Judge Interventions" section (only rendered
  when interventions exist) listing each intervention
- Use existing dashboard CSS classes/patterns — no new CSS framework or
  significant styling changes
- The handler already loads `IssueDetail` via the reader — no handler changes
  needed, just template changes

### Acceptance criteria

- [ ] Issue detail page shows judge interventions when present
- [ ] Run overview shows visual indicator for judge-affected issues
- [ ] No rendering when `JudgeInterventions` is empty
- [ ] Dashboard templates render without error

### Test cases

- **Intervention displayed**: Load issue detail with one judge intervention.
  Verify the intervention section renders with rule name and detail
- **Multiple interventions**: Load issue with two interventions. Verify both
  are rendered in order
- **No interventions**: Load issue with empty `JudgeInterventions`. Verify no
  intervention section rendered
- **Overview indicator**: Load run overview with one judge-affected issue.
  Verify the indicator appears on that issue's row
