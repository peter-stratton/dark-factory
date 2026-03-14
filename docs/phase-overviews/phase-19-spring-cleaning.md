# Phase 19: Spring Cleaning

Eighteen phases of feature work left the codebase with duplicated patterns, magic strings, and copy-pasted boilerplate that accumulated faster than it was cleaned. Phase 19 is a focused consolidation pass: unified verdict parsing, typed constants replacing bare strings, extracted review cycle functions, shared CLI helpers, and a `CommandRunnerFunc` type adopted across packages. No new user-facing features -- just a codebase where every file is a clean example of the project's conventions.

---

## Unified Verdict Parsing

**What it does:** Replaces two nearly-identical functions (`ParseReviewResult` and `ParseQualityResult`) with a single `ParseVerdict` function. All agent prompts now emit `AGENT_RESULT=APPROVED` or `AGENT_RESULT=CHANGES_REQUESTED` instead of role-specific prefixes.

**Example:** Previously, the quality reviewer prompt used `QUALITY_RESULT=` and the functional reviewer used `REVIEW_RESULT=`, each parsed by its own function. Now both prompts use the same prefix:

```
AGENT_RESULT=APPROVED
```

The unified parser in `internal/agent/verdict.go`:

```go
func ParseVerdict(stdout, keyword string) string
```

Both `ParseReviewResult(stdout)` and `ParseQualityResult(stdout)` now delegate to `ParseVerdict(stdout, "AGENT")`. The prompt templates (`reviewer.txt` and `quality_reviewer.txt`) were updated to emit the unified `AGENT_RESULT=` prefix.

---

## Shared Critical Rules Template Variable

**What it does:** Extracts the protected-paths and scenario-dir rules that were duplicated across five prompt templates into a single `{{.SharedRules}}` template variable. Agent-specific rules (branch creation, review directory mandates) remain in each prompt.

**Example:** The `buildSharedRules()` function in `internal/agent/implementer.go` generates the shared rules from config values:

```go
func buildSharedRules(protectedPaths, scenarioDir string) string
```

Every prompt template that previously had its own copy of "Do NOT modify protected paths" and "Do NOT modify scenario specs" now includes:

```
{{.SharedRules}}
```

The `SharedRules` field on `PromptData` is populated by `newPromptData()` for all agent roles -- implementer, reviewer, quality reviewer, recon, and spec generator.

---

## Extracted Review Cycle Functions

**What it does:** The quality review and functional review retry loops were deeply nested inside `ProcessIssue()`. Phase 19 extracts each into its own function with explicit inputs and outputs, making the main orchestration loop readable at a glance.

**Example:** The quality review cycle in `internal/agent/loop.go`:

```go
func runQualityReviewCycle(
    ctx context.Context,
    issue github.Issue,
    prNum int,
    branch string,
    baseSHA string,
    cfg *config.Config,
    prompts *Prompts,
    authEnv map[string]string,
    logger *slog.Logger,
    hook RunDataHook,
    sessionID *string,
    fixCycles *int,
) (bool, error)
```

Returns `(true, nil)` when the quality reviewer approves, `(false, nil)` when all attempts are exhausted, and `(false, err)` on hard failures. The functional review cycle follows the same pattern with `runFunctionalReviewCycle()`, which additionally returns the outcome status and whether the PR was merged. The main `ProcessIssue()` function now reads as a sequence of named steps rather than a wall of nested conditionals.

---

## Typed Outcome Status Constants

**What it does:** Replaces bare string literals like `"implemented"`, `"needs-human-review"`, and `"failed"` with a typed `OutcomeStatus` type and named constants.

**Example:** In `internal/agent/loop.go`:

```go
type OutcomeStatus string

const (
    StatusImplemented      OutcomeStatus = "implemented"
    StatusReadyToMerge     OutcomeStatus = "ready-to-merge"
    StatusNeedsHumanReview OutcomeStatus = "needs-human-review"
    StatusFailed           OutcomeStatus = "failed"
)
```

Switch statements and return values throughout the orchestration loop now use these constants. The `rundata` package mirrors them as plain string constants for packages that cannot import the orchestration layer.

---

## Typed Merge Strategy Enums

**What it does:** Defines `FeatureMergeStrategy` and `RollupMergeStrategy` types with `Valid()` methods, replacing inline string comparisons in config validation and the merge decision path.

**Example:** In `internal/config/config.go`:

```go
type FeatureMergeStrategy string

const (
    MergeNone    FeatureMergeStrategy = "none"
    MergeLowRisk FeatureMergeStrategy = "low_risk"
    MergeAll     FeatureMergeStrategy = "all"
)

func (s FeatureMergeStrategy) Valid() bool {
    switch s {
    case MergeNone, MergeLowRisk, MergeAll:
        return true
    }
    return false
}
```

`RollupMergeStrategy` follows the same pattern with `RollupNone`, `RollupManual`, and `RollupAuto`. Config validation now calls `s.Valid()` instead of checking against hardcoded string lists.

---

## CommandRunnerFunc Type

**What it does:** Creates a shared function type in `internal/exec` for running shell commands, replacing three independent `var CommandRunner` definitions across packages. Enables consistent dependency injection for testing.

**Example:** In `internal/exec/exec.go`:

```go
type CommandRunnerFunc func(name string, args ...string) ([]byte, error)

var Default CommandRunnerFunc = func(name string, args ...string) ([]byte, error) {
    return osexec.Command(name, args...).CombinedOutput()
}
```

Packages that need to run shell commands (guardrails, doctor, sandbox) declare a package-level variable of type `gexec.CommandRunnerFunc` and default to `gexec.Default`. Tests swap in a fake by reassigning the variable -- no interfaces, no constructor injection, just a function type.

---

## CLI Helper Consolidation

**What it does:** Extracts three helpers that were duplicated across CLI command files: `parseCLIFlags()` for shared flag parsing, `resolveTag()` for milestone resolution, and `fetchVetData()` for vet command data fetching.

**Example:** The `parseCLIFlags()` function in `internal/cmd/cmdutil.go` consolidates flag extraction from `run.go` and `implement.go`:

```go
func parseCLIFlags(cmd *cobra.Command) config.CLIFlags
```

It reads seven flags (`--repo`, `--max-retries`, `--no-sandbox`, `--auto-merge-feature`, `--auto-merge-rollup`, `--base-branch`, `--default-branch`) and only writes values that were explicitly set (`cmd.Flags().Changed()`). Both `run.go` and `implement.go` now call this single function.

The `resolveTag()` helper in `internal/cmd/vet_helpers.go` handles the `--milestone` vs `--tag` mutual exclusivity and milestone title lookup. The `fetchVetData()` helper consolidates the GitHub API calls shared by `vet issues`, `vet scenarios`, and `vet roadmap`.

---

## WalkMarkdownFiles Utility

**What it does:** Extracts the repeated pattern of walking a directory for `.md` files into a shared helper in `internal/mdutil`, replacing three identical `filepath.WalkDir` + `.md` filter implementations.

**Example:** In `internal/mdutil/walk.go`:

```go
func WalkMarkdownFiles(dir string, fn func(path string) error) error
```

Walks the directory recursively, calling `fn` for each markdown file. Vet commands that previously had their own walk-and-filter boilerplate now call `mdutil.WalkMarkdownFiles(dir, func(path string) error { ... })`.

---

## Run Data Path Helpers

**What it does:** Extracts `issueDir()` and `issueRetryDir()` methods on `rundata.Writer`, replacing 15+ inline `fmt.Sprintf("%d", issueNum)` path constructions scattered across writer methods.

**Example:** In `internal/rundata/writer.go`:

```go
func (w *Writer) issueDir(issueNum int) string {
    return filepath.Join(w.dir, "issues", strconv.Itoa(issueNum))
}

func (w *Writer) issueRetryDir(issueNum, retryNum int) string {
    return filepath.Join(w.issueDir(issueNum), "retries", strconv.Itoa(retryNum))
}
```

Every `Write*Result` method now uses these helpers instead of constructing paths inline. The companion `writeJSONMkdirs()` function handles the `os.MkdirAll` + `json.MarshalIndent` + `os.WriteFile` sequence that was previously duplicated in each writer method.

---

## Truncation Limits Config Struct

**What it does:** Groups related truncation limits (`verify_output` and `pr_diff`) into a `TruncationLimits` struct instead of keeping them as separate top-level config fields.

**Example:** In `internal/config/config.go`:

```go
type TruncationLimits struct {
    VerifyOutput int `yaml:"verify_output"`
    PRDiff       int `yaml:"pr_diff"`
}
```

Defaults are 4096 bytes for verify output and 30000 bytes for PR diffs. These control how much agent output and diff context is included in prompts -- critical for staying within context window limits.

---

## Punchlist Parsing Helpers

**What it does:** Extracts `extractPrefixedItem()` to replace duplicated `HasPrefix`/`TrimPrefix` chains when parsing markdown checkboxes and bullet points from agent output.

**Example:** In `internal/punchlist/punchlist.go`:

```go
func extractPrefixedItem(line string, prefixes ...string) (string, bool)
```

Checks each prefix in order, returns the stripped content and `true` on first match. Used for parsing `- [ ]`, `- `, `* `, and numbered list items from punchlist and scenario spec output.

---

## Test Helper Consolidation

**What it does:** Consolidates duplicated skill test helpers into a shared `helpers_test.go` file in `internal/skills/`, replacing six copies of the same `readSkill` and `parseFrontmatter` functions.

**Example:** In `internal/skills/helpers_test.go`:

```go
func readSkill(t *testing.T, name string) string {
    t.Helper()
    path := name + "/SKILL.md"
    data, err := fs.ReadFile(skills.SkillFiles, path)
    if err != nil {
        t.Fatalf("reading %s: %v", path, err)
    }
    return string(data)
}
```

All skill test files (`godark_create_planning_doc_test.go`, `godark_define_architecture_test.go`, etc.) now use these shared helpers instead of their own local copies.
