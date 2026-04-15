# Scenario: Model field in step tracking pipeline

Relates to: Issue #778

## Setup
- A stats.db database initialized with the current schema (pre-migration)
- A godark config with `model: opus` and `model_overrides: { recon: sonnet }`
- An agent pipeline that executes recon, spec-generator, and implement steps

## Cases

### Model column added by migration
- GIVEN a stats.db that was created before this change (no `model` column)
- WHEN `migrate()` runs
- THEN the `step_results` table has a `model` column with default value `''`

### Migration is idempotent
- GIVEN a stats.db that already has the `model` column
- WHEN `migrate()` runs again
- THEN no error is returned and existing data is unchanged

### Default model persisted to step result
- GIVEN a config with `model: opus` and no overrides for the implement step
- WHEN the implement step completes and stats are written
- THEN the `step_results` row for the implement step has `model = "opus"`

### Model override persisted to step result
- GIVEN a config with `model: opus` and `model_overrides.recon: sonnet`
- WHEN the recon step completes and stats are written
- THEN the `step_results` row for the recon step has `model = "sonnet"`

### Multiple steps in same run use correct models
- GIVEN a config with `model: opus` and `model_overrides.recon: sonnet`
- WHEN a full pipeline run completes (recon, spec-generator, implement)
- THEN the recon step has `model = "sonnet"` and spec-generator and implement steps have `model = "opus"`

### Retry steps record model
- GIVEN a config with `model: opus` and `model_overrides.implementer_retry: sonnet`
- WHEN an implement step fails and a retry executes
- THEN the retry step result has `model = "sonnet"`
