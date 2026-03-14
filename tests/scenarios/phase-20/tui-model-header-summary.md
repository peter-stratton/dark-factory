# Scenario: TUI model, header, and summary bar

Relates to: Issue #442

## Setup
- `internal/tui/model.go` defines `Model` implementing `tea.Model`
- `internal/tui/header.go` provides `renderHeader()`
- `internal/tui/summary.go` provides `renderSummary()`
- `internal/tui/styles.go` defines centralized Lip Gloss styles
- Model is initialized with run metadata fields

## Cases

### Model satisfies tea.Model interface
Assign a `tui.Model` value to a variable of type `tea.Model`.
- The assignment compiles without error (compile-time interface check)

### Header renders all metadata fields
Call `renderHeader()` with repo `"phs/dark-factory"`, milestone `"Phase 20"`, timestamp `"20260314-142305"`, baseBranch `"phase-20"`, mergeFeature `"low_risk"`, mergeRollup `"manual"`.
- Output contains `"phs/dark-factory"`
- Output contains `"Phase 20"`
- Output contains `"20260314-142305"`
- Output contains `"phase-20"` (base branch)
- Output contains `"low_risk"` (feature merge setting)
- Output contains `"manual"` (rollup merge setting)

### Header omits base branch when empty
Call `renderHeader()` with baseBranch `""`.
- Output does not contain `"base:"` or `"Base branch:"`
- Other metadata fields still render normally

### Header omits auto-merge when not set
Call `renderHeader()` with mergeFeature `""` and mergeRollup `""`.
- Output does not contain `"auto-merge"` or `"feature="` or `"rollup="`

### Header uses Lip Gloss styles
Inspect the rendered header output.
- No raw ANSI escape codes like `\033[32m` are hardcoded in the source
- Styling is applied via `lipgloss.NewStyle()` calls

### Summary bar renders zero state
Call `renderSummary()` with all counts at 0 and cost 0.00.
- Output contains `"0 merged"`
- Output contains `"0 in review"`
- Output contains `"0 queued"`
- Output contains `"0 failed"`

### Summary bar renders non-zero counts
Call `renderSummary()` with implemented 3, readyToMerge 1, queued 4, failed 1.
- Output contains `"3 merged"`
- Output contains `"1 in review"`
- Output contains `"4 queued"`
- Output contains `"1 failed"`

### View composes header and summary
Call `View()` on a Model with metadata set.
- The returned string starts with header content
- The returned string ends with summary content
- Header appears before summary in the output
