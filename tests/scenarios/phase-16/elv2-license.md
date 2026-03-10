# Scenario: ELv2 license file

Relates to: Issue #288

## Setup
- The repo root is the working directory

## Cases

### File exists
Check that `LICENSE` is present at the repo root.
- `LICENSE` file exists

### Header fields populated
Read the `LICENSE` file and check the header section.
- File contains `Dark Factory` as the Licensed Work
- File contains `2029-03-09` as the Change Date
- File contains `Peter Stratton` as the Licensor

### Canonical license text
Read the `LICENSE` file body.
- File contains the string `Elastic License 2.0`
- File contains the standard limitation of use clause
