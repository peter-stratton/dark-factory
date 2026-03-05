# Scenario: Architecture layer parser

Relates to: Issue #124

## Setup
- The `internal/harness/layers` package is tested directly via Go unit tests
- JSON input is provided as in-memory strings via `strings.NewReader` for
  `Parse` tests and as temp files for `ParseFile` tests
- No external services or network access required

## Cases

### Valid JSON parses correctly
Pass a valid JSON definition with 3 layers (types, config, orchestrator) where
config imports types and orchestrator imports both.
- `Parse` returns a non-nil `*Definition` with 3 layers
- Each layer has the correct `Name`, `Dir`, and `Imports` values

### ParseFile reads from disk
Write a valid JSON definition to a temp file. Call `ParseFile` with the path.
- Returns a non-nil `*Definition`
- The parsed content matches the file content

### Empty layers returns error
Pass `{"layers": []}` to `Parse`.
- An error is returned
- The error message mentions empty layers

### Duplicate layer names returns error
Pass a definition with two layers both named `"config"`.
- An error is returned
- The error message mentions the duplicate name `"config"`

### Bad import reference returns error
Pass a definition where a layer imports `"nonexistent"`.
- An error is returned
- The error message mentions `"nonexistent"`

### Empty name returns error
Pass a definition with a layer where `"name"` is `""`.
- An error is returned
- The error message mentions empty name

### Empty dir returns error
Pass a definition with a layer where `"dir"` is `""`.
- An error is returned
- The error message mentions empty dir

### Invalid JSON returns parse error
Pass `{malformed` to `Parse`.
- An error is returned
- The error is a JSON syntax or unmarshal error

### Missing imports field parses as empty slice
Pass a layer JSON object without the `"imports"` key.
- The layer parses successfully
- The `Imports` field is an empty (or nil) slice, not an error
