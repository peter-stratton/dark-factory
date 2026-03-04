# Scenario: Vet command scaffold and validation framework

Relates to: Issue #14

## Setup
- The CLI binary is built or the `cmd` package is imported directly
- The vet package (`internal/vet`) is imported for findings types
- No external services or network access required

## Cases

### Vet help shows subcommands
Run `godark vet --help`.
- Output lists `issues` subcommand with description
- Output lists `scenarios` subcommand with description
- Output lists `roadmap` subcommand with description

### Vet subcommand help shows flags
Run `godark vet issues --help`.
- Output includes `--repo` flag with description
- Output includes `--milestone` flag with description
- Output includes `--json` flag with description

### Create a finding with error severity
Construct a `Finding` with severity `Error`, a message, and a location.
- `Severity` field is `Error`
- `Message` field contains the provided text
- `Location` field contains the provided file path or issue number

### Report with error findings exits non-zero
Build a report containing one error finding and one warning finding.
- Report summary shows 1 error and 1 warning
- Report indicates exit code 1

### Report with only warnings exits zero
Build a report containing two warning findings and no error findings.
- Report summary shows 0 errors and 2 warnings
- Report indicates exit code 0

### Empty report exits zero
Build a report with no findings.
- Report summary shows 0 errors, 0 warnings, 0 info
- Output contains a message indicating no issues found
- Report indicates exit code 0

### JSON output format
Build a report with findings and render with JSON flag.
- Output is valid JSON
- JSON contains a `findings` array with severity, message, and location fields
- JSON contains a `summary` object with error, warning, and info counts
