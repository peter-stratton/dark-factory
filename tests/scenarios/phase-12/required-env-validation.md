# Scenario: Required env validation at startup

Relates to: Issue #222

## Setup
- The `internal/config/` package `ValidateRequiredEnv` function
- The `godark run` and `godark implement` command startup paths

## Cases

### All variables present
Call `ValidateRequiredEnv([]string{"HOME", "PATH"})`.
- Returns nil (no error)

### One variable missing
Call `ValidateRequiredEnv([]string{"DEFINITELY_NOT_SET_12345"})`.
- Returns an error
- Error message contains `DEFINITELY_NOT_SET_12345`

### Multiple variables missing
Call `ValidateRequiredEnv([]string{"MISSING_A_12345", "MISSING_B_12345"})`.
- Returns an error
- Error message contains both `MISSING_A_12345` and `MISSING_B_12345`

### Empty list skips validation
Call `ValidateRequiredEnv([]string{})`.
- Returns nil (no error)

### Nil list skips validation
Call `ValidateRequiredEnv(nil)`.
- Returns nil (no error)

### Mixed present and missing
Call `ValidateRequiredEnv([]string{"HOME", "DEFINITELY_NOT_SET_12345"})`.
- Returns an error
- Error message contains `DEFINITELY_NOT_SET_12345`
- Error message does not contain `HOME`

### Validation runs before Docker image build
In the `godark run` command path, `ValidateRequiredEnv` is called before
any Docker image build or agent invocation.
- Missing env var causes immediate exit with error
- No Docker image is built
