# Scenario: MergeStrategy typed enum with validation

Relates to: Issue #387

## Setup
- The `internal/config` package defines `FeatureMergeStrategy` and
  `RollupMergeStrategy` types with constants and `Valid()` methods

## Cases

### Valid feature strategies accepted
Check `Valid()` for each feature strategy.
- `FeatureMergeStrategy("none").Valid()` returns true
- `FeatureMergeStrategy("low_risk").Valid()` returns true
- `FeatureMergeStrategy("all").Valid()` returns true

### Invalid feature strategy rejected
Check `Valid()` for an invalid value.
- `FeatureMergeStrategy("invalid").Valid()` returns false

### Valid rollup strategies accepted
Check `Valid()` for each rollup strategy.
- `RollupMergeStrategy("none").Valid()` returns true
- `RollupMergeStrategy("manual").Valid()` returns true
- `RollupMergeStrategy("auto").Valid()` returns true

### Config validation rejects bad strategy
Parse a config YAML with `auto_merge: { feature: "bad" }`.
- Validation returns an error mentioning invalid strategy

### YAML round-trip preserves values
Marshal a config with `MergeAll` feature strategy to YAML and unmarshal it back.
- The unmarshalled value equals `MergeAll`

### Loop merge decision uses constants
Read `internal/agent/loop.go` merge decision switch.
- Cases reference `config.MergeAll` and `config.MergeLowRisk`, not string literals
