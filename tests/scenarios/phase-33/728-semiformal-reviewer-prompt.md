# Scenario: Semi-formal reviewer prompt template

Relates to: Issue #728

## Setup
- The `prompts/reviewer_semiformal.txt` file exists in the embedded prompt filesystem
- A populated `PromptData` struct is available with standard reviewer template variables (`IssueNumber`, `IssueTitle`, `IssueBody`, `PRNumber`, `Repo`, `ScenarioDir`, `ReviewDir`, `HasScenarioSpec`, `ArchitectureDocContent`, `ArchitectureJSON`, `EnforceArchitecture`, `TestCommand`, `BuildCommand`, `SharedRules`, `GeneratedPaths`)
- The `harnessPromptFiles` list in `scaffold.go` is the canonical registry of prompts installed by `godark init` and `godark new`
- The `configTail` constant in `init.go` defines the default `prompts:` section for new `godark.yaml` files

## Cases

### Template renders with all semi-formal sections
- GIVEN a `PromptData` with `HasScenarioSpec=true` and all standard fields populated
- WHEN `reviewer_semiformal.txt` is rendered via `RenderPrompt`
- THEN the output contains the section headers "PREMISES", "ACCEPTANCE TRACE", "REGRESSION TRACE", "UNCOVERED PATHS", and "FORMAL CONCLUSION"
- THEN the output contains the instruction that AGENT_RESULT must match the FORMAL CONCLUSION

### Template renders without scenario spec
- GIVEN a `PromptData` with `HasScenarioSpec=false`
- WHEN `reviewer_semiformal.txt` is rendered via `RenderPrompt`
- THEN the conditional integration test steps (write tests to ReviewDir, run tests) are absent
- THEN the semi-formal analysis block (all five sections) is still present

### Scaffold installs semiformal prompt
- GIVEN a target directory for prompt installation
- WHEN `writeHarnessPrompts` is called
- THEN `reviewer_semiformal.txt` is written to `prompts/reviewer_semiformal.txt` in the target directory

### Config tail includes semiformal prompt path
- GIVEN the `configTail` constant in `init.go`
- WHEN a new `godark.yaml` is generated from `configTail`
- THEN the `prompts:` section includes `reviewer_semiformal: prompts/reviewer_semiformal.txt`
