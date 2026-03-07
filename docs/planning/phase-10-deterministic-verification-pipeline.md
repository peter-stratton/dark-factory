# Phase 10: Deterministic Verification Pipeline

> **Goal:** Agent implementation passes through a deterministic verify step
> (build + lint + test) run by Go code — not by the agent — before review
> begins. Failures are fed back to the implementer automatically, saving
> review cycles and tokens. Agents are also restricted from running
> destructive shell commands.

## Milestone

`Phase 10`

---

## Issue 176: Lint command and verify config block

### Description

Add configuration fields for the lint command and the verify step behavior.
The `lint_command` follows the same pattern as `build_command` and
`test_command` — a user-provided string (can be a shell script path) that
dark-factory runs and checks the exit code. The `verify:` block controls
fix attempt limits and whether verify failures block review.

A new `verify_fix` prompt path is also added to the `Prompts` struct so the
fix cycle prompt can be configured or overridden.

### Key constraints

- Modify `internal/config/config.go`:
  - Add `LintCommand string` field with yaml tag `lint_command`
  - Add `Verify` struct:
    ```go
    type Verify struct {
        MaxFixAttempts int  `yaml:"max_fix_attempts"`
        Blocking       bool `yaml:"blocking"`
    }
    ```
  - Add `Verify Verify` field to `Config` with yaml tag `verify`
  - Add `VerifyFix string` field to `Prompts` struct with yaml tag `verify_fix`
  - Defaults: `LintCommand: ""` (skip), `Verify.MaxFixAttempts: 2`,
    `Verify.Blocking: true`

### Acceptance criteria

- [ ] `Config` has `LintCommand` field, empty by default
- [ ] `Config` has `Verify` struct with `MaxFixAttempts` and `Blocking` fields
- [ ] `Prompts` struct has `VerifyFix` field
- [ ] Defaults are `MaxFixAttempts: 2`, `Blocking: true`, `LintCommand: ""`

### Test cases

- **Config defaults**: New config has correct verify defaults
- **Lint command override**: Setting `lint_command: "./scripts/lint.sh"` in
  YAML is reflected in parsed config
- **Verify block override**: Setting `verify: {max_fix_attempts: 5, blocking: false}`
  is reflected in parsed config
- **Verify fix prompt path**: Setting `prompts: {verify_fix: "custom/fix.txt"}`
  is reflected in parsed config

---

## Issue 177: Verify step execution logic

### Description

New file `internal/agent/verify.go` containing the types and execution logic
for running deterministic verification checks (build, lint, test) against the
code an agent just pushed. Each check runs a configured command and captures
a structured pass/fail result with summarized output.

The verify step runs commands via a `CommandRunner` function parameter so the
caller can provide either a host runner or a sandbox runner. Output is
truncated to a reasonable length (4 KB) to avoid overwhelming the fix prompt.

This is pure new code — no existing files are modified.

### Key constraints

- New file: `internal/agent/verify.go`
- Exported types:
  ```go
  // Check defines a single verification command to run.
  type Check struct {
      Name    string // "build", "lint", or "test"
      Command string // shell command to execute
  }

  // CheckResult holds the outcome of a single verification check.
  type CheckResult struct {
      Name    string `json:"name"`
      Passed  bool   `json:"passed"`
      Output  string `json:"output"`  // combined stdout+stderr, truncated
      ExitCode int   `json:"exit_code"`
  }

  // VerifyResult holds the outcome of running all verification checks.
  type VerifyResult struct {
      Checks    []CheckResult `json:"checks"`
      AllPassed bool          `json:"all_passed"`
  }

  // CommandRunner executes a shell command and returns stdout, stderr,
  // exit code, and any execution error.
  type CommandRunner func(ctx context.Context, command string) (stdout, stderr []byte, exitCode int, err error)
  ```
- Exported function:
  ```go
  // RunVerify executes each check in sequence using the provided runner.
  // It stops at the first failure unless all checks are requested.
  // Returns a VerifyResult with outcomes for all checks that were run.
  func RunVerify(ctx context.Context, checks []Check, run CommandRunner) VerifyResult
  ```
- `RunVerify` skips checks with an empty `Command` string
- Output truncation: keep the last 4096 bytes of combined stdout+stderr
- Checks run in order: build, then lint, then test — stops at first failure
  (no point running tests if the build fails)

### Acceptance criteria

- [ ] `RunVerify` runs checks in sequence and returns structured results
- [ ] Checks with empty command are skipped (not counted as failures)
- [ ] Execution stops at the first failing check
- [ ] Output is truncated to 4096 bytes (keeping the tail)
- [ ] A passing run has `AllPassed: true` and all checks have `Passed: true`

### Test cases

- **All pass**: Three checks that exit 0 produce `AllPassed: true`
- **Build fails**: Build exits 1 — only build result returned, lint and test
  skipped
- **Lint fails**: Build passes, lint exits 1 — build and lint results returned,
  test skipped
- **Empty command skipped**: Check with `Command: ""` is omitted from results
- **All empty**: All checks have empty commands — `AllPassed: true`, empty
  `Checks` slice
- **Output truncation**: Check producing 10 KB of output is truncated to last
  4096 bytes
- **Context cancellation**: Cancelled context stops execution and returns
  results so far

---

## Issue 178: Verify fix prompt template

### Description

New prompt template `prompts/verify_fix.txt` for the fix cycle after a verify
failure. The prompt tells the implementer agent which checks failed, includes
the truncated error output, and asks it to fix the issues and push.

This also adds a `VerifyErrors` field to `PromptData` so the template can
render the structured failure summary, and loads the new prompt in
`LoadPrompts`.

### Key constraints

- New file: `prompts/verify_fix.txt` — embedded via existing `//go:embed`
  directive in `prompts/embed.go`
- Modify `internal/agent/prompt.go`:
  - Add `VerifyErrors string` field to `PromptData`
  - Add `VerifyFix string` field to `Prompts` struct
  - Load `verify_fix.txt` in `LoadPrompts` (optional, same pattern as
    `Punchlist`)
- Template variables used: `{{.Repo}}`, `{{.IssueNumber}}`, `{{.IssueTitle}}`,
  `{{.PRNumber}}`, `{{.VerifyErrors}}`, `{{.BuildCommand}}`,
  `{{.TestCommand}}`, `{{.ProtectedPaths}}`
- The prompt should:
  - Show each failing check name and its truncated output
  - Instruct the agent to fix the failures and push to the existing branch
  - Remind the agent not to modify protected paths
  - Not ask the agent to re-run the checks (dark-factory handles re-verification)

### Acceptance criteria

- [ ] `prompts/verify_fix.txt` exists and is embedded
- [ ] `PromptData` has `VerifyErrors` field
- [ ] `Prompts` struct has `VerifyFix` field loaded in `LoadPrompts`
- [ ] Template renders with verify error data

### Test cases

- **Template renders**: `RenderPrompt` with `VerifyErrors` set produces output
  containing the error text
- **Empty verify errors**: Template renders without errors when `VerifyErrors`
  is empty
- **Load from config path**: Custom `verify_fix` path in config loads the
  file at that path
- **Load embedded default**: No config override loads embedded `verify_fix.txt`

---

## Issue 199: Wire verify step into agent loop (host mode)

**Blocked by**: #176, #177

### Description

Insert the deterministic verify step into `ProcessIssue` between the guard
rails (step 3) and the quality review gate (step 4). After the implementer
finishes and guard rails pass, the orchestrator builds the check list from
config and runs `RunVerify` with a host-mode `CommandRunner`.

This issue covers only the host-mode runner (`sh -c <command>` via
`GuardRunner`) and the basic pass/skip paths. The fix cycle and sandbox
runner are handled in follow-on issues.

### Key constraints

- Modify `internal/agent/loop.go`:
  - After step 3 (guard rails), insert new step 3.5: verify
  - Build `[]Check` from `cfg.BuildCommand`, `cfg.LintCommand`,
    `cfg.TestCommand` (skip empty)
  - If no checks configured, skip verify entirely
  - Create a host-mode `CommandRunner` that runs `sh -c <command>` via
    `GuardRunner` in the checked-out PR branch
  - Call `RunVerify` with the checks and runner
  - If `AllPassed`: log success, proceed to review
  - If not `AllPassed`:
    - If `cfg.Verify.Blocking`: fail the issue with verify error summary
    - If `!cfg.Verify.Blocking`: log warning, proceed to review
  - No fix cycle in this issue — that is handled by issue 183

### Acceptance criteria

- [ ] Verify step runs between guard rails and quality review
- [ ] Empty commands are skipped; no checks configured skips verify entirely
- [ ] Host-mode runner executes `sh -c <command>` via `GuardRunner`
- [ ] `Blocking: true` fails the issue when verify fails
- [ ] `Blocking: false` warns and proceeds to review

### Test cases

- **All checks pass**: Verify succeeds, proceeds directly to review
- **No commands configured**: Verify step is skipped entirely
- **Host mode verify**: Verify commands run on host via `sh -c`
- **Blocking failure**: Verify fails with `Blocking: true`, issue status is
  "failed"
- **Non-blocking failure**: Verify fails with `Blocking: false`, proceeds to
  review with warning
- **Verify runs between guard rails and quality review**: Guard rails run
  before verify, quality review runs after verify passes

---

## Issue 200: Verify fix cycle

**Blocked by**: #178, #199

### Description

Add the fix-retry loop to the verify step. When verify fails and a fix
prompt is configured, the failure summary is fed back to the implementer
agent for a fix attempt (reusing the session ID). After fixing, verify
re-runs. This repeats up to `MaxFixAttempts` times.

### Key constraints

- Modify `internal/agent/implementer.go`:
  - Add `VerifyFix` function (similar to `Retry`) that renders the
    `verify_fix.txt` prompt with `VerifyErrors` and invokes `Run` with
    role `implementer_retry`
  - Accept `prevSessionID` for session continuity
- Modify `internal/agent/loop.go`:
  - Replace the direct fail/warn on verify failure with the fix cycle:
    - Format `VerifyErrors` from the `VerifyResult` (check name + output)
    - Render `verify_fix.txt` with the errors
    - Call `VerifyFix` with the fix prompt (resume session)
    - Re-run verify
    - Loop up to `cfg.Verify.MaxFixAttempts` times
  - If verify still fails after fix attempts:
    - If `cfg.Verify.Blocking`: fail the issue with verify error
    - If `!cfg.Verify.Blocking`: log warning, proceed to review
  - Re-check protected path drift after each fix attempt

### Acceptance criteria

- [ ] Fix cycle invokes implementer with verify errors and resumes session
- [ ] Fix cycle respects `MaxFixAttempts` limit
- [ ] `Blocking: true` fails the issue when verify exhausts fix attempts
- [ ] `Blocking: false` warns and proceeds to review after exhausted attempts
- [ ] Protected path drift is re-checked after each fix attempt
- [ ] Session ID is forwarded for context resumption

### Test cases

- **Build fails, fix succeeds**: First verify fails, fix attempt passes
  verify, proceeds to review
- **Exhausted fix attempts (blocking)**: Verify fails, fixes fail, issue
  status is "failed"
- **Exhausted fix attempts (non-blocking)**: Verify fails, fixes fail,
  proceeds to review with warning
- **Drift check after fix**: Protected path drift is re-checked after each
  fix attempt
- **Session continuity**: Fix agent receives previous session ID for context
  resumption
- **Fix prompt contains error output**: Rendered prompt includes check names
  and truncated output from the failed verify

---

## Issue 201: Sandbox-mode verify runner

**Blocked by**: #199

### Description

Add a sandbox `CommandRunner` variant for the verify step. When
`cfg.NoSandbox` is false, verify commands run inside a container using
`sandbox.RunContainer` with the same Docker image. The container clones
the repo, checks out the PR branch, and runs the verify command.

### Key constraints

- New function in `internal/agent/verify.go` (or `loop.go`):
  - `sandboxCommandRunner(ctx, image, repo, branch, logger)` returning a
    `CommandRunner` that:
    - Calls `sandbox.RunContainer` with the verify command
    - Uses the same Docker image as agent containers (`cfg.Docker.Image`)
    - Container script: `git clone <repo> /workspace && cd /workspace &&
      git checkout <branch> && sh -c <command>`
    - Returns stdout, stderr, exit code from the container result
- Modify `internal/agent/loop.go`:
  - When building the `CommandRunner`, check `cfg.NoSandbox`:
    - `true`: use the existing host-mode runner (from issue 181)
    - `false`: use the new sandbox runner
  - Pass `prBranch` and `cfg.Docker.Image` to the sandbox runner

### Acceptance criteria

- [ ] Sandbox runner executes verify commands inside a container
- [ ] Container uses the same image as agent containers
- [ ] Container clones the repo and checks out the PR branch
- [ ] Stdout, stderr, and exit code are captured correctly
- [ ] `cfg.NoSandbox` selects between host and sandbox runners

### Test cases

- **Sandbox mode verify**: Verify commands run inside container with correct
  image and branch
- **Host mode unchanged**: `NoSandbox: true` still uses the host runner
- **Container failure**: Non-zero exit code from container is captured as
  check failure
- **Container cleanup**: Container is removed after verify completes

---

## Issue 179: Bash deny-list in agent runner

### Description

Add a configurable deny-list of destructive shell commands that the
`PreToolUse` hook blocks before the agent can execute them. The deny-list
is passed to the agent runner via a `GODARK_DENIED_COMMANDS` environment
variable (comma-separated patterns). When a Bash tool call matches a denied
pattern, the hook blocks it and returns a system message explaining why.

### Key constraints

- Modify `internal/config/config.go`:
  - Add `DeniedCommands []string` field with yaml tag `denied_commands`
  - Default: `["rm -rf", "git push --force", "git push -f",
    "git reset --hard", "git clean -f"]`
- Modify `internal/agent/implementer.go`:
  - In `newRunOpts`, add `GODARK_DENIED_COMMANDS` to env (comma-separated
    from `cfg.DeniedCommands`)
- Modify `internal/agent/runner/agent_runner.py`:
  - Read `GODARK_DENIED_COMMANDS` from environment
  - New `make_denied_commands_hook(denied_patterns)` function returning an
    async `PreToolUse` hook
  - Hook checks Bash tool's `command` input against each denied pattern
    using substring match (same approach as protected path heuristic)
  - On match: return `{"decision": "block", "systemMessage": "..."}` with
    explanation of which pattern matched and why the command is blocked
  - Register hook in the `PreToolUse` hooks list alongside the existing
    protected path hook, matching on `"Bash"`

### Acceptance criteria

- [ ] Config has `DeniedCommands` with sensible defaults
- [ ] `GODARK_DENIED_COMMANDS` is passed to agent runner via env
- [ ] Agent runner blocks Bash commands matching denied patterns
- [ ] Blocked commands receive a system message explaining the denial
- [ ] Non-matching Bash commands are allowed through

### Test cases

- **Config defaults**: Default denied commands include `rm -rf`,
  `git push --force`, `git reset --hard`
- **Config override**: Setting `denied_commands: ["rm -rf"]` in YAML replaces
  the defaults
- **Command blocked**: Bash command `rm -rf /workspace` matches `rm -rf` and
  is blocked
- **Command allowed**: Bash command `go test ./...` does not match any pattern
  and is allowed
- **System message**: Blocked command response includes the matching pattern
- **Empty deny-list**: Setting `denied_commands: []` disables the hook entirely

---

## Issue 182: Verify step run data integration

**Blocked by**: #199, #200

### Description

Extend the run data system to record verify step results alongside existing
agent step telemetry. Each verify attempt (initial + fix retries) is written
to the per-issue run data directory. The `RunDataHook` interface is extended
with a new method for verify results.

### Key constraints

- Modify `internal/rundata/writer.go`:
  - New type:
    ```go
    type VerifyStepResult struct {
        Attempt     int            `json:"attempt"` // 0-indexed
        Checks      []CheckResult  `json:"checks"`
        AllPassed   bool           `json:"all_passed"`
        FixAttempted bool          `json:"fix_attempted"`
    }

    type CheckResult struct {
        Name     string `json:"name"`
        Passed   bool   `json:"passed"`
        Output   string `json:"output,omitempty"`
        ExitCode int    `json:"exit_code"`
    }
    ```
  - `WriteVerifyResult(issueNum int, step VerifyStepResult) error` — writes
    to `issues/<issueNum>/verify-<attempt>.json`
- Modify `internal/agent/runhook.go`:
  - Add `WriteVerifyResult(issueNumber int, step rundata.VerifyStepResult) error`
    to `RunDataHook` interface
- Modify `internal/agent/loop.go`:
  - After each verify attempt, call `hook.WriteVerifyResult` (nil-safe)
- Modify `internal/rundata/reader.go`:
  - Add `VerifyResults []VerifyStepResult` field to `IssueDetail`
  - Read `verify-*.json` files in `ReadIssueDetail`

### Acceptance criteria

- [ ] `VerifyStepResult` and `CheckResult` types defined in rundata
- [ ] `WriteVerifyResult` writes to correct path
- [ ] `RunDataHook` interface includes `WriteVerifyResult`
- [ ] Loop calls hook after each verify attempt
- [ ] Reader loads verify results into `IssueDetail`

### Test cases

- **Write verify result**: `WriteVerifyResult(42, step)` creates
  `issues/42/verify-0.json`
- **Multiple attempts**: Two verify attempts create `verify-0.json` and
  `verify-1.json`
- **Read verify results**: `ReadIssueDetail` returns verify results when files
  exist
- **Missing verify files**: `ReadIssueDetail` returns nil verify results when
  no files exist (backwards compatible)
- **Hook called**: Mock hook verifies `WriteVerifyResult` called after verify
  step in loop
