# Dark Factory — Context Seed

> A Go CLI that orchestrates autonomous AI agents to implement GitHub issues,
> review their own work, and merge — without human intervention.

This document captures everything learned from building a bash prototype of this
system. Use it to build a proper Go CLI from scratch.

---

## What This Tool Does

Dark Factory is a two-agent development loop. Given a GitHub repo and a milestone:

1. **Fetch** open issues from the milestone, sorted by issue number (ascending)
2. **Check dependencies** — issues declare `Blocked by: #N` or `Depends on: #N` in their body; skip any issue whose dependencies are still open
3. **Agent 1 (Implementer)** — spins up Claude Code to implement the issue, write unit tests, and open a PR
4. **Guard checks** — verify the PR exists, contains `Closes #N`, and didn't touch protected files
5. **Agent 2 (Reviewer)** — spins up a separate Claude Code instance that reads human-authored scenario specs, generates ephemeral integration tests against the real code, runs all tests, and approves or requests changes
6. **Retry loop** — if the reviewer rejects, Agent 1 reads the review comments and pushes fixes (max N retries)
7. **Merge or escalate** — approved PRs are squash-merged; failed PRs are labeled `needs-human-review`
8. **Repeat** — move to the next unblocked issue

---

## Architecture Learned from the Prototype

### Two-agent separation is critical

Agent 1 implements. Agent 2 reviews. They never share a session. Agent 2 reads
Agent 1's code via `gh pr diff` and `gh pr checkout`, not by sharing context.
This prevents the implementing agent from "grading its own homework."

### Three-layer test strategy

| Layer | Who writes | Where it lives | When it runs |
|---|---|---|---|
| **Unit tests** | Agent 1 | `internal/foo/foo_test.go` | Agent 1 runs `go test ./...` before opening PR |
| **Scenario specs** | Human | `tests/scenarios/*.md` | Agent 2 reads these as behavioral contracts |
| **Review tests** | Agent 2 | `tests/review/` (ephemeral) | Agent 2 generates Go tests from specs + real code, runs them, deletes them |

The scenario specs are markdown, not code. The human describes *what* should be true.
Agent 2 reads the spec plus the actual implementation and generates runnable tests.
This avoids the problem of writing Go tests in advance without knowing the
implementation's function signatures or package structure.

### Scenario spec format

```markdown
# Scenario: Feature name

Relates to: Issue #N

## Setup
- Description of test fixture state

## Cases

### Case name
Description of what to do.
- Expected outcome 1
- Expected outcome 2
```

The `Relates to: Issue #N` line is how the script maps specs to issues (via grep).
An issue can have multiple related specs, and a spec can relate to multiple issues.

### Dependency detection

Issues declare dependencies in their body using markdown:

```
**Blocked by**: #1 (Telegram bot)
```

or:

```
Depends on: #3, #5
```

The tool parses these with case-insensitive matching and checks each referenced
issue's state. If any dependency is open, the issue is skipped. After a PR merges
and closes an issue, the next iteration picks up newly-unblocked issues.

### Priority ordering

Issues are fetched by priority label (p1, p2, p3, then unlabeled) and then sorted
by issue number ascending within each priority tier. This ensures build-order
dependencies are naturally respected even without explicit `Blocked by` declarations.

---

## Lessons Learned (Bugs and Fixes from Production Use)

### Agent committed directly to main

**Problem:** Agent 1 pushed a commit to main instead of creating a feature branch,
bypassing Agent 2's review entirely.

**Fix (prompt):** Added `CRITICAL: create a feature branch` and
`NEVER commit directly to main` to the agent prompt.

**Fix (infrastructure):** The script should verify a PR exists after Agent 1 finishes.
If no PR is found, the issue is marked as failed. Also: enable branch protection
on the target repo's main branch to prevent direct pushes.

**Fix (Go CLI):** Consider having the CLI create the feature branch itself before
launching Agent 1, rather than relying on the agent to do it.

### Agent forgot `Closes #N` in PR

**Problem:** If the agent doesn't include `Closes #N` in the commit message or
PR body, the issue doesn't auto-close on merge, breaking the dependency chain.

**Fix:** After Agent 1 opens a PR, the script checks the PR title and body for
a closing keyword. If missing, it appends `Closes #N` to the PR body automatically.

### `git pull` failed after merge due to uncommitted local changes

**Problem:** The repo had uncommitted changes from the user. After squash-merge,
`git pull` couldn't fast-forward.

**Fix:** Use `git pull --rebase origin main` instead of `git pull origin main`.

**Better fix for the Go CLI:** The CLI should work in a git worktree or a clean
clone, never in the user's working directory. This eliminates the entire class
of "dirty working tree" problems.

### Issue ordering was reversed

**Problem:** `gh issue list` returns newest first by default. The loop processed
issues in reverse order (8, 7, 6... instead of 1, 2, 3...).

**Fix:** Sort issue numbers ascending after fetching.

### Dependency grep didn't match markdown bold syntax

**Problem:** Issues used `**Blocked by**: #1` (with markdown bold markers).
The grep pattern `[Bb]locked by[: ]` didn't match because `**` preceded the text.

**Fix:** Simplified to `grep -oi 'blocked by.*'` which matches regardless of
surrounding markdown formatting.

### Claude Code interactive prompts blocked headless execution

**Problem:** Several interactive prompts appear in a fresh Claude Code environment:
1. First-run theme selection wizard
2. Workspace trust dialog
3. `--dangerously-skip-permissions` confirmation

**Fix (theme/trust):** Pre-configure `~/.claude.json` with:
```json
{
  "hasCompletedOnboarding": true,
  "theme": "dark",
  "numStartups": 1,
  "projects": {
    "/workspace": {
      "hasTrustDialogAccepted": true,
      "hasCompletedProjectOnboarding": true
    }
  }
}
```

**Fix (permissions):** Use `-p` (print mode) flag alongside
`--dangerously-skip-permissions`. Print mode suppresses all interactive prompts.

### Claude Code native binary is macOS-only

**Problem:** The native `claude` binary is a Mach-O arm64 executable.
Docker containers (Linux) need the npm package.

**Fix:** Install via `npm install -g @anthropic-ai/claude-code` in the Dockerfile.
This requires Node.js in the container.

### Container ran as root

**Problem:** Claude Code refuses `--dangerously-skip-permissions` when running as root.

**Fix:** Create a non-root user in the Dockerfile:
```dockerfile
RUN useradd -m -s /bin/bash devloop
USER devloop
```

---

## Docker Sandbox Design

The prototype runs in Docker for safety. Key design decisions:

- **Mount the repo** at `/workspace` — agent changes persist to the real repo
  (this is intentional; PRs and commits need to land in the real remote)
- **Non-root user** — Claude Code requires this for `--dangerously-skip-permissions`
- **Pre-configured `.claude.json`** — skips all interactive first-run prompts
- **Auth via environment variables:**
  - `CLAUDE_CODE_OAUTH_TOKEN` — from `claude setup-token` (subscription-based)
  - `GH_TOKEN` — from `gh auth token`
  - `ANTHROPIC_API_KEY` — alternative to OAuth token (pay-per-token)
- **Logged output** — script-level events go to `logs/dev-loop-YYYYMMDD-HHMMSS.log`
  (agent session output stays ephemeral; only the structured log survives)

### What the Go CLI should do differently

Instead of mounting the user's working directory, the CLI should:
1. Clone the repo (or create a worktree) into a temp directory inside the container
2. Run all agent work against the clone
3. PRs and pushes go to the remote (not the local checkout)
4. The user's working directory is never touched

This eliminates dirty-tree conflicts entirely.

---

## Agent Prompt Design

### Implementer (fresh mode)

```
Read CLAUDE.md completely before doing anything else.

Then implement GitHub issue #N.

Before writing any code:
1. Read the full issue body carefully, including the ## Test cases section
2. Identify every file you expect to create or modify
3. Post a brief implementation plan as a comment on issue #N via gh CLI

CRITICAL: Before writing any code, create a feature branch:
  git checkout -b issue-N-<slugified-title>
NEVER commit directly to main. All work must be on a feature branch.

Then implement the issue exactly as specified:
- Write the implementation code
- Write unit tests alongside the code (e.g. internal/foo/foo_test.go)
- Unit tests should cover the test cases listed in the issue
- Do not modify anything in tests/scenarios/ under any circumstances

After implementation:
- Run: go test ./... (must pass)
- Run: GOARCH=arm64 GOOS=linux go build -o bin/ ./cmd/... (must pass)
- Do NOT read, modify, or create files in tests/scenarios/ or tests/review/
- Commit all changes with message that includes 'Closes #N'
- Push the feature branch and open a PR targeting main
- Do NOT push to main directly under any circumstances
```

### Implementer (retry mode)

```
Read CLAUDE.md completely before doing anything else.

You are fixing review failures on PR #P for issue #N.

Steps:
1. Check out the PR branch: gh pr checkout P
2. Read the review comments to understand what failed:
   gh pr view P --comments --json reviews,comments
   Also read the latest review: gh pr reviews P
3. Read the issue body for #N to recall the requirements
4. Fix the issues described in the review
5. Run: go test ./... (must pass)
6. Run: GOARCH=arm64 GOOS=linux go build -o bin/ ./cmd/... (must pass)
7. Do NOT read, modify, or create files in tests/scenarios/ or tests/review/
8. Commit the fixes with a message like 'Fix review feedback for #N'
9. Push to the existing PR branch: git push
```

### Reviewer

```
You are a code reviewer. Your job is to validate a PR against scenario
specs and acceptance criteria.

Read CLAUDE.md first.

Then:
1. Check out PR #P: gh pr checkout P
2. Read these scenario spec files: [list of matched files]
   Do NOT modify these files.
3. Read the issue body for #N to understand the acceptance criteria.
4. Review the code changes: gh pr diff P
5. Run the agent's unit tests: go test ./... -v -count=1
6. Verify cross-compilation: GOARCH=arm64 GOOS=linux go build -o bin/ ./cmd/...
7. Based on the scenario spec and the actual implementation, write Go integration
   tests in tests/review/ (create the directory). These tests should:
   - Import or invoke the actual code from this PR
   - Validate the behaviors described in the markdown scenario spec
   - Use temp directories, never touch real data
   - Be runnable with: go test ./tests/review/ -v -count=1
8. Run your generated tests: go test ./tests/review/ -v -count=1

If ALL tests pass and acceptance criteria are met:
- Delete tests/review/
- Approve the PR
- Print exactly: REVIEW_RESULT=APPROVED

If any test fails or criteria are not met:
- Delete tests/review/
- Request changes with detailed failure description
- Print exactly: REVIEW_RESULT=CHANGES_REQUESTED

IMPORTANT:
- Do NOT modify any files except in tests/review/
- Do NOT push any commits
- Always clean up tests/review/ before finishing
```

---

## CLI Design (Proposed)

```
dark-factory run --milestone "Phase 1" --repo owner/repo
dark-factory run --milestone "Phase 1" --repo owner/repo --dry-run
dark-factory run --milestone "Phase 1" --repo owner/repo --max-retries 3
dark-factory run --issue 42 --repo owner/repo          # single issue mode
dark-factory status --repo owner/repo                   # show run log summary
```

### Configuration file (`dark-factory.yaml`)

```yaml
repo: owner/repo
milestone: "Phase 1"
max_retries: 2

# Agent execution
claude_flags: ["-p", "--dangerously-skip-permissions"]
build_command: "go build -o bin/ ./cmd/..."
test_command: "go test ./..."
cross_compile:
  GOOS: linux
  GOARCH: arm64

# Protected paths (agents cannot modify these)
protected_paths:
  - tests/scenarios/
  - CLAUDE.md

# Scenario spec directory
scenario_dir: tests/scenarios/

# Ephemeral review test directory (gitignored)
review_dir: tests/review/

# Logging
log_dir: logs/

# Docker sandbox
docker:
  image: dark-factory-runner
  dockerfile: Dockerfile.devloop
  mount: /workspace
  user: devloop

# Prompt templates (can be overridden per-project)
prompts:
  implementer: prompts/implementer.md
  implementer_retry: prompts/implementer-retry.md
  reviewer: prompts/reviewer.md
```

### Key design decisions for the Go CLI

1. **Prompt templates are files, not hardcoded strings** — users can customize
   agent behavior per project without modifying the CLI source

2. **Build/test commands are configurable** — not every project is Go; the tool
   should work for any language where you can express "build" and "test" as shell commands

3. **Protected paths are declarative** — instead of hardcoding `tests/scenarios/`,
   let the config specify which paths agents cannot touch

4. **Docker is the default execution mode** — running agents on the host should
   require an explicit `--no-sandbox` flag

5. **Structured logging** — JSON log lines to a file, human-readable summary to
   stdout. The log file is the primary artifact for debugging failed runs.

6. **Single-issue mode** — `--issue N` for testing the loop on one issue without
   processing an entire milestone

---

## Guard Rails (Script-Level, Not Prompt-Level)

These are checks the CLI performs regardless of what the agent does:

| Guard | When | Action on failure |
|---|---|---|
| PR exists | After Agent 1 | Mark issue as failed, continue |
| `Closes #N` in PR | After Agent 1 | Auto-append to PR body |
| No protected path changes | After Agent 1 | Close PR, mark failed |
| No scenario spec found | Before Agent 2 | Comment on PR warning, continue |
| `REVIEW_RESULT` in output | After Agent 2 | If missing, mark as failed |
| Retry limit exceeded | After Agent 2 | Label `needs-human-review` |
| Baseline commit recorded | Before Agent 1 | Log hash for rollback |

---

## What the Prototype Got Right

- Two-agent separation (implement vs review) prevents self-grading
- Markdown scenario specs let humans define "what done looks like" without
  knowing implementation details
- Dependency detection from issue bodies respects build order
- Retry loop with review feedback gives agents a fair shot at fixing mistakes
- Structured logging survives overnight unattended runs
- Git baseline hashes enable rollback without tag clutter
- `Closes #N` auto-append is belt-and-suspenders for issue lifecycle
- Protected path checking prevents agents from modifying human-authored specs

## What the Prototype Got Wrong

- Running in the user's working directory caused merge conflicts
- Hardcoded Go-specific commands (should be configurable for any language)
- Prompt templates embedded in bash strings (hard to maintain, no syntax highlighting)
- No structured output format (grepping for `REVIEW_RESULT=APPROVED` is fragile)
- No timeout on agent execution (a stuck agent blocks the entire loop)
- No cost tracking (running 8 issues through two agents adds up)
- Agent 2's reviewer prompt doesn't prevent Agent 2 from connecting to the reviewer
  prompt's pre-existing git state; should always start from a clean checkout
- No way to resume a partially-completed run (if the script crashes at issue #5,
  you restart from #1 and re-skip closed issues, which is slow)
