# Phase 29: Complete CLI Migration

> **Goal:** Remove all remnants of the Python SDK runner path. The only way to
> run agents is via the Claude CLI inside Docker containers. No `--no-sandbox`,
> no `runHost`, no `agent_runner.py`. Simplifies the codebase and eliminates a
> dead code path that would fail at runtime.

## Milestone

`Phase 29: Complete CLI Migration`

---

## Issue 668: Delete Python runner package and runHost function

### Description

Remove the dead host-mode execution path. Delete the `internal/agent/runner/`
package (agent_runner.py, embed.go, embed_test.go, test_hooks.py, __pycache__),
the `runHost()` function, and all host-specific helpers in launcher.go (`Runner`
var, `writerFunc` type, `goosForRusage` var). Simplify `Run()` to always call
`runSandbox()` — remove the `noSandbox bool` parameter. Update all 6 agent
caller files to drop the removed parameter.

### Key constraints

- Delete `internal/agent/runner/` directory entirely
- In `internal/agent/launcher.go`:
  - Delete `Runner` var (lines 69-98), `goosForRusage` var (line 67),
    `writerFunc` type and method (lines 100-108)
  - Delete `runHost()` function (lines 137-231)
  - Change `Run()` signature from
    `Run(ctx, opts, noSandbox bool, logger)` to `Run(ctx, opts, logger)`
  - Body becomes: `return runSandbox(ctx, opts, logger)`
  - Remove imports: `os/exec`, `runtime`, `syscall`, `runner` package
    (verify each is not used by remaining code before removing)
- Update 8 call sites across 6 files — remove `cfg.NoSandbox` argument:
  - `internal/agent/implementer.go` (3 calls: lines 47, 92, 125)
  - `internal/agent/punchlist.go` (line 75)
  - `internal/agent/quality_reviewer.go` (line 43)
  - `internal/agent/recon.go` (line 34)
  - `internal/agent/reviewer.go` (line 39)
  - `internal/agent/specgen.go` (line 38)
- Delete `internal/agent/launcher_test.go` entirely — all 22 tests
  (`TestRun_NoSandbox_*`) test the removed `runHost()` path
- Remove `runner` from `infrastructure` layer paths in `docs/architecture.json`

### Acceptance criteria

- [ ] `internal/agent/runner/` directory does not exist
- [ ] `runHost` function does not exist in launcher.go
- [ ] `Run()` has no `noSandbox` parameter
- [ ] No Go file imports `internal/agent/runner`
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes

### Test cases

- **Build passes**: `go build ./...` completes with no errors after deletion
- **No dangling imports**: `grep -r "agent/runner" --include="*.go"` returns
  no matches
- **Run always sandboxes**: `Run()` calls `runSandbox()` unconditionally

---

## Issue 670: Remove NoSandbox from config, commands, orchestrator, and doctor

**Blocked by**: #668

### Description

Remove the `NoSandbox` field from the Config struct, the `--no-sandbox` CLI
flag from all commands, and all conditional branches guarded by `NoSandbox`
across the orchestrator, commands, loop, verify, and doctor. After this change,
Docker image builds and sandbox verify runners are unconditional.

### Key constraints

- `internal/config/config.go`:
  - Remove `NoSandbox bool` from `Config` struct (line 237)
  - Remove `NoSandbox *bool` from `CLIFlags` struct (line 364)
  - Remove flag override in `applyFlags()` (lines 586-587)
- `internal/cmd/cmdutil.go`:
  - Remove `no-sandbox` flag parsing block (lines 24-27)
- `internal/cmd/run.go`:
  - Remove `--no-sandbox` flag registration (line 293)
  - Remove `if cfg.NoSandbox` warning block (line 65)
- `internal/cmd/implement.go`:
  - Remove `--no-sandbox` flag registration (line 539)
  - Remove `if cfg.NoSandbox` warning block (line 87)
  - Remove `if !cfg.NoSandbox` Docker build guard (line 158) — make
    `BuildImage` unconditional
  - Remove `if cfg.NoSandbox` PullAfterMerge block (line 355) — delete
    entirely, not make unconditional
- `internal/orchestrator/orchestrator.go`:
  - Remove `if !cfg.NoSandbox` guard around `BuildImage` (line 358) — make
    unconditional
  - Remove `if !cfg.NoSandbox` guard around `BuildImage` (line 637) — make
    unconditional
  - Remove `if cfg.NoSandbox { PullAfterMerge(...) }` block (line 798) —
    delete entirely (sandbox gets fresh clone, no local sync needed)
- `internal/agent/loop.go`:
  - Remove `if cfg.NoSandbox` blocks (lines 305-308, 407-410) — always use
    `sandboxCommandRunner()`
  - Delete `newHostRunner()` function (lines 1107-1119)
- `internal/agent/verify.go`:
  - Remove `if cfg.NoSandbox` block (line 174) — always use
    `sandboxCommandRunner()`
- `internal/doctor/doctor.go`:
  - Remove `NoSandbox` from `Opts` struct (line 65)
  - Remove `if !opts.NoSandbox` guard around Docker check (line 79) — make
    Docker check unconditional
  - Remove `if opts.NoSandbox` block for host toolchain checks (line 118) —
    delete entirely
- `internal/cmd/doctor.go`:
  - Remove `--no-sandbox` flag (line 61)
  - Remove `noSandbox` local var and `NoSandbox` in opts (lines 30, 50)

### Acceptance criteria

- [ ] `NoSandbox` field does not exist in Config or CLIFlags structs
- [ ] `--no-sandbox` flag not registered on any command
- [ ] No `if cfg.NoSandbox` or `if !cfg.NoSandbox` blocks remain
- [ ] `newHostRunner()` function does not exist
- [ ] `PullAfterMerge` is not called from the main wave loop
- [ ] Docker check is always included in doctor output
- [ ] `go build ./...` passes
- [ ] `go test ./internal/config/... ./internal/cmd/... ./internal/doctor/...` passes

### Test cases

- **Config without NoSandbox**: Parse YAML with `no_sandbox: true` — field is
  silently ignored (yaml.v3 non-strict mode)
- **BuildImage always called**: Orchestrator test without NoSandbox still calls
  BuildImage (verify via stub)
- **Doctor always checks Docker**: Doctor run includes "Docker daemon running"
  check regardless of flags
- **No host toolchain checks**: Doctor does not include runtime/golangci-lint
  checks (those were host-only)

---

## Issue 671: Migrate test infrastructure from Runner to SandboxRunner stubs

**Blocked by**: #670

### Description

Update all test functions that relied on `NoSandbox: true` and/or `Runner`
stubs to work with the sandbox-only code path. The orchestrator tests set
`NoSandbox: true` to skip `BuildImage` — after removal, they need to stub
`BuildImage` instead (or verify it's already stubbed at the right level). The
loop/agent tests that stub `Runner` need to stub `SandboxRunner` or the
underlying `sandbox.SplitRunner` instead.

### Key constraints

- `internal/agent/loop_test.go`:
  - Remove `NoSandbox: true` from `loopConfig()` (line 113)
  - `setupLoopTest()` currently stubs `Runner` (line 94) — it already also
    stubs `sandbox.CommandRunner`, `sandbox.CommandRunnerWithContext`, and
    `sandbox.SplitRunner` (lines 42-59). Verify the sandbox path works with
    the existing sandbox stubs and remove the `Runner` stub
  - Remove `NoSandbox: true` from `sandboxLoopConfig()` and
    `verifyLoopConfig()` if present
  - Delete `TestProcessIssue_VerifyHostModeUnchangedWithNoSandbox` and
    `TestHostRunner_*` tests that test the removed distinction
- `internal/agent/helpers_test.go`:
  - Remove `NoSandbox: true` from `testConfig()` (line 94)
- `internal/agent/punchlist_test.go`:
  - Remove `NoSandbox: true` from test configs (4 occurrences)
- `internal/agent/rebase_test.go`:
  - Remove `NoSandbox: true` from test config (line 19)
- `internal/orchestrator/orchestrator_test.go`:
  - Remove all `cfg.NoSandbox = true` lines (~31 occurrences)
  - Verify tests still pass — they stub `processIssueFn` so they never reach
    `Run()` or `BuildImage`. If any test calls through to `BuildImage`,
    add a `sandbox.CommandRunner` stub
- `internal/config/config_test.go`:
  - Delete `TestNoSandboxDefault`, `TestNoSandboxFromYAML`,
    `TestNoSandboxFlagOverride` test functions
- `internal/cmd/run_test.go`:
  - Delete `TestNoSandboxFlagParsing`, `TestNoSandboxConfigFile`,
    `TestNoSandboxDefaultFalse`, `TestNoSandboxFlagOverridesConfig`,
    `TestNoSandboxWarning`
- `internal/cmd/root_test.go`:
  - Remove `"no-sandbox"` from expected flag list
- `internal/cmd/cmdutil_test.go`:
  - Remove `"no-sandbox"` from flag registration and assertions
- `internal/doctor/doctor_test.go`:
  - Delete all `TestRun_NoSandbox_*` and `TestChecks_NoSandbox_*` functions
    (~10 functions)

### Acceptance criteria

- [ ] No test file references `NoSandbox`
- [ ] No test file stubs `Runner` (the host-mode var)
- [ ] `go test ./...` passes with no failures
- [ ] `go test ./... -race` passes

### Test cases

- **Full suite passes**: `go test ./...` exits 0
- **Race clean**: `go test ./... -race` exits 0
- **No stale references**: `grep -r "NoSandbox\|Runner =" --include="*_test.go"`
  returns no matches (excluding `SandboxRunner`)

---

## Issue 669: Update docs and config schemas for sandbox-only mode

### Description

Remove `no_sandbox` from the godark.md config reference, the JSON schema used
by the configure-project skill, and any references in the installed CLAUDE.md
template. This is content-layer only — no Go code changes.

### Key constraints

- `internal/harness/templates/godark.md`:
  - Remove the `no_sandbox` row from the "Agent behavior" config table (line 54)
- `.claude/godark.md`:
  - Same removal (installed copy)
- `internal/skills/godark-configure-project/godark-config-schema.json`:
  - Remove `no_sandbox` property (lines 100-103)
- `.claude/skills/godark-configure-project/godark-config-schema.json`:
  - Same removal (installed copy)

### Acceptance criteria

- [ ] `no_sandbox` does not appear in godark.md template
- [ ] `no_sandbox` does not appear in config schema JSON
- [ ] `go test ./internal/skills/...` passes (embedded content tests)

### Test cases

- **Schema valid**: JSON schema parses without errors after removal
- **Skills test passes**: `go test ./internal/skills/...` exits 0
- **No stale references**: `grep -r "no_sandbox" internal/harness/ internal/skills/ .claude/` returns no matches

---

## Integration chain audit

The removal is purely destructive — no new types, fields, or functions are
introduced. The chain to verify is that every reference to the removed symbols
is covered by an issue:

```
config.NoSandbox defined in config.go
  → read by applyFlags() in config.go               ← Issue 2
  → read by Run() param in launcher.go               ← Issue 1
  → read by loop.go verify runner selection           ← Issue 2
  → read by verify.go verify runner selection         ← Issue 2
  → read by orchestrator.go BuildImage guards         ← Issue 2
  → read by orchestrator.go PullAfterMerge guard      ← Issue 2
  → read by run.go/implement.go warning + flags       ← Issue 2
  → read by doctor.go Opts + checks                   ← Issue 2
  → set in ~47 test functions                         ← Issue 3
  → documented in godark.md + schema                  ← Issue 4

Runner var defined in launcher.go
  → called by runHost() in launcher.go                ← Issue 1 (both deleted)
  → stubbed in loop_test.go setupLoopTest()           ← Issue 3
  → stubbed in launcher_test.go                       ← Issue 1 (file deleted)

runner.FS (embed) used by runHost() in launcher.go    ← Issue 1 (both deleted)

newHostRunner() defined in loop.go
  → called by loop.go verify selection                ← Issue 2
  → called by verify.go verify selection              ← Issue 2

PullAfterMerge() called from orchestrator.go
  → guarded by NoSandbox                             ← Issue 2

architecture.json references runner/
  → paths entry for infrastructure layer              ← Issue 1
```

All hops are covered. No gaps.
