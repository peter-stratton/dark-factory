# Phase 20: Terminal UI

godark has always communicated through structured log lines -- useful for debugging, but hard to scan during a live run. Phase 20 replaces the wall of `slog` text with a live terminal interface built on Bubble Tea. When stdout is an interactive terminal, `godark run` and `godark implement` render a full-screen TUI with a metadata header, a per-issue progress table with animated spinners, and a summary bar. When piped or redirected, the output falls back to plain text. The architecture is clean: a `ProgressReporter` interface in the foundation layer decouples the orchestrator from any display concern.

---

## Progress Reporter Interface

**What it does:** Defines a `ProgressReporter` interface in `internal/progress/` (foundation layer) that the orchestrator calls instead of `fmt.Printf`. Two implementations exist: `TextReporter` for plain-text output and `TUIReporter` for the Bubble Tea interface. The orchestrator doesn't know or care which one it's talking to.

**Example:** The interface uses only primitive types so it can live in the foundation layer without importing domain types:

```go
type ProgressReporter interface {
    RunStarted(repo, milestone, timestamp, baseBranch, mergeFeature, mergeRollup string, issueCount int)
    IssueStarted(issueNumber int, title string)
    IssueStageChanged(issueNumber int, stage string)
    IssueCompleted(issueNumber int, title, status string, prNumber, retries int, errMsg string)
    WaveStarted(wave, count int)
    AllBlocked(total, blocked int)
    RollupCreated(prNumber int, prURL string, merged bool)
    RunFinished(implemented, readyToMerge, needsHumanReview, failed, blocked int)
    PunchlistText(text string)
}
```

The `TextReporter` in `internal/progress/text.go` writes the exact same format strings the orchestrator used to emit directly. `IssueCompleted` with status `"implemented"` writes `"  #42 add cost tracking — implemented (PR #87, 0 retries)\n"` -- byte-for-byte identical to the pre-Phase 20 output. Users who pipe godark output to scripts see no difference.

---

## TUI Header

**What it does:** The header chrome mirrors the dashboard's run detail view. Line one shows the `godark` logo left-aligned with the repo name right-aligned. Line two shows milestone, run timestamp, base branch (when configured), and auto-merge settings (when configured). Empty fields are omitted cleanly -- no stray separators.

**Example:** A run of `godark run --tag phase-19 --base-branch feature/phase-19` renders:

```
 godark                                    phs/dark-factory
 Phase 19: Spring Cleaning · 20260312-093015 · base: feature/phase-19 · auto-merge: feature=all rollup=manual
```

The timestamp comes from the run data directory name, populated when the orchestrator calls `reporter.RunStarted()`:

```go
var runTimestamp string
if writer != nil {
    runTimestamp = filepath.Base(writer.Dir())
}
reporter.RunStarted(cfg.Repo, milestone, runTimestamp, cfg.BaseBranch,
    string(cfg.AutoMerge.Feature), string(cfg.AutoMerge.Rollup), len(issues))
```

A `godark implement 42` run without a milestone or base branch renders a minimal header:

```
 godark                                    phs/dark-factory
 auto-merge: feature=all rollup=manual
```

---

## Issue Table

**What it does:** Each issue in the run is rendered as a row with a status marker, issue number, title, and a status badge. Rows appear as issues start processing and update live as they progress through stages. In-progress issues show an animated spinner.

**Example:** During a Phase 19 run with 5 issues, the table might look like:

```
■ #384 unified verdict parser                  MERGED
■ #385 extract shared CRITICAL RULES template  MERGED
■ #386 extract review cycle function           MERGED
● #387 extract handoff policy function         REVIEW
○ #388 consolidate CLI flag parser             QUEUED
```

Status markers and badges are mapped by a state machine in `internal/tui/table.go`:

| Status | Marker | Badge | Color |
|--------|--------|-------|-------|
| `implemented` / `merged` | `■` | `MERGED` | green |
| `ready-to-merge` / `needs-human-review` | `●` | `REVIEW` | yellow |
| `failed` | `✕` | `FAILED` | red |
| in-progress (stage set, no final status) | spinner | stage name | muted |
| queued (no stage, no status) | `○` | `QUEUED` | muted |

Long issue titles are truncated with `…` to fit the terminal width. The table recalculates layout on every `WindowSizeMsg`.

---

## Summary Bar

**What it does:** A footer line below a horizontal divider shows aggregate counts and running cost, styled with semantic colors -- green for merged, yellow for in-review, red for failed.

**Example:** After a run completes:

```
────────────────────────────────────────
3 merged · 1 in review · 8 queued · 0 failed · $2.14 total cost
```

The summary updates when `RunFinishedMsg` arrives. The `Update()` handler maps the message fields to display counts:

```go
case RunFinishedMsg:
    m.merged = msg.Implemented
    m.inReview = msg.ReadyToMerge + msg.NeedsHumanReview
    m.failed = msg.Failed
    m.queued = msg.Blocked
```

---

## Adaptive Color Scheme

**What it does:** All colors use Lip Gloss `AdaptiveColor` pairs that automatically switch between light and dark palettes based on the terminal's background color. The TUI is readable on both dark terminals (iTerm, Alacritty) and light terminals (macOS default, VS Code light theme).

**Example:** The color palette in `internal/tui/styles.go` defines pairs for every semantic color:

```go
colorBright = lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#FFFDF5"}
colorGreen  = lipgloss.AdaptiveColor{Light: "#067D52", Dark: "#04B575"}
colorYellow = lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#F9E04B"}
colorRed    = lipgloss.AdaptiveColor{Light: "#CC3355", Dark: "#FF5F87"}
```

On a dark terminal, "bright" text renders as near-white (`#FFFDF5`). On a light terminal, the same text renders as near-black (`#1A1A1A`). Lip Gloss detects the terminal background at startup and selects the right value automatically -- no user configuration needed.

Badge styles also adapt. The `MERGED` badge uses white text on a green background on light terminals, and dark text on green on dark terminals:

```go
badgeMergedStyle = lipgloss.NewStyle().
    Foreground(lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#1A1A1A"}).
    Background(colorGreen).
    Bold(true).
    Padding(0, 1)
```

---

## Hybrid Output Mode

**What it does:** The TUI activates automatically when stdout is an interactive terminal. When output is piped, redirected, or run in CI, godark falls back to `TextReporter` and produces the same plain-text output as before Phase 20. A `--no-tui` flag forces plain-text mode even in interactive terminals.

**Example:** Normal interactive usage launches the TUI:

```
$ godark run --tag phase-19
```

The alt-screen activates, the header renders, and issues stream in. Piped output skips the TUI entirely:

```
$ godark run --tag phase-19 | tee run.log
```

This produces the traditional text output -- one line per issue outcome, a results summary at the end. Scripts that parse godark output are unaffected.

To force text mode in an interactive terminal (useful for debugging or copying output):

```
$ godark run --tag phase-19 --no-tui
```

The detection logic in `internal/cmd/run.go`:

```go
useTUI := !noTUI && isTerminalFn(int(os.Stdout.Fd()))
```

Both `godark run` and `godark implement` support `--no-tui`.

---

## File-Only Logger for TUI Mode

**What it does:** When the TUI is active, it owns the screen -- `slog` text output would corrupt the display. `NewLoggerFileOnly()` in `internal/logging/logger.go` creates a logger that writes JSON to `debug.log` but produces no stdout output. The full structured log is always available on disk regardless of display mode.

**Example:** In TUI mode, `run.go` selects the file-only logger factory:

```go
logFactory := logging.NewLogger
if useTUI {
    logFactory = logging.NewLoggerFileOnly
}
```

The orchestrator uses this factory to create the run-directory logger. All `logger.Info()`, `logger.Warn()`, and `logger.Error()` calls still write to `debug.log` in the run directory -- they just don't echo to stdout. The log viewer in `godark status` shows the same data regardless of whether the run used TUI mode or text mode.

---

## TUI Reporter

**What it does:** `TUIReporter` in `internal/tui/reporter.go` bridges the `ProgressReporter` interface and Bubble Tea's message-driven update loop. Each reporter method translates its arguments into a typed Bubble Tea message and sends it to the running program via `p.Send()`.

**Example:** When the orchestrator calls `reporter.IssueCompleted(42, "add endpoint", "implemented", 87, 0, "")`, the TUI reporter sends:

```go
func (r *TUIReporter) IssueCompleted(issueNumber int, title, status string, prNumber, retries int, errMsg string) {
    r.program.Send(IssueCompletedMsg{
        Number:   issueNumber,
        Title:    title,
        Status:   status,
        PRNumber: prNumber,
        Retries:  retries,
        ErrMsg:   errMsg,
    })
}
```

The Bubble Tea event loop picks up the message, `Model.Update()` finds the matching issue row by number, updates its status and badge, and `Model.View()` re-renders the table on the next frame. The orchestrator runs in a goroutine; the Bubble Tea program runs on the main thread. When the orchestrator finishes, it sends `tea.QuitMsg{}` to exit the TUI cleanly.

`PunchlistText` is a deliberate no-op in TUI mode -- punchlist output is written to the file specified by `--punchlist` and doesn't clutter the live display.

---

## Architecture

**What it does:** The TUI follows the existing presentation layer pattern established by the dashboard. `internal/tui/` sits in the presentation layer alongside `internal/dashboard/`, with the same dependency rules: may import from domain and foundation, must not import from orchestration, service, infrastructure, or content.

**Example:** The layer registration in `docs/architecture.json`:

```json
{
  "name": "presentation",
  "paths": ["internal/dashboard/", "internal/tui/"],
  "may_depend_on": ["domain", "foundation"],
  "must_not_depend_on": ["cmd", "orchestration", "service", "infrastructure", "content"]
}
```

The `ProgressReporter` interface lives in `internal/progress/` (foundation layer) so both the orchestrator (orchestration layer) and the TUI reporter (presentation layer) can import it without creating a cycle. The `TextReporter` also lives in foundation. The `TUIReporter` lives in `internal/tui/` because it imports Bubble Tea -- a presentation concern.

The cmd layer (`internal/cmd/run.go`, `internal/cmd/implement.go`) is the only place where the concrete reporter type is chosen. The orchestrator accepts a `ProgressReporter` and never knows whether it's talking to a terminal or a plain-text writer.
