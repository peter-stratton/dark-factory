# Phase 29: Complete CLI Migration

Phase 5 migrated agent execution from the Claude CLI to the Python Agent SDK. Phase 28 added container health monitoring on top of the sandbox path. But the old host-mode execution path (`runHost`, `--no-sandbox`, the Python runner package) lingered as dead code that would fail at runtime if anyone tried to use it. Phase 29 removes it all. The codebase drops from two execution paths to one, the test suite shrinks, and the config surface gets simpler. Pure cleanup -- no new features, just less code to maintain.

---

## Runner Package Deletion

**What it does:** Removes the `internal/agent/runner/` package entirely -- the embedded Python Agent SDK wrapper (`agent_runner.py`), its Go embed harness (`embed.go`), and test utilities. This was the Python-side execution layer that `runHost()` invoked.

**Example:** Before Phase 29, the infrastructure layer in `docs/architecture.json` included the runner:

```json
"paths": ["internal/github/", "internal/ghapp/", "internal/lock/", "internal/sandbox/", "internal/pypi/", "internal/agent/runner/", "internal/notify/"]
```

After:

```json
"paths": ["internal/github/", "internal/ghapp/", "internal/lock/", "internal/sandbox/", "internal/pypi/", "internal/notify/"]
```

Four files deleted: `agent_runner.py` (the SDK wrapper script), `embed.go` (Go embed directive), `embed_test.go`, and `test_hooks.py`. No Go file in the repository imports the package anymore.

---

## Simplified Run() Function

**What it does:** The `Run()` function in `internal/agent/launcher.go` loses its `noSandbox bool` parameter and unconditionally delegates to `runSandbox()`. The `runHost()` function, the `Runner` var, `writerFunc` type, and `goosForRusage` var are all deleted.

**Example:** Before:

```go
func Run(ctx context.Context, opts RunOpts, noSandbox bool, logger *slog.Logger) (*Result, error) {
    if noSandbox {
        return runHost(ctx, opts, logger)
    }
    return runSandbox(ctx, opts, logger)
}
```

After:

```go
// Run invokes a Claude Code agent with the given prompt inside a Docker container.
func Run(ctx context.Context, opts RunOpts, logger *slog.Logger) (*Result, error) {
    return runSandbox(ctx, opts, logger)
}
```

The comment says it all -- there's only one way to run agents now. The `RunOpts` struct stays clean with no runner-path-specific fields:

```go
type RunOpts struct {
    Prompt            string
    Role              string
    Env               map[string]string
    Image             string
    Repo              string
    Branch            string
    WorkDir           string
    Timeout           time.Duration
    MountDockerSocket bool
    JudgeConfig       *config.Judge
}
```

The only testability seam is `SandboxRunner`, which replaces the old `Runner` var:

```go
var SandboxRunner = func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
    return sandbox.RunContainer(ctx, opts, logger)
}
```

---

## Agent Caller Cleanup

**What it does:** All six agent caller files drop the `cfg.NoSandbox` argument from their `Run()` calls. The pattern becomes three clean steps: render prompt, build opts, call `Run()`.

**Example:** The implementer in `internal/agent/implementer.go`:

```go
opts, err := newRunOpts(rendered, cfg, authEnv, "implementer")
if err != nil {
    return nil, err
}

return Run(ctx, opts, logger)
```

Same pattern in `Retry()`, `VerifyFix()`, `Review()`, `QualityReview()`, `GenerateSpec()`, `Recon()`, and `GenerateAcceptanceTests()`. Eight call sites across six files, all simplified.

---

## NoSandbox Config Removal

**What it does:** Removes the `NoSandbox bool` field from the `Config` struct, the `NoSandbox *bool` from `CLIFlags`, the `--no-sandbox` flag from all Cobra commands (`run`, `implement`, `doctor`), and every `if cfg.NoSandbox` conditional branch in the codebase.

**Example:** The `godark run` and `godark implement` commands no longer accept `--no-sandbox`. Before, running without Docker would show a warning:

```
WARNING: Running without sandbox. Agent has full access to your system.
```

That code path is gone. Docker is the only way. The orchestrator's `BuildImage` call is now unconditional -- no `if !cfg.NoSandbox` guard. The `PullAfterMerge` call (which synced the host repo after a merge in host mode) is deleted entirely since sandbox containers get fresh clones.

In `internal/agent/loop.go`, the command runner selection simplified from:

```go
if cfg.NoSandbox {
    verifyRunner = newHostRunner()
} else {
    verifyRunner = sandboxCommandRunner(...)
}
```

To just:

```go
verifyRunner = sandboxCommandRunner(...)
```

The `newHostRunner()` function is deleted.

---

## Doctor Unconditional Docker Check

**What it does:** `godark doctor` now always checks that the Docker daemon is running. Previously, the check was skipped when `--no-sandbox` was set, and host toolchain checks (Go, golangci-lint) ran instead.

**Example:** Running `godark doctor` now always includes:

```
Docker daemon running .......................... OK
```

The `Opts` struct in `internal/doctor/doctor.go` no longer has a `NoSandbox` field. The Docker check starts unconditionally at line 75:

```go
func Checks(opts Opts) []*Check {
    var checks []*Check
    checks = append(checks, &Check{
        Name: "Docker daemon running",
        Fix:  "Start Docker Desktop or the Docker daemon...",
        run: func() bool {
            _, err := CommandRunner("docker", "info")
            return err == nil
        },
    })
```

Host toolchain checks (checking for Go, linters on the host machine) are deleted since agents never run on the host.

---

## Test Infrastructure Migration

**What it does:** Migrates ~40 test functions from `Runner` stubs and `NoSandbox: true` configs to `SandboxRunner` stubs. Deletes `launcher_test.go` (22 `TestRun_NoSandbox_*` tests), config tests for the removed field, doctor tests for host-mode checks, and CLI flag tests for `--no-sandbox`.

**Example:** Test configs across `loop_test.go`, `helpers_test.go`, `punchlist_test.go`, `rebase_test.go`, and `orchestrator_test.go` all used `NoSandbox: true` to avoid Docker during testing. After the migration, tests stub `SandboxRunner` instead:

```go
// Before: skip Docker entirely
cfg := &config.Config{NoSandbox: true}

// After: stub the sandbox runner
orig := agent.SandboxRunner
agent.SandboxRunner = func(ctx context.Context, opts sandbox.RunOpts, logger *slog.Logger) (*sandbox.RunResult, error) {
    return &sandbox.RunResult{Stdout: "...", ExitCode: 0}, nil
}
defer func() { agent.SandboxRunner = orig }()
```

The orchestrator tests (~31 occurrences of `cfg.NoSandbox = true`) were the largest batch. They stub `processIssueFn` at a higher level so they never reach `Run()` or `BuildImage`, making the `NoSandbox` bypass unnecessary.

---

## Docs and Schema Cleanup

**What it does:** Removes `no_sandbox` from the godark.md config reference, the JSON schema used by the configure-project skill, and any documentation that referenced host-mode execution.

**Example:** The config schema in `.claude/skills/godark-configure-project/godark-config-schema.json` previously included:

```json
"no_sandbox": {
    "type": "boolean",
    "default": false
}
```

That property is removed. The godark.md config reference table drops the `no_sandbox` row from the "Agent behavior" section. Old `godark.yaml` files with `no_sandbox: true` are silently ignored since Go's `yaml.v3` is non-strict by default.
