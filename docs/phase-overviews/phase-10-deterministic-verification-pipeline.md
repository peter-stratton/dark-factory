# Phase 10: Deterministic Verification Pipeline

Before Phase 10, the only thing standing between an agent's implementation and
the review cycle was a set of guard rails -- PR existence checks, `Closes #N`
enforcement, protected path drift detection. If the code didn't compile, the
reviewer agent would discover that. If tests failed, the reviewer would
discover that too. Every one of those discoveries cost a full review cycle:
tokens, wall-clock time, and a retry round-trip. Phase 10 inserts a
machine-driven verify step -- build, lint, test -- between implementation and
review, run by Go code on the orchestrator side. Failures get fed back to the
implementer automatically for a fix attempt before any reviewer sees the code.
On top of that, agents are now blocked from running destructive shell commands
like `rm -rf` or `git push --force`.

---

## Go-Side Verify Step

After the implementer finishes and guard rails pass, the orchestrator builds an
ordered list of checks from `build_command`, `lint_command`, and `test_command`
in `godark.yaml`. It runs them in sequence -- build first, then lint, then
test -- and stops at the first failure. Output is captured and truncated to 4 KB
so it fits in a follow-up prompt without overwhelming the context window.

The verify step runs in the same environment as the agent: inside a Docker
container when sandboxing is enabled, or on the host when `no_sandbox: true`.
The sandbox runner clones the repo, checks out the PR branch, and executes
the command -- the agent never touches the verify process.

**What you see in practice.** With this `godark.yaml`:

```yaml
build_command: "go build ./..."
lint_command: "golangci-lint run ./..."
test_command: "go test ./..."
```

The orchestrator log shows the verify step slotting in after guard rails:

```
INFO processing issue  issue_number=42 title="Add user profile endpoint"
INFO running verify step  issue_number=42 check_count=3
INFO verify step passed  issue_number=42
```

If the build fails, you see a structured failure instead of burning a review
cycle:

```
INFO running verify step  issue_number=42 check_count=3
WARN verify step failed  issue_number=42 failed_checks=["build"]
```

---

## Lint Command Config

Phase 10 adds `lint_command` as a first-class config field alongside
`build_command` and `test_command`. Same pattern: provide any command or shell
script path, dark-factory runs it and checks the exit code.

**Example config:**

```yaml
lint_command: "golangci-lint run ./..."
```

Or for a multi-language project:

```yaml
lint_command: "./scripts/lint.sh"
```

An empty string (or omitting the field entirely) skips the lint check. The
build and test checks work the same way -- if all three are empty, the entire
verify step is skipped.

---

## Auto-Fix Cycle

When verify fails, the orchestrator doesn't immediately give up or punt to the
reviewer. It renders a `verify_fix.txt` prompt containing the failing check
name and truncated error output, then sends it back to the implementer agent.
The agent resumes its previous session (same `session_id`), so it has full
context about what it built and why. After the agent pushes a fix, the
orchestrator re-runs verify. This loops up to `max_fix_attempts` times.

**What happens when a build breaks.** The implementer introduces a type error.
Verify catches it:

```
=== build (exit code 1) ===
./internal/handler/profile.go:47:12: cannot use resp (variable of type
*ProfileResponse) as type string in return statement
```

The orchestrator feeds this to the implementer via the verify-fix prompt. The
agent fixes the type, pushes, and verify re-runs. If it passes, the flow
continues to review. If it fails again and `max_fix_attempts` is exhausted, the
behavior depends on the `blocking` setting.

**Config controlling fix attempts:**

```yaml
verify:
  max_fix_attempts: 2   # default
  blocking: true        # default -- fail the issue if verify can't pass
```

Setting `blocking: false` lets verify failures through to the reviewer as
warnings -- useful during early adoption when your lint config might be noisy.

---

## Verify Behavior Config

The `verify:` block in `godark.yaml` controls two things: how many fix
attempts the orchestrator makes before giving up, and whether verify failures
block the issue or just warn.

**Strict mode (default):**

```yaml
verify:
  max_fix_attempts: 2
  blocking: true
```

A failing verify after two fix attempts marks the issue as `"failed"`. No
review cycle runs. No tokens wasted on reviewing broken code.

**Lenient mode:**

```yaml
verify:
  max_fix_attempts: 0
  blocking: false
```

Verify runs as a diagnostic. Failures are logged as warnings, but the issue
proceeds to review regardless. Useful for projects where the lint or test
commands are flaky and you want observability without hard gates.

---

## Sandbox-Mode Verify Runner

When sandboxing is enabled (the default), verify commands run inside a Docker
container using the same image as the agent containers. The container clones
the repo, checks out the PR branch, and executes the verify command via
`sh -c`. This means your verify environment matches your agent environment
exactly -- same OS packages, same SDK version, same PATH.

When `no_sandbox: true`, verify runs directly on the host via `sh -c`. The
orchestrator selects the runner automatically based on the config:

```go
if cfg.NoSandbox {
    verifyRunner = newHostRunner()
} else {
    verifyRunner = sandboxCommandRunner(cfg.Docker.Image, cfg.Repo, branch, logger)
}
```

No user action required -- the right runner is chosen based on the same
`no_sandbox` flag that controls agent execution.

---

## Bash Deny-List

Agents run shell commands freely. That's powerful but dangerous -- nothing
previously stopped an agent from running `rm -rf /workspace` or
`git push --force`. Phase 10 adds a configurable deny-list of destructive
command patterns. The agent runner's `PreToolUse` hook checks every Bash tool
call against the list using substring matching. Blocked commands get a system
message explaining why, so the agent can adjust its approach.

**Default deny-list:**

```yaml
denied_commands:
  - "rm -rf"
  - "git push --force"
  - "git push -f"
  - "git reset --hard"
  - "git clean -f"
```

These defaults ship out of the box. You can override them to tighten or loosen
the policy:

```yaml
denied_commands:
  - "rm -rf"
  - "git push --force"
  - "git push -f"
  - "git reset --hard"
  - "git clean -f"
  - "curl | sh"
  - "wget -O - | sh"
```

Setting `denied_commands: []` disables the hook entirely.

When an agent tries to run a blocked command, it sees a message like:

> Cannot run Bash command matching denied pattern: "rm -rf". The command
> "rm -rf /workspace/tests" contains a pattern that is blocked by
> GODARK_DENIED_COMMANDS to prevent destructive operations. Please adjust your
> approach to avoid this command.

The deny-list is passed to the Python agent runner via the
`GODARK_DENIED_COMMANDS` environment variable, keeping the hook logic
co-located with the other `PreToolUse` guards (protected paths, generated
paths).

---

## Run Data Integration

Every verify attempt -- initial and fix retries -- is recorded as a JSON file
in the run data directory. Each attempt writes to
`issues/<N>/verify-<attempt>.json` with structured results: which checks ran,
pass/fail per check, exit codes, truncated output, and whether a fix was
attempted.

**Example verify result file** (`issues/42/verify-0.json`):

```json
{
  "attempt": 0,
  "checks": [
    {"name": "build", "passed": true, "exit_code": 0},
    {"name": "lint", "passed": false, "output": "...", "exit_code": 1}
  ],
  "all_passed": false,
  "fix_attempted": false
}
```

After a fix attempt, `verify-1.json` appears:

```json
{
  "attempt": 1,
  "checks": [
    {"name": "build", "passed": true, "exit_code": 0},
    {"name": "lint", "passed": true, "exit_code": 0},
    {"name": "test", "passed": true, "exit_code": 0}
  ],
  "all_passed": true,
  "fix_attempted": true
}
```

The dashboard reads these files through the `rundata.Reader`, which loads
`VerifyResults` into the `IssueDetail` struct. Missing verify files are handled
gracefully for backward compatibility with runs from before Phase 10.

---

## Key Files

| Area | Path |
|---|---|
| Verify types and execution | `internal/agent/verify.go` |
| Verify tests | `internal/agent/verify_test.go` |
| Agent loop (verify wiring) | `internal/agent/loop.go` |
| Config (Verify, LintCommand, DeniedCommands) | `internal/config/config.go` |
| Verify-fix prompt template | `prompts/verify_fix.txt` |
| Bash deny-list hook | `internal/agent/runner/agent_runner.py` |
| Run data types (VerifyStepResult) | `internal/rundata/writer.go` |
| Run data reader (verify loading) | `internal/rundata/reader.go` |
| VerifyFix implementer function | `internal/agent/implementer.go` |
| Run data bridge | `internal/agent/rundata.go` |
