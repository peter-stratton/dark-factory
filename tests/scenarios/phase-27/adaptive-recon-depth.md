# Scenario: Adaptive recon depth by issue type

Relates to: Issue #633

## Setup
- `PromptData` in `internal/agent/prompt.go` has a `ReconDepth` field
- `internal/agent/loop.go` contains the depth detection heuristic before the
  recon call
- `prompts/recon.txt` uses `{{.ReconDepth}}` conditional sections
- `internal/config/config.go` has a `recon_depth` config field

## Cases

### Wiring issue auto-detects as light
Create an issue with title "Wire Labels struct into callers".
Run the depth detection heuristic.
- Detected depth is `"light"`

### Refactor issue auto-detects as light
Create an issue with title "Refactor label package from constants to struct".
Run the depth detection heuristic.
- Detected depth is `"light"`

### Issue with many code blocks auto-detects as skip
Create an issue with body containing 4 fenced code blocks.
Run the depth detection heuristic.
- Detected depth is `"skip"`

### Feature issue auto-detects as full
Create an issue with title "Add expense repository" and body with 1 code block.
Run the depth detection heuristic.
- Detected depth is `"full"`

### Config override forces depth
Set `recon_depth: light` in config.
Run depth detection for an issue that would normally be `"full"`.
- Detected depth is `"light"` (config overrides heuristic)

### Auto config uses heuristic
Set `recon_depth: auto` in config (or leave empty).
Run depth detection for a wiring issue.
- Detected depth is `"light"` (heuristic applies)

### Light recon renders only Priority 1
Render `recon.txt` with `ReconDepth: "light"`.
- Output contains the Priority 1 section
- Output does not contain Priority 2 or Priority 3 sections

### Full recon renders all priorities
Render `recon.txt` with `ReconDepth: "full"`.
- Output contains Priority 1, Priority 2, and Priority 3 sections

### Skipped recon bypasses agent call
Set depth to `"skip"` for an issue.
Run `ProcessIssue`.
- Recon agent is not invoked
- Implementer receives empty recon brief
- Skip reason is logged
