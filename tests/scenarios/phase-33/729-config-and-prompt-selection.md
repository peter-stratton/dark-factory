# Scenario: Config and prompt selection for semi-formal review

Relates to: Issue #729

## Setup
- The `Config` struct in `internal/config/config.go` has a `Review` field
- The `Prompts` struct in both `internal/config/config.go` and `internal/agent/prompt.go` has a `ReviewerSemiformal` field
- `LoadPrompts()` loads the semiformal reviewer prompt from config or embedded fallback
- `Review()` in `internal/agent/reviewer.go` accepts a prompt string parameter
- `runFunctionalReviewCycle` in `loop.go` selects the prompt based on config and attempt number

## Cases

### Config unmarshals review section
- GIVEN a YAML config containing `review:\n  semiformal: true\n  semiformal_on_retry: false`
- WHEN the config is unmarshalled into a `Config` struct
- THEN `cfg.Review.Semiformal` is `true`
- THEN `cfg.Review.SemiformalOnRetry` is `false`

### Config defaults to false when review section absent
- GIVEN a YAML config with no `review:` section
- WHEN the config is unmarshalled into a `Config` struct
- THEN `cfg.Review.Semiformal` is `false`
- THEN `cfg.Review.SemiformalOnRetry` is `false`

### LoadPrompts loads semiformal reviewer
- GIVEN a prompts directory containing `reviewer_semiformal.txt`
- WHEN `LoadPrompts()` is called
- THEN `Prompts.ReviewerSemiformal` is non-empty and contains the template content

### Review function accepts prompt parameter
- GIVEN a custom prompt string passed to `Review()`
- WHEN the reviewer agent runs
- THEN the rendered prompt uses the provided string, not the default `prompts.Reviewer`

### Semiformal prompt selected when enabled
- GIVEN `cfg.Review.Semiformal` is `true` and `cfg.Review.SemiformalOnRetry` is `false`
- WHEN `runFunctionalReviewCycle` selects a prompt at attempt 0
- THEN the semiformal reviewer prompt is used

### Standard prompt used when both flags false
- GIVEN `cfg.Review.Semiformal` is `false` and `cfg.Review.SemiformalOnRetry` is `false`
- WHEN `runFunctionalReviewCycle` selects a prompt at any attempt
- THEN the standard reviewer prompt is used

### Semiformal prompt selected on retry only
- GIVEN `cfg.Review.Semiformal` is `false` and `cfg.Review.SemiformalOnRetry` is `true`
- WHEN `runFunctionalReviewCycle` selects a prompt at attempt 0
- THEN the standard reviewer prompt is used
- WHEN `runFunctionalReviewCycle` selects a prompt at attempt 1
- THEN the semiformal reviewer prompt is used
