## Phase 29: Complete CLI Migration ✅

**Goal**: Remove all remnants of the Python SDK runner path. The only way to
run agents is via the Claude CLI inside Docker containers. No `--no-sandbox`,
no `runHost`, no `agent_runner.py`. Simplifies the codebase and eliminates a
dead code path that would fail at runtime.

**Milestone**: `Phase 29: Complete CLI Migration` | **Label**: `phase-29`

- Remove `--no-sandbox` flag, `NoSandbox` config field, and all conditional
  branches across orchestrator, commands, agent loop, and doctor
- Delete `runHost()` function, `Runner` var, `writerFunc`, and `goosForRusage`
  from launcher.go
- Delete `internal/agent/runner/` package (agent_runner.py, embed.go,
  embed_test.go, test_hooks.py)
- Migrate test infrastructure from `Runner` stubs to `SandboxRunner` stubs
  (~40 test functions in loop_test.go, helpers_test.go, punchlist_test.go,
  rebase_test.go)
- Remove `PullAfterMerge` host-mode conditionals from orchestrator and
  implement command
- Remove `NoSandbox` from doctor opts and host toolchain checks
- Update config schema, godark.md template, and flag tests

**Issues**: #668–#671

**Planning doc**: `docs/planning/phase-29-complete-cli-migration.md`

