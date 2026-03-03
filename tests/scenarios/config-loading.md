# Scenario: Config loading and validation

Relates to: Issue #2

## Setup
- A temporary directory containing a `godark.yaml` config file
- The config package (`internal/config`) is imported directly
- No external services or network access required

## Cases

### Load a complete YAML config file
Parse a valid `godark.yaml` with all fields populated.
- All fields are populated on the returned `Config` struct
- `Repo` matches the value in the YAML file
- `Milestone` matches the value in the YAML file
- `MaxRetries` matches the value in the YAML file
- `ProtectedPaths` contains the entries from the YAML file
- Nested structs (`Docker`, `Prompts`, `CrossCompile`) are populated correctly

### Apply default values when fields are omitted
Load a minimal config file that only sets `repo` and `milestone`.
- `MaxRetries` defaults to `2`
- `ScenarioDir` defaults to `tests/scenarios/`
- `ReviewDir` defaults to `tests/review/`
- `LogDir` defaults to `logs/`

### CLI flags override config file values
Load a config file, then apply `CLIFlags` with different values.
- Flag values take precedence over YAML values for `Repo`, `Milestone`, `Issue`, and `MaxRetries`
- Unset flags (nil pointers) do not overwrite YAML values

### Missing config file with sufficient flags
Call `Load` with a non-existent config path but provide all required values via `CLIFlags`.
- No error is returned
- The returned config uses flag values for `Repo` and `Milestone`
- Default values are applied for other fields

### Validation rejects missing repo
Load a config with an empty `repo` and no `--repo` flag.
- An error is returned
- The error message mentions `repo is required`

### Validation rejects missing milestone and issue
Load a config with no `milestone` and no `issue` (both zero values).
- An error is returned
- The error message mentions `milestone or issue is required`

### Malformed YAML returns a parse error
Provide a file with invalid YAML syntax.
- An error is returned
- The error message mentions `parsing config file`
