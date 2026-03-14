# Phase 20: Terminal UI

> **Goal:** `godark run` renders a live, rich terminal interface using Bubble Tea
> when running interactively. The TUI shows run metadata, per-issue progress with
> live status updates, and a summary bar — replacing the current structured log
> output in interactive mode while preserving JSON output for piped/non-interactive
> contexts.

## Milestone

`Phase 20`

---

## Issue 439: Add Bubble Tea dependencies and update architecture.json

### Description

Add the Charm ecosystem packages to `go.mod` and register `internal/tui/` as a
new path in the presentation layer of `docs/architecture.json`. This is pure
setup — no Go code beyond the dependency import.

### Key constraints

- Add to `go.mod`:
  - `github.com/charmbracelet/bubbletea`
  - `github.com/charmbracelet/lipgloss`
  - `github.com/charmbracelet/bubbles`
- Update `docs/architecture.json` presentation layer (line 27): add
  `"internal/tui/"` to the `paths` array alongside `"internal/dashboard/"`
- Same dependency rules as `internal/dashboard/`: may depend on `domain` and
  `foundation`; must not depend on `cmd`, `orchestration`, `service`,
  `infrastructure`, or `content`
- Run `go mod tidy` to resolve transitive dependencies

### Acceptance criteria

- [ ] `go.mod` includes `bubbletea`, `lipgloss`, and `bubbles`
- [ ] `go build ./...` succeeds
- [ ] `docs/architecture.json` presentation layer paths include
  `"internal/tui/"`
- [ ] `godark vet architecture` passes (no cycle or layer violations)

### Test cases

- **Build succeeds**: `go build ./...` exits 0 after dependency addition
- **Vet passes**: `godark vet architecture` reports no violations with the
  updated layer definition

---

## Issue 440: Progress reporter interface and plain-text implementation

### Description

Define a `ProgressReporter` interface in the foundation layer that the
orchestrator calls instead of `fmt.Printf` for user-facing progress output.
Provide a `TextReporter` implementation that reproduces the current terminal
output exactly — same format strings, same line structure. This ensures zero
behavior change for existing users before the TUI is wired in.

The interface uses only primitive types (int, string, bool) so it can live in
foundation without importing domain types.

New package: `internal/progress/` in the foundation layer.

### Key constraints

- New file `internal/progress/reporter.go`:
  ```go
  // ProgressReporter receives progress events from the orchestrator.
  type ProgressReporter interface {
      // RunStarted signals the beginning of a run with metadata.
      RunStarted(repo, milestone, timestamp, baseBranch, mergeFeature, mergeRollup string, issueCount int)
      // IssueStarted signals that processing has begun for an issue.
      IssueStarted(issueNumber int, title string)
      // IssueStageChanged signals a stage transition for an in-progress issue.
      IssueStageChanged(issueNumber int, stage string)
      // IssueCompleted signals the final outcome for an issue.
      IssueCompleted(issueNumber int, title, status string, prNumber, retries int, errMsg string)
      // WaveStarted signals a new dependency re-resolution wave.
      WaveStarted(wave, count int)
      // AllBlocked signals that all issues are blocked.
      AllBlocked(total, blocked int)
      // RollupCreated signals rollup PR creation and optional merge.
      RollupCreated(prNumber int, prURL string, merged bool)
      // RunFinished signals the final summary.
      RunFinished(implemented, readyToMerge, needsHumanReview, failed, blocked int)
      // PunchlistText outputs a punchlist fragment as it becomes available.
      PunchlistText(text string)
  }
  ```
- New file `internal/progress/text.go`:
  - `TextReporter` struct implementing `ProgressReporter`
  - Each method writes the same format string currently used in
    `orchestrator.go` and `implement.go` (e.g., `IssueCompleted` for
    `StatusImplemented` writes
    `"  #%d %s — implemented (PR #%d, %d retries)\n"`)
  - Writes to an `io.Writer` (default `os.Stdout`) for testability
- Update `docs/architecture.json` foundation layer (line 47): add
  `"internal/progress/"` to the `paths` array
- The `TextReporter` does NOT handle dry-run output — dry-run uses its own
  `printDryRun()` function which stays as-is

### Acceptance criteria

- [ ] `ProgressReporter` interface defined in `internal/progress/reporter.go`
  with all methods listed above
- [ ] `TextReporter` implements `ProgressReporter` and writes to a configurable
  `io.Writer`
- [ ] `TextReporter` output matches the current `fmt.Printf` format strings
  exactly
- [ ] Package is in the foundation layer with no imports from other project
  packages
- [ ] `godark vet architecture` passes

### Test cases

- **IssueCompleted implemented**: `IssueCompleted(42, "add cost tracking",
  "implemented", 87, 0, "")` writes
  `"  #42 add cost tracking — implemented (PR #87, 0 retries)\n"`
- **IssueCompleted failed**: `IssueCompleted(42, "add cost tracking", "failed",
  0, 0, "timeout")` writes `"  #42 add cost tracking — failed: timeout\n"`
- **RunFinished**: `RunFinished(3, 1, 1, 1, 2)` writes `"Results: 3
  implemented, 1 ready-to-merge, 1 needs-human-review, 1 failed, 2 skipped
  (blocked)\n"`
- **WaveStarted**: `WaveStarted(2, 3)` writes
  `"\n--- Wave 2: 3 newly unblocked issues ---\n"`
- **AllBlocked**: `AllBlocked(5, 5)` writes `"All issues are blocked — nothing
  to process.\n"` followed by the summary line
- **Writer injection**: `TextReporter` constructed with a `bytes.Buffer` writes
  output to that buffer, not stdout

---

## Issue 441: Replace orchestrator and cmd fmt output with progress reporter

**Blocked by**: #440

### Description

Add a `ProgressReporter` parameter to the orchestrator's `Run()` and
`processIssues()` functions and to `implement.go`'s processing loop. Replace all
user-facing `fmt.Printf` calls with the corresponding reporter method calls.
Pass a `TextReporter` from the cmd layer so behavior is identical to today.

This issue is purely mechanical — every `fmt.Printf` that produces progress
output becomes a reporter method call. Logger calls are untouched. Dry-run
output (`printDryRun`) stays as-is.

### Key constraints

- Modify `internal/orchestrator/orchestrator.go`:
  - Add `reporter progress.ProgressReporter` parameter to `Run()` (line 38)
    and `processIssues()` (line 209)
  - Replace `fmt.Printf("  #%d %s — implemented ...")` (line 383) →
    `reporter.IssueCompleted(issue.Number, issue.Title, "implemented",
    outcome.PRNumber, outcome.Retries, "")`
  - Replace `fmt.Printf("  #%d %s — ready-to-merge ...")` (line 392) →
    `reporter.IssueCompleted(...)` with status `"ready-to-merge"`
  - Replace `fmt.Printf("  #%d %s — needs human review ...")` (line 395) →
    `reporter.IssueCompleted(...)` with status `"needs-human-review"`
  - Replace `fmt.Printf("  #%d %s — failed ...")` (line 402) →
    `reporter.IssueCompleted(...)` with status `"failed"`
  - Replace `fmt.Printf("\n--- Wave %d ...")` (line 331) →
    `reporter.WaveStarted(wave, len(batch))`
  - Replace `fmt.Println("All issues are blocked ...")` (lines 320-321) →
    `reporter.AllBlocked(...)`
  - Replace `fmt.Printf("Results: ...")` (lines 475-477) →
    `reporter.RunFinished(...)`
  - Replace `fmt.Printf("Rollup PR #%d created ...")` (line 631) and
    merge/open lines (lines 657, 660) → `reporter.RollupCreated(...)`
  - Replace `fmt.Print(text)` for punchlist (line 443) →
    `reporter.PunchlistText(text)`
  - `printDryRun()` remains unchanged (fmt.Printf is fine for one-shot output)
  - `printSummary()` helper can be removed or inlined since reporter handles it
- Modify `internal/cmd/run.go`:
  - Create `progress.NewTextReporter(os.Stdout)` and pass to
    `orchestrator.Run()`
- Modify `internal/cmd/implement.go`:
  - Replace the per-issue `fmt.Printf` calls (lines 189, 215, 221, 224, 231)
    with reporter method calls
  - Replace `fmt.Printf("Results: ...")` (line 278) with
    `reporter.RunFinished(...)`
  - Replace punchlist `fmt.Println()` and text output with
    `reporter.PunchlistText(text)`
  - Create `progress.NewTextReporter(os.Stdout)` at the top of the command
- Total `fmt.Printf` calls replaced: ~23 in orchestrator.go + ~11 in
  implement.go = ~34
- All `logger.Info` / `logger.Warn` calls are NOT replaced — they continue
  writing to `debug.log` and (in non-TUI mode) stdout

### Acceptance criteria

- [ ] `orchestrator.Run()` accepts a `ProgressReporter` parameter
- [ ] All user-facing `fmt.Printf` calls in `orchestrator.go` replaced with
  reporter method calls (dry-run output excluded)
- [ ] All user-facing `fmt.Printf` calls in `implement.go` replaced with
  reporter method calls
- [ ] `run.go` creates and passes a `TextReporter`
- [ ] `implement.go` creates and passes a `TextReporter`
- [ ] Existing terminal output is byte-for-byte identical when running with
  `TextReporter`

### Test cases

- **Orchestrator calls reporter**: Mock reporter receives `IssueCompleted` call
  with correct parameters after `processIssueFn` returns an outcome
- **Wave triggers reporter**: Mock reporter receives `WaveStarted(2, N)` on
  second wave
- **RunFinished called**: Mock reporter receives final summary counts matching
  the stats struct
- **Implement calls reporter**: Mock reporter receives `IssueCompleted` for each
  processed issue in `implement.go`
- **Dry-run unchanged**: Dry-run mode does not call any reporter methods (uses
  `printDryRun` directly)
- **Logger calls preserved**: `logger.Info("issue outcome", ...)` still called
  alongside reporter methods

---

## Issue 442: TUI model, header, and summary bar

**Blocked by**: #439

### Description

Create the core Bubble Tea `Model` struct in `internal/tui/` with header and
summary bar rendering. The header displays run metadata mirroring the dashboard
run detail view (`run-detail.html` line 75). The summary bar shows aggregate
issue counts and running cost.

This issue creates the static chrome of the TUI. The issue table (dynamic
content) is a separate issue.

### Key constraints

- New file `internal/tui/model.go`:
  - `Model` struct with fields for run metadata (repo, milestone, timestamp,
    baseBranch, mergeFeature, mergeRollup), issue state map, and aggregate
    counts
  - `Init() tea.Cmd` — returns nil (no initial command)
  - `Update(tea.Msg) (tea.Model, tea.Cmd)` — handles window size messages and
    custom progress messages
  - `View() string` — composes header + placeholder for table + summary bar
- New file `internal/tui/header.go`:
  - `renderHeader()` function using Lip Gloss styles
  - Format: `godark` logo text left-aligned, `repo` right-aligned on first line
  - Second line: `milestone · timestamp · base: baseBranch · auto-merge:
    feature=X rollup=Y`
  - Omit base branch segment when empty (same conditional as dashboard line 75)
  - Omit auto-merge segment when nil
  - Use Lip Gloss `NewStyle()` for colors and borders — muted colors for labels,
    bright for values
- New file `internal/tui/summary.go`:
  - `renderSummary()` function using Lip Gloss styles
  - Format: `N merged · N in review · N queued · N failed · $X.XX total cost`
  - Positioned at bottom of terminal (use terminal height from window size msg)
  - Counts and cost updated via model fields
- New file `internal/tui/styles.go`:
  - Centralized Lip Gloss style definitions (colors, borders, spacing)
  - Follow the dashboard's visual hierarchy: muted for chrome, green for
    success, yellow for in-review, red for failures
- `internal/tui/` must not import from orchestration, service, infrastructure,
  or content layers

### Acceptance criteria

- [ ] `Model` struct implements `tea.Model` interface (Init, Update, View)
- [ ] Header renders repo, milestone, timestamp on the chrome line
- [ ] Header conditionally includes base branch and auto-merge settings
- [ ] Summary bar renders aggregate counts
- [ ] Styles use Lip Gloss (no raw ANSI escape codes)

### Test cases

- **Header full metadata**: `renderHeader()` with all fields set produces output
  containing repo, milestone, timestamp, base branch, and auto-merge settings
- **Header minimal**: `renderHeader()` with empty baseBranch and nil autoMerge
  omits those segments
- **Summary zero state**: `renderSummary()` with all zeros renders
  `"0 merged · 0 in review · 0 queued · 0 failed · $0.00 total cost"`
- **Summary with counts**: `renderSummary()` with non-zero values renders each
  count correctly
- **Model implements tea.Model**: `Model` satisfies the `tea.Model` interface
  (compile-time check)

---

## Issue 443: TUI issue table component

**Blocked by**: #442

### Description

Add the issue table to the TUI model. Each issue is rendered as a row with a
status marker, issue number, title, and current stage. Rows update live as the
orchestrator reports progress via Bubble Tea messages.

### Key constraints

- New file `internal/tui/table.go`:
  - `issueRow` struct: `number int`, `title string`, `status string`,
    `stage string`, `prNumber int`, `retries int`, `errMsg string`
  - Status markers using Lip Gloss styled characters:
    - `○` — queued (dim/muted)
    - Bubbles spinner — in progress (animated)
    - `■` — completed/merged (green)
    - `●` — in review / ready-to-merge (yellow)
    - `✕` — failed (red)
  - Stage labels: `recon`, `implement`, `verify`, `review`, `merged`, `failed`
  - `renderTable()` function that iterates issue rows and composes styled output
  - Table respects terminal width — truncates long titles with ellipsis
- New file `internal/tui/messages.go`:
  - Custom Bubble Tea message types for progress events:
    - `IssueStartedMsg{Number int, Title string}`
    - `IssueStageChangedMsg{Number int, Stage string}`
    - `IssueCompletedMsg{Number int, Title, Status string, PRNumber, Retries int, ErrMsg string}`
    - `WaveStartedMsg{Wave, Count int}`
    - `RunFinishedMsg{Implemented, ReadyToMerge, NeedsHumanReview, Failed, Blocked int}`
  - These messages are sent to the Bubble Tea program by the TUI reporter
    (next issue)
- Modify `internal/tui/model.go`:
  - Add `issues []issueRow` and `issueIndex map[int]int` to `Model`
  - Handle new message types in `Update()`: update issue state, trigger
    re-render
  - Add spinner model from `bubbles/spinner` for in-progress animation
  - Compose table between header and summary in `View()`

### Acceptance criteria

- [ ] Issue rows render with correct status markers for each state
- [ ] In-progress issues show an animated spinner
- [ ] `IssueStartedMsg` adds a new row to the table
- [ ] `IssueStageChangedMsg` updates the stage label on the correct row
- [ ] `IssueCompletedMsg` updates the status marker and stage for the correct
  row
- [ ] Long titles are truncated to fit terminal width

### Test cases

- **Queued row**: Row with status `""` renders `○` marker
- **In-progress row**: Row with stage `"implement"` renders spinner marker
  and `implement` stage label
- **Completed row**: Row with status `"implemented"` renders `■` marker in green
- **Failed row**: Row with status `"failed"` renders `✕` marker in red with
  error message
- **Stage update**: Sending `IssueStageChangedMsg{Number: 42, Stage: "verify"}`
  to `Update()` changes issue 42's stage to `"verify"`
- **Title truncation**: A 120-character title in a 80-column terminal is
  truncated with `…`

---

## Issue 444: TUI progress reporter implementation

**Blocked by**: #440, #443

### Description

Create a `TUIReporter` that implements the `ProgressReporter` interface by
sending Bubble Tea messages to the running program. This is the adapter between
the orchestrator's reporter calls and the TUI's message-driven update loop.

### Key constraints

- New file `internal/tui/reporter.go`:
  - `TUIReporter` struct holding a `*tea.Program` reference
  - `NewTUIReporter(p *tea.Program) *TUIReporter`
  - Each `ProgressReporter` method sends the corresponding message type via
    `p.Send()`:
    - `IssueStarted()` → `p.Send(IssueStartedMsg{...})`
    - `IssueStageChanged()` → `p.Send(IssueStageChangedMsg{...})`
    - `IssueCompleted()` → `p.Send(IssueCompletedMsg{...})`
    - `WaveStarted()` → `p.Send(WaveStartedMsg{...})`
    - `RunFinished()` → `p.Send(RunFinishedMsg{...})`
    - `RunStarted()` → `p.Send(RunStartedMsg{...})` (sets header metadata)
    - `AllBlocked()` → `p.Send(RunFinishedMsg{...})` with all counts as blocked
    - `RollupCreated()` → `p.Send(RollupCreatedMsg{...})`
    - `PunchlistText()` — no-op in TUI mode (punchlist is written to file;
      TUI doesn't display it)
  - `TUIReporter` lives in `internal/tui/` (presentation layer), NOT in
    `internal/progress/` (foundation layer) — it imports `tea` which is a
    presentation concern
- The `TUIReporter` is created in the cmd layer after the `tea.Program` is
  initialized, and passed to the orchestrator as a `ProgressReporter`

### Acceptance criteria

- [ ] `TUIReporter` implements `progress.ProgressReporter`
- [ ] Each reporter method sends the correct Bubble Tea message type
- [ ] `PunchlistText` is a no-op (does not crash or block)
- [ ] `TUIReporter` lives in `internal/tui/` package
- [ ] No imports from orchestration, service, or infrastructure layers

### Test cases

- **IssueStarted sends message**: Calling `IssueStarted(42, "add endpoint")`
  calls `p.Send()` with `IssueStartedMsg{Number: 42, Title: "add endpoint"}`
- **IssueCompleted sends message**: Calling
  `IssueCompleted(42, "add endpoint", "implemented", 87, 0, "")` calls
  `p.Send()` with matching `IssueCompletedMsg`
- **RunFinished sends message**: Calling `RunFinished(3, 1, 0, 1, 2)` calls
  `p.Send()` with matching `RunFinishedMsg`
- **PunchlistText no-op**: Calling `PunchlistText("some text")` does not panic
  or send any message

---

## Issue 445: Hybrid output mode and --no-tui flag

**Blocked by**: #441, #444

### Description

Add terminal detection logic to `cmd/run.go` and `cmd/implement.go` that selects
between `TUIReporter` and `TextReporter` based on whether stdout is an
interactive terminal. Add a `--no-tui` flag to force plain-text output even in
interactive terminals.

When TUI mode is active, the Bubble Tea program runs on a separate goroutine
while the orchestrator runs on the main goroutine (or vice versa — Bubble Tea's
`Run()` blocks, so the orchestrator must run in a goroutine that sends messages
back). The TUI program's lifecycle must handle graceful shutdown on SIGINT.

### Key constraints

- Modify `internal/cmd/run.go`:
  - Add `--no-tui` flag (bool, default false)
  - Decision logic:
    ```
    if !noTUI && term.IsTerminal(int(os.Stdout.Fd())) {
        // TUI mode
    } else {
        // Text mode (current behavior)
    }
    ```
  - TUI mode startup sequence:
    1. Create `tui.Model` with initial metadata
    2. Create `tea.NewProgram(model, tea.WithAltScreen())`
    3. Create `tui.NewTUIReporter(program)`
    4. Launch orchestrator in a goroutine, passing the TUI reporter
    5. Call `program.Run()` (blocks until quit)
    6. Return orchestrator error after program exits
  - Text mode: create `progress.NewTextReporter(os.Stdout)` and pass to
    orchestrator (current behavior, unchanged)
  - Suppress slog text handler stdout output when TUI is active — the TUI
    owns the screen; logger still writes JSON to `debug.log`
- Modify `internal/cmd/implement.go`:
  - Same `--no-tui` flag and detection logic
  - Same TUI/text reporter selection pattern
- Modify `internal/logging/logger.go`:
  - Add `NewLoggerFileOnly(dir string) (*slog.Logger, error)` variant that
    writes JSON to `debug.log` but does NOT write to stdout (for TUI mode where
    the TUI owns the screen)
  - Alternative: add a `quiet bool` parameter to `NewLogger` — use whichever
    is cleaner
- When the orchestrator finishes (goroutine returns), send a quit message to
  the Bubble Tea program so it exits cleanly

### Acceptance criteria

- [ ] `--no-tui` flag exists on both `run` and `implement` commands
- [ ] Interactive terminal without `--no-tui` launches TUI mode
- [ ] Piped stdout (non-terminal) uses text reporter regardless of `--no-tui`
- [ ] `--no-tui` in an interactive terminal uses text reporter
- [ ] TUI program exits cleanly when the orchestrator finishes
- [ ] SIGINT (Ctrl-C) shuts down both TUI and orchestrator gracefully
- [ ] Logger does not write text to stdout in TUI mode (JSON to file only)

### Test cases

- **Terminal detection**: When `os.Stdout` is a terminal and `--no-tui` is
  false, TUI reporter is selected
- **Piped detection**: When `os.Stdout` is not a terminal, text reporter is
  selected regardless of `--no-tui`
- **Force text mode**: When `--no-tui` is true and stdout is a terminal, text
  reporter is selected
- **Graceful shutdown**: Sending SIGINT during a TUI run exits without panic
  and returns the orchestrator's context cancellation error
- **Logger file-only mode**: `NewLoggerFileOnly()` writes JSON to `debug.log`
  but produces no stdout output
