## Phase 10: Deterministic Verification Pipeline ✅

**Goal**: Agent implementation passes through a deterministic verify step
(build + lint + test) run by Go code — not by the agent — before review begins.
Failures are fed back to the implementer automatically, saving review cycles
and tokens. Agents are also restricted from running destructive shell commands.

**Milestone**: `Phase 10` | **Label**: `phase-10`

### Lint command config
- Add `lint_command` field to `godark.yaml` (empty string = skip)
- User provides any command or shell script — dark-factory runs it and checks
  the exit code, same pattern as `build_command` and `test_command`

### Go-side verify step
- New deterministic step in the agent loop between implementation and review
- Runs `build_command`, `lint_command`, and `test_command` in sequence
- Captures structured pass/fail result with summarized error output (not raw
  terminal dumps) — only the failing command's stderr/stdout, truncated to a
  reasonable length
- Runs inside the sandbox if sandboxing is enabled

### Auto-fix cycle
- On verify failure, feed the structured error summary back to the implementer
  for a fix attempt (reuses session for context continuity)
- Configurable max fix attempts before escalating to review or failing
- Verify step re-runs after each fix attempt

### Verify behavior config
- `verify:` config block controlling which checks run and failure behavior
- Option to treat verify failures as blocking (default) or warning-only
- Individual checks can be disabled (e.g. skip lint, keep build + test)

### Bash deny-list
- Deny-list for destructive commands in the `PreToolUse` hook (`rm -rf`,
  `git push --force`, `git reset --hard`, `curl | sh`, etc.)
- Configurable via `denied_commands` in `godark.yaml`
- Agent receives a system message explaining why the command was blocked

### Run data integration
- Verify step results written to run data (pass/fail per check, duration,
  fix attempt count)
- Quality flags for verify failures surfaced in dashboard

**Issues**: #176–#182

**Planning doc**: `docs/planning/phase-10-deterministic-verification-pipeline.md`

