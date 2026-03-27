## Phase 17: Configurable Base Branch ✅

**Goal**: godark supports branching off and merging into a configurable base
branch instead of always targeting the repo's default branch. Teams that require
peer review on merges to main can run godark autonomously on sub-branches under
a parent feature branch, then submit the parent for human review.

**Milestone**: `Phase 17` | **Label**: `phase-17`

- Add `base_branch` config field to `godark.yaml` and `--base-branch` CLI flag,
  defaulting to the repo's default branch when unset
- Pass `--base` flag to `gh pr create` so PRs target the configured base branch
- Replace hardcoded "main" references in prompt templates with a
  `{{.BaseBranch}}` template variable
- Update orchestrator post-merge pull to use the configured base branch instead
  of hardcoded `origin main`
- Track base branch in run data for audit trail
- Surface base branch name in the status dashboard on run detail pages
- Two-tier merge model via structured `auto_merge` config: `auto_merge.feature`
  controls how feature PRs are merged into the base branch (`none`, `low_risk`,
  `all`); `auto_merge.rollup` controls what godark does with the rollup PR that
  merges the base branch into main (`none` = human handles everything, `manual` =
  godark opens the PR and human merges, `auto` = godark opens and merges)

**Issues**: #311-#316

**Planning doc**: `docs/planning/phase-17-configurable-base-branch.md`

