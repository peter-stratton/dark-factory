# Phase 17: Configurable Base Branch

> **Goal:** godark supports branching off and merging into a configurable base
> branch instead of always targeting the repo's default branch. Teams that
> require peer review on merges to main can run godark autonomously on
> sub-branches under a parent feature branch, then submit the parent for human
> review.

## Milestone

`Phase 17`

---

## Issue 311: Add `base_branch` config field and CLI flag

### Description

Add a `BaseBranch` field to `Config` and a `--base-branch` flag to the
`implement` (and `run`) commands. When set, godark creates feature branches off
this branch and targets PRs against it instead of the repo's default branch.
When empty (default), behavior is unchanged - PRs target the repo's default
branch.

### Key constraints

- Modify `internal/config/config.go`:
  - Add `BaseBranch string` field with yaml tag `base_branch` to `Config`
  - Add `BaseBranch *string` to `CLIFlags`
  - Add `BaseBranch` handling in `applyFlags()`
  - Default: `""` (empty string means use repo default branch)
  - No validation needed - empty means "not configured"
- Modify `internal/cmd/implement.go`:
  - Add `--base-branch` flag
  - Wire `cmd.Flags().Changed("base-branch")` into `CLIFlags`
- Modify `internal/cmd/run.go`:
  - Same `--base-branch` flag and wiring

### Acceptance criteria

- [ ] `Config` has `BaseBranch` field, empty by default
- [ ] Setting `base_branch: feature/foo` in YAML is reflected in parsed config
- [ ] `--base-branch feature/foo` CLI flag overrides YAML value
- [ ] Existing tests pass (no regressions)

### Test cases

- **Config default**: New config has empty `BaseBranch`
- **YAML override**: Setting `base_branch: "my-feature"` in YAML is reflected
  in parsed config
- **CLI flag override**: `--base-branch` flag overrides YAML value
- **CLI flag not set**: When flag is not passed, YAML value is preserved

---

## Issue 312: Pass `--base` flag to `gh pr create`

**Blocked by**: #311

### Description

When `BaseBranch` is configured, the implementer agent needs to create PRs
targeting that branch instead of the repo default. Pass `BaseBranch` through
to the `PromptData` struct and update the implementer prompt template to
include `--base {{.BaseBranch}}` in the `gh pr create` command when set.

The agent also needs to branch off the base branch rather than whatever HEAD
happens to be. Update the branch creation instructions in the prompt to
`git fetch origin && git checkout -b <branch> origin/{{.BaseBranch}}` when
`BaseBranch` is set.

### Key constraints

- Modify `internal/agent/prompt.go`:
  - Add `BaseBranch string` to `PromptData`
- Modify the call sites that construct `PromptData` (in `internal/agent/`)
  to populate `BaseBranch` from `cfg.BaseBranch`
- Modify `prompts/implementer.txt`:
  - Update branch creation to base off `{{.BaseBranch}}` when set
  - Update `gh pr create` to include `--base {{.BaseBranch}}` when set
- Modify `prompts/spec_generator.txt`:
  - Same branch creation update

### Acceptance criteria

- [ ] `PromptData` has `BaseBranch` field
- [ ] Implementer prompt includes `--base <branch>` in `gh pr create` when
  `BaseBranch` is non-empty
- [ ] Implementer prompt branches off `origin/<BaseBranch>` when set
- [ ] When `BaseBranch` is empty, prompt behavior is unchanged (no `--base`
  flag, branches off current HEAD)
- [ ] Spec generator prompt branches off `origin/<BaseBranch>` when set

### Test cases

- **Render with BaseBranch set**: `RenderPrompt` with `BaseBranch: "feature/foo"`
  produces prompt containing `--base feature/foo`
- **Render with BaseBranch empty**: `RenderPrompt` with empty `BaseBranch`
  produces prompt without `--base` flag
- **Branch creation with BaseBranch**: Prompt contains
  `git checkout -b <branch> origin/feature/foo` when set
- **Branch creation without BaseBranch**: Prompt contains
  `git checkout -b <branch>` (current behavior) when empty

---

## Issue 313: Replace hardcoded "main" in prompt templates

**Blocked by**: #312

### Description

The implementer and spec generator prompts contain `Never commit directly to
main`. Replace this with `Never commit directly to {{.BaseBranch}}` when set,
falling back to `main` when empty. This prevents agents from accidentally
pushing to the configured base branch.

### Key constraints

- Modify `prompts/implementer.txt`:
  - Replace `Never commit directly to main` with a conditional that uses
    `{{.BaseBranch}}` when non-empty, `main` otherwise
- Modify `prompts/spec_generator.txt`:
  - Same replacement
- No Go code changes needed (BaseBranch already in PromptData from prior issue)

### Acceptance criteria

- [ ] Implementer prompt says "Never commit directly to feature/foo" when
  `BaseBranch` is `"feature/foo"`
- [ ] Implementer prompt says "Never commit directly to main" when
  `BaseBranch` is empty
- [ ] Spec generator prompt has the same behavior

### Test cases

- **Implementer with BaseBranch**: Rendered prompt contains
  "Never commit directly to feature/foo"
- **Implementer without BaseBranch**: Rendered prompt contains
  "Never commit directly to main"
- **Spec generator with BaseBranch**: Same as implementer tests

---

## Issue 314: Update `PullAfterMerge` to use configured base branch

**Blocked by**: #311

### Description

`orchestrator.PullAfterMerge` currently hardcodes `git pull --rebase origin
main`. Update it to accept the base branch as a parameter and use that instead.
Update all call sites (`implement.go`, `run.go`) to pass `cfg.BaseBranch`
through, falling back to `"main"` when empty.

### Key constraints

- Modify `internal/orchestrator/orchestrator.go`:
  - Change `PullAfterMerge(logger)` signature to
    `PullAfterMerge(branch string, logger *slog.Logger)`
  - Replace hardcoded `"main"` with the `branch` parameter
  - Update the warning message to reference the actual branch name
- Modify `internal/cmd/implement.go`:
  - Pass `cfg.BaseBranch` (or `"main"` if empty) to `PullAfterMerge`
- Modify `internal/cmd/run.go` (if it calls `PullAfterMerge`):
  - Same change
- Update any existing tests for `PullAfterMerge`

### Acceptance criteria

- [ ] `PullAfterMerge` pulls from the specified branch, not hardcoded "main"
- [ ] When `BaseBranch` is empty, callers pass "main" (preserving current
  behavior)
- [ ] Warning message references the actual branch name
- [ ] Existing `PullAfterMerge` tests updated and passing

### Test cases

- **Pull from custom branch**: `PullAfterMerge("feature/foo", logger)` runs
  `git pull --rebase origin feature/foo`
- **Pull from main**: `PullAfterMerge("main", logger)` runs
  `git pull --rebase origin main`
- **Dirty repo warning**: Warning message references the configured branch name,
  not hardcoded "main"

---

## Issue 315: Track base branch in run data

**Blocked by**: #311

### Description

Record the configured `BaseBranch` in `RunMeta` so that run data captures which
branch was targeted. This supports audit trail and makes it possible for the
dashboard to display the base branch.

### Key constraints

- Modify `internal/rundata/writer.go`:
  - Add `BaseBranch string` field to `RunMeta` with json tag
    `base_branch,omitempty`
  - Update `New()` to accept and store the base branch
- Modify call sites that create `rundata.Writer` (`implement.go`, `run.go`):
  - Pass `cfg.BaseBranch` to the writer constructor
- Existing run data files without `base_branch` should deserialize cleanly
  (omitempty handles this)

### Acceptance criteria

- [ ] `RunMeta` has `BaseBranch` field
- [ ] `run.json` includes `base_branch` when configured
- [ ] `run.json` omits `base_branch` when empty (backwards compatible)
- [ ] Existing run data without `base_branch` loads without error

### Test cases

- **Write with base branch**: Creating a writer with `BaseBranch: "feature/foo"`
  produces `run.json` containing `"base_branch":"feature/foo"`
- **Write without base branch**: Creating a writer with empty `BaseBranch`
  produces `run.json` without `base_branch` key
- **Read old data**: Loading run data that lacks `base_branch` field succeeds
  with empty `BaseBranch`

---

## Issue 316: Surface base branch in dashboard run detail page

**Blocked by**: #315

### Description

Display the configured base branch on the run detail page in the status
dashboard. When `BaseBranch` is non-empty, show it in the run metadata section
(next to repo, milestone, etc.). When empty, show nothing (default branch
behavior is implied).

### Key constraints

- Modify `internal/dashboard/handlers.go`:
  - `RunDetailData` already embeds `RunMeta` which will have `BaseBranch` -
    no struct changes needed
- Modify `internal/dashboard/templates/run-detail.html`:
  - Add conditional display of base branch in the metadata section
  - Only show when `BaseBranch` is non-empty
- No backend logic changes beyond what the rundata issue provides

### Acceptance criteria

- [ ] Run detail page shows "Base branch: feature/foo" when configured
- [ ] Run detail page does not show base branch info when empty
- [ ] Display is positioned near existing metadata (repo, milestone, timestamp)

### Test cases

- **Detail page with base branch**: Run with `BaseBranch: "feature/foo"` renders
  page containing "feature/foo"
- **Detail page without base branch**: Run with empty `BaseBranch` does not
  render any base branch element
