# Phase 1: Skeleton + Orchestration

> **Goal:** `godark run --milestone "Phase 1" --repo owner/repo --dry-run` works end-to-end.
> Fetches issues, resolves dependencies, sorts by priority, and prints the execution plan.
>
> No agent execution in this phase — just the orchestration brain.

## Milestone

`Phase 1`

---

## Issue 1: Project scaffold and CLI skeleton

### Description

Initialize the Go project and wire up a Cobra CLI with the top-level command
structure. The binary should compile and print help text.

### Key constraints

- Module path: `github.com/phs/dark-factory`
- Entry point: `cmd/godark/main.go`
- Use Cobra for CLI framework
- Subcommands to stub (no implementation yet): `run`, `status`, `vet`
- Flags on `run`: `--repo`, `--milestone`, `--issue`, `--max-retries`, `--dry-run`, `--no-sandbox`, `--config`
- `--config` defaults to `godark.yaml` in current directory
- Build target: `GOARCH=arm64 GOOS=darwin` (Mac Silicon)

### Acceptance criteria

- [ ] `go build ./cmd/godark` produces a binary
- [ ] `godark --help` shows subcommands (run, status, vet)
- [ ] `godark run --help` shows all flags with descriptions
- [ ] `GOARCH=arm64 GOOS=darwin go build ./cmd/godark` succeeds
- [ ] `go test ./...` passes (even if only a placeholder test exists)

### Test cases

- **Help output**: `godark --help` lists run, status, vet subcommands
- **Run flags**: `godark run --help` includes --repo, --milestone, --issue, --max-retries, --dry-run, --no-sandbox, --config
- **Build succeeds**: `go build ./cmd/godark` exits 0

---

## Issue 2: Configuration file parsing

**Blocked by**: #1 (Project scaffold)

### Description

Parse `godark.yaml` into a typed Go config struct. Support all fields
from the proposed config schema in `docs/CONTEXT.md`. CLI flags override
config file values.

### Key constraints

- Use `gopkg.in/yaml.v3` for parsing
- Config struct lives in `internal/config/config.go`
- Validation: repo is required (either from config or --repo flag), milestone or issue is required
- `Load(path string, flags CLIFlags) (*Config, error)` function that merges file + flags
- Missing config file is not an error if all required values come from flags

### Acceptance criteria

- [ ] Valid YAML file is parsed into Config struct with all fields populated
- [ ] CLI flags override config file values
- [ ] Missing config file + sufficient flags = no error
- [ ] Missing required fields (repo, milestone/issue) returns descriptive error
- [ ] `go test ./internal/config/` passes

### Test cases

- **Full config parse**: YAML with all fields populates every Config field correctly
- **Flag override**: --repo flag overrides repo in YAML
- **Missing file, flags sufficient**: No YAML file + --repo + --milestone = valid config
- **Missing file, flags insufficient**: No YAML file + no --repo = error mentioning "repo"
- **Minimal config**: YAML with only `repo` and `milestone` uses sensible defaults for everything else
- **Invalid YAML**: Malformed YAML returns a parse error

---

## Issue 3: GitHub issue fetching

**Blocked by**: #2 (Configuration)

### Description

Fetch open issues from a GitHub milestone using the `gh` CLI. Return them as
typed Go structs with the fields needed for orchestration.

### Key constraints

- Package: `internal/github/issues.go`
- Shell out to `gh issue list` (not the GitHub API directly) — keeps auth simple
- Fetch fields: number, title, body, labels, state
- Filter by milestone name
- Sort by priority label (p1 → p2 → p3 → unlabeled), then by issue number ascending within each tier
- Return `[]Issue` where Issue has: Number, Title, Body, Labels, Priority

### Acceptance criteria

- [ ] `FetchMilestoneIssues(repo, milestone)` returns sorted issues
- [ ] Priority ordering: p1 before p2 before p3 before unlabeled
- [ ] Within same priority: lower issue number first
- [ ] Issues without the milestone are excluded
- [ ] `go test ./internal/github/` passes

### Test cases

- **Priority sorting**: Issues with mixed p1/p2/p3/unlabeled labels sort correctly
- **Number sorting within tier**: Two p1 issues sort by number ascending
- **Body included**: Returned Issue structs include the full issue body text
- **Label parsing**: Labels are returned as string slices
- **Empty milestone**: Milestone with no open issues returns empty slice, no error

---

## Issue 4: Dependency resolver

**Blocked by**: #3 (GitHub issue fetching)

### Description

Parse issue bodies for dependency declarations (`Blocked by`, `Depends on`)
and filter out issues whose dependencies are still open.

### Key constraints

- Package: `internal/deps/resolver.go`
- Case-insensitive matching for `blocked by` and `depends on`
- Handle markdown formatting: `**Blocked by**: #1`, `Blocked by: #1, #3`, `Depends on: #5 (description)`
- Extract referenced issue numbers
- Given a list of issues and their states, return only unblocked issues (all dependencies are closed)
- Preserve the priority/number sort order from issue fetching

### Acceptance criteria

- [ ] Parses `**Blocked by**: #1 (description)` correctly
- [ ] Parses `Depends on: #3, #5` correctly (multiple deps)
- [ ] Case-insensitive matching works
- [ ] Issues with all dependencies closed are included in output
- [ ] Issues with any open dependency are excluded
- [ ] Issues with no dependency declarations are always included
- [ ] `go test ./internal/deps/` passes

### Test cases

- **Simple blocked by**: Issue with `**Blocked by**: #1` where #1 is open → excluded
- **Closed dependency**: Issue with `**Blocked by**: #1` where #1 is closed → included
- **Multiple dependencies**: `Depends on: #3, #5` where #3 closed, #5 open → excluded
- **All deps closed**: `Depends on: #3, #5` where both closed → included
- **No dependencies**: Issue body without any dependency keywords → included
- **Markdown bold variant**: `**Depends on**: #2` with bold markers → parsed correctly
- **Mixed case**: `BLOCKED BY: #4` → parsed correctly
- **Parenthetical description**: `Blocked by: #1 (CLI scaffold)` → extracts #1

---

## Issue 5: Structured logging foundation

**Blocked by**: #1 (Project scaffold)

### Description

Set up a logging package that writes structured JSON to a log file and
human-readable summaries to stdout. This will be used throughout the
orchestration loop and later by agents.

### Key constraints

- Package: `internal/logging/logger.go`
- Use `log/slog` from the standard library
- Two handlers: JSON file handler + text stdout handler (multi-handler)
- Log file path from config (`log_dir`), filename: `run-YYYYMMDD-HHMMSS.json`
- Standard fields on every log line: `timestamp`, `level`, `component`, `issue_number` (when applicable)
- Log levels: Debug, Info, Warn, Error
- `NewLogger(logDir string) (*slog.Logger, error)` creates both handlers

### Acceptance criteria

- [ ] Logger writes JSON lines to file
- [ ] Logger writes human-readable text to stdout
- [ ] Log file is created in configured directory with timestamp-based name
- [ ] Structured fields (component, issue_number) appear in JSON output
- [ ] `go test ./internal/logging/` passes

### Test cases

- **JSON output**: Log entry written to file is valid JSON with timestamp, level, msg fields
- **Stdout output**: Log entry appears on stdout in human-readable format
- **File naming**: Log file matches pattern `run-YYYYMMDD-HHMMSS.json`
- **Structured fields**: `logger.With("component", "orchestrator", "issue_number", 5)` includes those fields in JSON
- **Directory creation**: If log_dir doesn't exist, it is created

---

## Issue 6: Orchestration loop and dry-run mode

**Blocked by**: #3 (GitHub issue fetching), #4 (Dependency resolver), #5 (Structured logging)

### Description

Wire everything together into the `run` subcommand. In dry-run mode, fetch
issues, resolve dependencies, and print the execution plan without taking
any action. This is the capstone of Phase 1.

### Key constraints

- Package: `internal/orchestrator/orchestrator.go`
- `Run(config, logger)` is the main entry point
- Steps: load config → fetch issues → resolve deps → iterate unblocked issues in order
- In dry-run mode: log each issue that would be processed (number, title, priority, deps)
- In normal mode: log a "not implemented yet" placeholder for each issue (agent execution comes in Phase 2)
- Exit code 0 on success, 1 on fatal error
- Print summary at end: total issues in milestone, blocked count, processable count

### Acceptance criteria

- [ ] `godark run --repo owner/repo --milestone "Phase 1" --dry-run` prints execution plan
- [ ] Blocked issues are listed separately with their blocking reasons
- [ ] Processable issues are listed in priority/number order
- [ ] Summary line shows total/blocked/processable counts
- [ ] Structured log file is created with all orchestration events
- [ ] Normal mode (without --dry-run) iterates issues and logs placeholder messages
- [ ] `go test ./internal/orchestrator/` passes

### Test cases

- **Dry-run output**: Lists issues in correct order with priority and dependency info
- **Blocked issue reporting**: Blocked issues show which open issues block them
- **Summary counts**: Summary line matches actual issue counts
- **Log file created**: Run creates a JSON log file in log_dir
- **Empty milestone**: Milestone with no issues prints "no issues found" and exits 0
- **All blocked**: If all issues are blocked, prints warning and exits 0

---

## Issue 7: CLAUDE.md and scenario specs for Phase 2

**Blocked by**: #1 (Project scaffold)

### Description

Write the project's `CLAUDE.md` (standing orders for agents) and initial
scenario specs that will be used to validate Phase 2 work. This sets up
the contracts that godark's own agents will follow.

### Key constraints

- `CLAUDE.md` at project root — rules for agents working on this repo
- `tests/scenarios/` directory for human-authored scenario specs
- Scenario specs should cover the Phase 1 functionality (config parsing, issue fetching, dependency resolution, dry-run output)
- Follow the scenario spec format from `docs/CONTEXT.md`
- Include `Relates to: Issue #N` tags linking specs to issues

### Acceptance criteria

- [ ] `CLAUDE.md` exists with standing orders (protected paths, build commands, test commands, coding conventions)
- [ ] `tests/scenarios/` directory exists with at least 3 scenario spec files
- [ ] Scenario specs cover: config loading, dependency resolution, dry-run output
- [ ] Each spec uses the standard format (Setup, Cases, expected outcomes)
- [ ] Each spec includes `Relates to: Issue #N` linking to relevant issues
- [ ] Protected paths declared: `tests/scenarios/`, `CLAUDE.md`

### Test cases

- **CLAUDE.md content**: File includes build command, test command, protected paths, and coding conventions
- **Scenario format**: Each spec file has Setup section, Cases section with expected outcomes
- **Issue linkage**: Every scenario spec has at least one `Relates to:` reference
