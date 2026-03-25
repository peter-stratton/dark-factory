# Phase 27: Agent Efficiency & Resilience

> **Goal:** Every agent step completes within its time budget, produces useful
> output even on timeout, and never wastes time on tools it can't access. Recon
> adapts its depth to issue complexity and codebase size. Prompts are audited
> for tool/permission alignment across all roles.

## Milestone

`Phase 27: Agent Efficiency & Resilience`

---

## Issue: Multi-pass recon prompt with generalized language

### Description

Restructure the recon prompt (`prompts/recon.txt`) into prioritized passes so
the agent naturally front-loads the most valuable work. Partial output from any
pass is useful on its own. Also remove Flutter/UI-specific language (list
screen, form screen, app shell, router, provider/DI) in favor of universal
patterns that work across Go, Flutter, Node, Rust, and Python projects.

### Key constraints

- Modify `prompts/recon.txt` — restructure into clearly delineated priority
  sections:
  - **Priority 1 — File list and drift detection (~30s):** List which files
    need to change, check architecture layer membership, flag anything that
    differs from the issue description (renamed functions, missing types,
    already-completed work)
  - **Priority 2 — Key signatures and interfaces (~1-2m):** Read the specific
    functions/types/interfaces the implementer will modify. Quote signatures
    and critical sections, not full files
  - **Priority 3 — Pattern example (~1-2m):** Find and quote one complete
    example of the same type of artifact being built (e.g., if adding a new
    agent function, quote an existing one like `recon.go`)
- Each priority section should instruct the agent to write its findings for
  that section before proceeding to the next — this ensures partial output is
  always useful
- Replace UI-specific terminology:
  - "list screen" / "form screen" → "the closest existing implementation of
    the same pattern"
  - "app shell" / "router" / "nav structure" → "entry point wiring"
  - "provider / DI setup" → "dependency wiring or initialization"
- Keep the instruction to include verbatim code, but scope it: "quote the
  relevant function/type, not the entire file"
- The `PromptData` struct does not need changes — recon.txt already has access
  to all fields it needs

### Acceptance criteria

- [ ] Recon prompt has 3 clearly delineated priority sections
- [ ] Each section instructs the agent to write findings before proceeding
- [ ] No Flutter/UI-specific language remains in the prompt
- [ ] Prompt renders without error with standard `PromptData`
- [ ] `go test ./internal/agent/` passes

### Test cases

- **Prompt renders**: `RenderPrompt` with standard `PromptData` produces valid
  output containing all three priority section headers
- **No UI-specific language**: Rendered prompt does not contain "list screen",
  "form screen", "app shell", "router", or "nav structure"
- **Issue context present**: Rendered prompt contains the issue title and body

---

## Issue: Partial recon brief on timeout

### Description

When the recon agent times out, capture whatever partial output was produced
and pass it to the implementer instead of discarding all work. The agent
runner streams assistant text blocks to stdout as they arrive, so partial
content is available in the container logs even on timeout.

Relates to #630.

### Key constraints

- Modify `handleNonBlockingResult()` in `internal/agent/loop.go` (line ~464):
  - Currently returns `""` when `result.TimedOut` is true
  - Change to return `result.ResultText` when it's non-empty, even on timeout
  - Prefix the partial brief with a note: `"[Recon timed out — partial brief
    follows]\n\n"` so the implementer knows it may be incomplete
- The `Result.ResultText` field is populated from the runner's
  `full_output = "\n\n".join(assistant_texts)` — this accumulates all
  `TextBlock` content streamed before the timeout
- On timeout, the container is killed and `docker logs` captures everything
  printed before the kill signal. The Go-side `parseRunnerOutput` extracts
  the last valid JSON line as the result. If no final result JSON exists
  (agent was mid-generation), `ResultText` will be empty — in that case,
  fall back to extracting raw assistant text from `Result.Stdout`
- Write the partial brief to run data (recon.json) with an `error: "timed out
  (partial brief)"` field so the dashboard shows it was partial
- The `handleNonBlockingResult` function is shared by recon, spec generator,
  and verify — the partial output behavior should apply to all of them

### Acceptance criteria

- [ ] Recon timeout with partial output passes the partial brief to implementer
- [ ] Partial brief is prefixed with timeout notice
- [ ] Recon timeout with no output returns empty string (current behavior)
- [ ] Partial brief written to run data with timeout error annotation
- [ ] Spec generator and verify also benefit from partial output on timeout

### Test cases

- **Partial output on timeout**: Create a `Result` with `TimedOut: true` and
  `ResultText: "## Files\n- config.go"`. Verify `handleNonBlockingResult`
  returns the text with timeout prefix
- **Empty output on timeout**: Create a `Result` with `TimedOut: true` and
  empty `ResultText`. Verify function returns `""`
- **Normal completion unchanged**: Create a `Result` with `TimedOut: false`.
  Verify function returns `ResultText` without prefix
- **Run data includes partial**: Verify the `StepResult` written via hook
  contains both the partial output and the timeout error annotation

---

## Issue: Adaptive recon depth by issue type

**Blocked by**: multi-pass recon prompt with generalized language

### Description

Add a mechanism to select recon depth based on issue characteristics. Wiring
and refactor issues need lightweight recon (file list + drift only), while
feature issues benefit from the full multi-pass recon. Issues with detailed
key constraints in the body may skip recon entirely.

### Key constraints

- Add `ReconDepth string` field to `PromptData` in `internal/agent/prompt.go`
  (line ~78) — values: `"full"` (default), `"light"`, `"skip"`
- Modify `newPromptData()` in `internal/agent/prompt.go` to default
  `ReconDepth` to `"full"`
- Add depth detection heuristic in `internal/agent/loop.go` before the recon
  call (line ~119):
  - `"skip"`: issue body contains >3 verbatim code blocks (fenced with
    triple backticks) — the planning doc already did the recon work
  - `"light"`: issue title contains "wire", "wiring", "refactor", "rename",
    "migrate", "update callers", or "cleanup" (case-insensitive)
  - `"full"`: default for everything else
  - Configurable override: add `recon_depth` field to `godark.yaml` config
    (`"full"`, `"light"`, `"skip"`, `"auto"` default)
- Update `prompts/recon.txt` to use `{{.ReconDepth}}` conditional:
  - `"light"`: only execute Priority 1 (file list + drift), skip Priorities
    2 and 3
  - `"full"`: execute all three priorities
- When depth is `"skip"`, bypass the recon call entirely in loop.go (same as
  when `prompts.Recon` is empty)
- Log the detected depth in the recon start message for diagnostics

### Acceptance criteria

- [ ] `ReconDepth` field exists on `PromptData`
- [ ] Wiring issues auto-detect as `"light"` depth
- [ ] Issues with verbose code blocks auto-detect as `"skip"`
- [ ] `recon_depth: light` in config forces light recon for all issues
- [ ] `recon_depth: auto` (default) uses heuristic detection
- [ ] Light recon produces only Priority 1 output
- [ ] Skipped recon logs the skip reason

### Test cases

- **Auto-detect wiring**: Issue title "Wire Labels struct into callers" →
  depth `"light"`
- **Auto-detect skip**: Issue body with 4 fenced code blocks → depth `"skip"`
- **Auto-detect full**: Issue title "Add expense repository" → depth `"full"`
- **Config override**: `recon_depth: light` in config → always `"light"`
  regardless of issue content
- **Light prompt renders**: Render recon.txt with `ReconDepth: "light"` →
  output contains Priority 1 section, does not contain Priority 2/3
- **Full prompt renders**: Render recon.txt with `ReconDepth: "full"` →
  output contains all three priority sections

---

## Issue: Per-step timeout configuration

### Description

Allow per-role timeout overrides so that fast agents (spec generator, recon)
don't waste 30 minutes on timeout when they should complete in under 5. The
current `agent_timeout` applies uniformly to all roles.

### Key constraints

- Add `Timeouts` struct to `internal/config/config.go`:
  ```go
  type Timeouts struct {
      Default       string `yaml:"default"`        // e.g. "30m", fallback
      SpecGenerator string `yaml:"spec_generator"`  // e.g. "3m"
      Recon         string `yaml:"recon"`           // e.g. "5m"
      Implementer   string `yaml:"implementer"`     // e.g. "30m"
      Reviewer      string `yaml:"reviewer"`        // e.g. "15m"
      VerifyFix     string `yaml:"verify_fix"`      // e.g. "10m"
  }
  ```
- Add `Timeouts Timeouts` field to `Config` struct with yaml tag `timeouts`
- Modify `newRunOpts()` in `internal/agent/prompt.go` to accept the role name
  and resolve timeout from the `Timeouts` struct:
  - Check role-specific field first, fall back to `Timeouts.Default`, then
    fall back to `AgentTimeout`, then fall back to 30m
- Preserve backwards compatibility: existing `agent_timeout` field still works
  as a global default. `timeouts:` block overrides it per-role
- Suggested defaults (not hardcoded — just documentation):
  - spec_generator: 3m
  - recon: 5m
  - implementer: 30m
  - reviewer: 15m
  - verify_fix: 10m

### Acceptance criteria

- [ ] `Timeouts` struct exists in config with per-role fields
- [ ] `newRunOpts` resolves timeout by role → default → agent_timeout → 30m
- [ ] Existing `agent_timeout` config still works as global fallback
- [ ] `go test ./internal/config/` passes
- [ ] `go test ./internal/agent/` passes

### Test cases

- **Role-specific timeout**: Config with `timeouts.recon: 5m` → recon gets 5m
- **Default fallback**: Config with `timeouts.default: 20m`, no role-specific →
  all roles get 20m
- **AgentTimeout fallback**: Config with only `agent_timeout: 25m` → all roles
  get 25m (backwards compat)
- **Cascade**: Config with `agent_timeout: 30m`, `timeouts.default: 20m`,
  `timeouts.recon: 5m` → recon gets 5m, implementer gets 20m
- **Empty config**: No timeout fields → all roles get 30m default

---

## Issue: Prompt and permission audit

### Description

Systematic scan of all prompt templates against their role permissions to
identify and fix any remaining instructions for unavailable tools. Also
evaluate whether "Read CLAUDE.md" instructions in prompts are valuable or
wasteful.

### Key constraints

- Audit each prompt file in `prompts/` against its role in `_ROLE_PERMISSIONS`:
  - `implementer.txt` (role: implementer) — has all tools. Check if "Read
    CLAUDE.md completely before doing anything else" is worth the context cost.
    The implementer already gets architecture/conventions via template
    variables. Consider removing or changing to "Read CLAUDE.md if it exists"
  - `implementer_retry.txt` (role: implementer_retry) — has all tools. Verify
    no issues
  - `reviewer.txt` (role: reviewer) — no Write/Edit. Already fixed (#626).
    Verify fix is complete
  - `quality_reviewer.txt` (role: quality_reviewer) — no Write/Edit. Check
    for any Write/Edit references
  - `spec_generator.txt` (role: spec_generator) — no Bash. Already fixed
    (#620). Verify fix is complete
  - `punchlist.txt` (role: punchlist) — no Write/Edit/Bash. Check for any
    references
  - `recon.txt` (role: recon) — no Write/Edit/Bash. Check for any references
  - `verify_fix.txt` (role: implementer — note: uses implementer role, not a
    separate verify role). Verify no issues
- For each mismatch found: fix the prompt to use available tools or remove the
  instruction
- Document the audit results in the PR description for future reference

### Acceptance criteria

- [ ] All 8 prompt files audited against their role permissions
- [ ] No prompt instructs the agent to use a tool it doesn't have access to
- [ ] "Read CLAUDE.md" instruction evaluated and either kept with justification
  or removed
- [ ] `go test ./internal/agent/` passes

### Test cases

- **No disallowed tool references**: For each prompt, render with standard
  PromptData and verify the output does not contain instructions to use tools
  not in the role's `allowed_tools`
- **Prompts render cleanly**: All 8 prompts render without template errors
