# Phase 17: Configurable Base Branch

godark has always targeted the repo's default branch -- feature branches fork from `main`, PRs merge into `main`, and post-merge pulls sync from `main`. Phase 17 makes that target configurable. Teams that require peer review before merging to `main` can point godark at a parent feature branch, let agents work autonomously under it, then submit the parent branch for human review. The entire pipeline -- branching, PR creation, prompt instructions, post-merge sync, and run data -- respects the configured base branch.

---

## Base Branch Configuration

**What it does:** Adds a `base_branch` field to `godark.yaml` and a `--base-branch` CLI flag to both `godark run` and `godark implement`. When set, all agent work targets this branch instead of the repo's default. When empty (the default), behavior is unchanged.

**Example:** A platform team requires human approval on merges to `main`. They create a parent branch for each milestone and configure godark to work under it:

```yaml
base_branch: feature/phase-17
```

Or override per-invocation:

```
$ godark implement --issue 42 --base-branch feature/phase-17
```

The CLI flag takes precedence over the YAML value. Internally, `Config.EffectiveBaseBranch()` resolves the final value, defaulting to `"main"` when nothing is set:

```go
func (c *Config) EffectiveBaseBranch() string {
    if c.BaseBranch == "" {
        return "main"
    }
    return c.BaseBranch
}
```

---

## Automatic Base Branch Creation

**What it does:** Before processing any issues, godark checks whether the configured base branch exists on the remote. If it does not, godark creates it from HEAD. This eliminates a manual setup step -- users can specify a new branch name and godark handles the rest.

**Example:** A developer kicks off a run targeting a branch that does not exist yet:

```
$ godark run --milestone "Phase 18" --base-branch feature/phase-18
```

`EnsureBaseBranch` runs `git ls-remote --heads origin feature/phase-18`. Finding no match, it creates the branch:

```
INFO base branch does not exist on remote, creating from HEAD  branch=feature/phase-18
INFO created base branch on remote  branch=feature/phase-18
```

If the branch already exists, the check is a no-op:

```
INFO base branch already exists on remote  branch=feature/phase-18
```

When `base_branch` is empty, `EnsureBaseBranch` returns immediately -- the agent will use the repo's default branch, which always exists.

---

## PR Targeting

**What it does:** When `BaseBranch` is configured, the implementer agent creates feature branches from `origin/<BaseBranch>` and passes `--base <BaseBranch>` to `gh pr create` so PRs target the correct branch. The spec generator uses the same branching logic.

**Example:** With `base_branch: feature/phase-17`, the rendered implementer prompt includes:

```
- Create a feature branch: git fetch origin && git checkout -b 42-add-widget origin/feature/phase-17
```

And the PR creation step:

```
6. Push the branch and open a pull request targeting feature/phase-17: gh pr create --base feature/phase-17 ...
```

When `BaseBranch` is empty, the prompt uses the original behavior -- `git checkout -b 42-add-widget` from the current HEAD, and `gh pr create` without `--base` (targeting the repo default).

The `PromptData` struct carries `BaseBranch` through to the templates:

```go
type PromptData struct {
    // ...
    // BaseBranch, when non-empty, is the target branch for PRs. Prompts use
    // this to branch off origin/<BaseBranch> and pass --base <BaseBranch> to
    // gh pr create.
    BaseBranch string
}
```

---

## Prompt Safety Rules

**What it does:** Prompt templates that previously hardcoded "Never commit directly to main" now use the configured base branch. This prevents agents from accidentally pushing directly to whichever branch they are targeting.

**Example:** With `base_branch: feature/phase-17`, the implementer prompt renders:

```
- Never commit directly to feature/phase-17
```

With no base branch configured:

```
- Never commit directly to main
```

Both the implementer and spec generator templates use the same conditional:

```
- Never commit directly to {{if .BaseBranch}}{{.BaseBranch}}{{else}}main{{end}}
```

---

## Post-Merge Synchronization

**What it does:** After merging a PR, the orchestrator pulls the latest changes so subsequent issues build on top of the merged code. `PullAfterMerge` now accepts a branch parameter instead of hardcoding `origin main`.

**Example:** In a multi-issue run with `base_branch: feature/phase-17`, the orchestrator merges issue #42's PR and then syncs:

```go
baseBranch := cfg.EffectiveBaseBranch()
// ... after merge ...
if err := PullAfterMerge(baseBranch, logger); err != nil {
    // stop the loop
}
```

This runs `git pull --rebase origin feature/phase-17`. If the working tree is dirty (perhaps from a failed previous step), the error message references the actual branch:

```
WARN local repo has uncommitted changes — commit your changes then pull  branch=feature/phase-17
```

---

## Run Data Tracking

**What it does:** The configured base branch is recorded in `RunMeta` and persisted to `run.json`. This creates an audit trail -- you can look at any historical run and see which branch it targeted.

**Example:** A run with `base_branch: feature/phase-17` produces a `run.json` containing:

```json
{
  "repo": "myorg/myservice",
  "milestone": "Phase 17",
  "base_branch": "feature/phase-17",
  "issue_numbers": [311, 312, 313],
  "started_at": "2026-03-10T14:30:00Z"
}
```

When `base_branch` is empty, the field is omitted entirely (`omitempty`), so existing tools that parse run data are unaffected. Loading old run data that predates this feature works without error -- `BaseBranch` simply defaults to its zero value.

---

## Dashboard Display

**What it does:** The run detail page in `godark status` shows the configured base branch in the header metadata, next to the milestone and timestamp. When no base branch is configured, nothing extra appears.

**Example:** A run targeting `feature/phase-17` renders the subtitle as:

```
Phase 17 · 20260310-143000 · Base branch: feature/phase-17
```

A run with no base branch configured renders the usual:

```
Phase 17 · 20260310-143000
```

The template uses a simple conditional:

```html
<p class="page-header__subtitle">
  {{.Meta.Milestone}} &middot; {{.Timestamp}}
  {{if .Meta.BaseBranch}} &middot; Base branch: {{.Meta.BaseBranch}}{{end}}
</p>
```
