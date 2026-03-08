# Phase 4: Agent Execution

Phase 4 is where Dark Factory stops planning and starts doing. It wires up
Claude Code agents that autonomously implement GitHub issues, review their own
work, and merge the results -- all inside Docker containers. The user points
godark at a milestone (or a single issue), walks away, and comes back to merged
PRs or clearly labeled escalations. Everything downstream in the project -- the
SDK migration, multi-language support, the dashboard -- builds on the agent
execution loop established here.

---

## Agent Launcher

**What it does:** Invokes a Claude agent with a rendered prompt template, either
inside a Docker sandbox container or directly on the host. Captures structured
output (session ID, cost, tool trace, verdict) from the agent's final JSON
line.

**Example:** When godark processes issue #42, the launcher renders the
implementer prompt template with the issue's title, body, repo, branch name,
and configured commands. It then runs the agent inside the pre-built Docker
image:

```
INFO running agent in sandbox  image=godark-abc123  timeout=30m0s
INFO host agent finished  exit_code=0
```

The launcher parses the agent's stdout for a final JSON line like:

```json
{"session_id": "sess_abc", "result": "...", "cost_usd": 0.47, "is_error": false, "tool_trace": ["Read:CLAUDE.md", "Edit:internal/foo.go", "Bash:go test ./..."]}
```

This structured output feeds the review loop, run data, and quality checks.

---

## Implementer Agent

**What it does:** Takes a GitHub issue, creates a feature branch, writes the
implementation and unit tests, then opens a pull request. In retry mode, it
checks out an existing PR, reads reviewer feedback, fixes the issues, and
pushes again. Session resumption means retries pick up where the agent left off
instead of cold-starting.

**Example:** A fresh implementation of issue #42:

```
$ godark implement 42

INFO starting implementer agent  issue_number=42  issue_title="Add user auth"  branch=42-add-user-auth
```

The agent receives a prompt that tells it to:
1. Read CLAUDE.md
2. Create branch `42-add-user-auth`
3. Write the code and tests
4. Run `go build ./...` and `go test ./...`
5. Commit with "Closes #42" and open a PR

If the reviewer later requests changes, the retry prompt tells the agent to
check out the PR, read the reviewer's `## Review Notes` comment, fix each
listed issue, and push. The retry reuses the agent's session ID so it
remembers its prior reasoning and file changes.

---

## Reviewer Agent

**What it does:** Checks out a PR, reads the matching scenario specs and the
implementer's notes, reviews the diff, generates ephemeral integration tests in
the configured review directory, runs them, and returns a verdict: `APPROVED`
or `CHANGES_REQUESTED`.

**Example:** After the implementer opens PR #87 for issue #42:

```
INFO starting reviewer agent  issue_number=42  pr_number=87
INFO reviewer finished  issue_number=42  pr_number=87  verdict=CHANGES_REQUESTED
```

The reviewer's prompt requires it to write integration tests to `tests/review/`,
run them, then clean up that directory before finishing. It posts a structured
`## Review Notes` comment on the PR listing specific issues the implementer must
fix. The orchestrator later checks the agent's tool trace to verify it actually
wrote files to the review directory -- if a reviewer approves without writing
tests, the approval is automatically rejected and the review re-runs.

---

## Guard Rails

**What it does:** Programmatic checks that run between agent steps -- not
prompt instructions the agent might ignore, but Go code the agent cannot
bypass. These catch issues that would be expensive to discover later.

The guard rails are:

- **PR existence check** -- if the implementer finishes but no PR exists on the
  expected branch, the issue fails immediately.
- **`Closes #N` auto-append** -- if the PR body is missing a closing reference,
  godark appends it via `gh pr edit` so the issue auto-closes on merge.
- **Protected path drift detection** -- diffs the PR against the base SHA; if
  any file in `protected_paths` was modified, the PR is closed with a comment
  explaining why.
- **Scenario spec warning** -- if no scenario spec references the issue, a
  warning comment is posted on the PR.
- **Review verdict parsing** -- extracts `REVIEW_RESULT=APPROVED` or
  `REVIEW_RESULT=CHANGES_REQUESTED` from the reviewer's structured JSON output,
  with fallback to text parsing.

**Example config in `godark.yaml`:**

```yaml
protected_paths: ["CLAUDE.md", "tests/scenarios/"]
```

If the implementer modifies `CLAUDE.md`, the guard rail fires:

```
INFO checking protected drift  base_sha=abc123
WARN closing PR: agent modified protected paths: [CLAUDE.md]
```

The PR is closed automatically with the comment: "Closing: agent modified
protected paths: [CLAUDE.md]".

---

## Review/Retry Loop

**What it does:** Orchestrates the back-and-forth between the implementer and
reviewer agents. If the reviewer requests changes, the implementer retries with
the reviewer's feedback. This repeats up to `max_retries` times. If the PR is
approved, it gets squash-merged and the branch is deleted. If retries are
exhausted, the PR is labeled `needs-human-review`.

**Example:** A run where the first review requests changes, the retry fixes
them, and the second review approves:

```
$ godark run --milestone "Phase 4"

INFO starting implementer agent  issue_number=42
INFO starting reviewer agent  issue_number=42  pr_number=87
INFO reviewer finished  verdict=CHANGES_REQUESTED
INFO retrying implementation  issue_number=42  attempt=1  retries_left=2
INFO starting implementer retry agent  issue_number=42  pr_number=87  resume_session=true
INFO starting reviewer agent  issue_number=42  pr_number=87
INFO reviewer finished  verdict=APPROVED
INFO pulled latest changes after merge
  #42 Add user auth -- implemented (PR #87, 1 retries)
```

When retries are exhausted without approval:

```
INFO retrying implementation  issue_number=42  attempt=3  retries_left=0
INFO reviewer finished  verdict=CHANGES_REQUESTED
  #42 Add user auth -- needs human review (PR #87)
```

The PR gets labeled `needs-human-review` and a comment explains how many cycles
were attempted.

---

## Orchestration Loop with Dependency Re-resolution

**What it does:** Processes a milestone's issues in priority order, respecting
dependency chains. After each successful merge, dependencies are re-resolved so
newly unblocked issues can run in the same invocation. Records a baseline
commit before each issue for rollback reference, and pulls `--rebase` after
each merge to keep the local repo in sync.

**Example:** A milestone with three issues where #44 depends on #43:

```
$ godark run --milestone "Phase 4" --dry-run

=== Execution Plan (dry-run) ===

Processable issues:
  #42 Add user auth [priority: p1]
  #43 Add session store [priority: p2]

Blocked issues:
  #44 Add session expiry (blocked by: #43)

Summary: 3 total, 1 blocked, 2 processable
```

In a live run, after #43 merges, the orchestrator re-fetches closed issues and
discovers #44 is now unblocked:

```
--- Wave 2: 1 newly unblocked issues ---
INFO starting implementer agent  issue_number=44
```

---

## Single-Issue Mode

**What it does:** Two ways to run a single issue without milestone or dependency
resolution. `godark run --issue N` processes one issue within a milestone
context. `godark implement N` skips milestone fetching entirely and goes
straight to the agent loop.

**Example:** Implementing a single issue directly:

```
$ godark implement 42
INFO starting implementer agent  issue_number=42  issue_title="Add user auth"
INFO starting reviewer agent  issue_number=42  pr_number=87
INFO reviewer finished  verdict=APPROVED
  #42 Add user auth -- implemented (PR #87, 0 retries)

Results: 1 implemented, 0 ready-to-merge, 0 needs-human-review, 0 failed
```

`godark implement` also supports multiple issues in one invocation:

```
$ godark implement 42 43 44
$ godark implement --issues 42,43,44
```

Both `run` and `implement` support `--no-merge` for cases where you want the
agent to get the PR approved but leave the merge to a human:

```
$ godark implement 42 --no-merge
  #42 Add user auth -- ready-to-merge (PR #87, 0 retries)
```

---

## Summary Stats

**What it does:** After all issues are processed, prints a one-line summary
showing how many issues were implemented, how many need human review, and how
many failed. Each issue also gets a per-issue status line as it completes.

**Example:**

```
  #42 Add user auth -- implemented (PR #87, 0 retries)
  #43 Add session store -- implemented (PR #88, 1 retries)
  #44 Add session expiry -- needs human review (PR #89)
  #45 Add rate limiting -- failed: implementer agent did not create a PR

Results: 2 implemented, 0 ready-to-merge, 1 needs-human-review, 1 failed, 0 skipped (blocked)
```
