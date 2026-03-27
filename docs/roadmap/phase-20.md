## Phase 20: Terminal UI ✅

**Goal**: `godark run` renders a live, rich terminal interface using Bubble Tea
when running interactively. The TUI shows run metadata, per-issue progress with
live status updates, and a summary bar — replacing the current structured log
output in interactive mode while preserving JSON output for piped/non-interactive
contexts.

**Milestone**: `Phase 20` | **Label**: `phase-20`

- Bubble Tea + Lip Gloss dependencies — add `github.com/charmbracelet/bubbletea`,
  `lipgloss`, and `bubbles` to `go.mod`
- TUI package scaffold — `internal/tui/` at the presentation layer; update
  `architecture.json` to include it alongside `internal/dashboard/`
- Header component — renders repo, milestone, timestamp, base branch, and
  auto-merge settings (feature + rollup), mirroring the dashboard run detail
  header
- Issue table component — rows with status markers (■ complete, ○ queued,
  spinner in-progress), issue number, title, current stage (recon → implement →
  verify → review → merged)
- Summary bar component — footer showing aggregate counts (merged, in review,
  queued, failed) and running cost total
- Live update event model — Bubble Tea message types for issue state
  transitions; orchestrator sends events as issues progress through the agent
  loop
- Hybrid output mode — TUI when `term.IsTerminal()` is true, current structured
  JSON/text when piped; `--no-tui` flag to force plain output
- Wire into `godark run` — replace `fmt.Println`/`logger.Info` output in the
  run command with TUI rendering; `debug.log` continues writing regardless of
  display mode
**Issues**: #439–#445

**Planning doc**: `docs/planning/phase-20-terminal-ui.md`

