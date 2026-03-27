## Phase 13: Human-in-the-Loop Review ✅

**Goal**: Humans can review godark-created PRs and request changes that the
agent automatically picks up and fixes. Teams adopt godark with full human
oversight and gradually increase autonomy as trust builds. This is the
critical path for org adoption — most teams will not start with auto-merge.

**Milestone**: `Phase 13` | **Label**: `phase-13`

### PR lifecycle state machine
- Each godark PR tracks state: `ai_review` → `awaiting_human` →
  `human_changes_requested` → `ai_fix` → `awaiting_human` (loop until
  approved or max cycles exceeded)
- State communicated via PR labels: `godark:awaiting-human-review`,
  `godark:fixing-review-feedback`, `godark:ready-to-merge`
- Labels are the source of truth — any external tooling or human can read
  the current state at a glance

### Feedback listener
- `godark watch` subcommand — polls for `CHANGES_REQUESTED` GitHub reviews
  and new review comments on godark-labeled PRs
- Configurable poll interval (default 60s)
- Filters to own PRs only (created by the configured GitHub user/app)
- Webhook mode as a future optimization (polling is simpler to deploy and
  sufficient for most orgs)

### Session resumption with human feedback
- When a human requests changes, feed their review comments into the
  implementer agent, resuming its prior session (`GODARK_SESSION_ID`)
- Agent has full context: original implementation reasoning, AI reviewer
  feedback from prior rounds, and now the human's feedback
- Human comments are treated the same as AI reviewer comments — the
  implementer sees a unified feedback stream
- After fixing, the agent pushes and re-labels the PR as
  `godark:awaiting-human-review`

### Graduated autonomy
- `auto_merge` config in `godark.yaml` controls merge behavior per-repo:
  ```yaml
  auto_merge: none       # default — stop at PR, human merges
  auto_merge: low_risk   # auto-merge small/safe PRs, stop for rest
  auto_merge: all        # human spot-checks only
  ```
- Risk classification for `low_risk` mode:
  - Lines changed threshold (configurable, e.g. < 200 lines)
  - No changes to protected paths, CI/CD configs, or dependency files
  - All verify checks passed on first attempt (no fix cycles)
  - No quality flags raised
- Risk assessment written to run data so humans can audit the classification

### Dashboard integration
- PRs awaiting human review surfaced prominently in run detail view
- Filter/sort by `awaiting_human` state across all runs
- Human feedback rounds visible in the issue detail dialogue timeline

### Notifications
- Pluggable notification provider model (`Notifier` interface) supporting
  multiple channels (Telegram at launch, extensible to Slack, email, etc.)
- Events: `run_complete`, `implementation_complete`, `abort`
- Provider-specific settings use `${VAR}` environment variable expansion
  for secrets
- Best-effort delivery — notification failures are logged, never block
  execution

### Config
```yaml
auto_merge: none  # none | low_risk | all
watch:
  poll_interval: 60s
risk_thresholds:
  max_lines: 200
  max_files: 10
notify:
  - provider: telegram
    events: [run_complete, abort]
    settings:
      bot_token: ${TELEGRAM_BOT_TOKEN}
      chat_id: "123456789"
```

**Issues**: #238–#249, #270–#272

**Planning doc**: `docs/planning/phase-13-human-in-the-loop-review.md`

