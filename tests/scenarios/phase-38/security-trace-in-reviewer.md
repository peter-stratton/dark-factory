# Scenario: Security trace in semi-formal reviewer

Relates to: Issue #786

## Setup
- The `prompts/reviewer_semiformal.txt` template is loaded and rendered with a populated `PromptData`
- `CheckSemiformalConsistency` in `internal/quality/quality.go` is available for parsing reviewer output

## Cases

### Prompt contains SECURITY TRACE section
- GIVEN the `reviewer_semiformal.txt` prompt template
- WHEN rendered with a valid `PromptData`
- THEN the output contains a "SECURITY TRACE" section header between "UNCOVERED PATHS" and "FORMAL CONCLUSION"

### SECURITY TRACE checks for all required categories
- GIVEN the rendered `reviewer_semiformal.txt` prompt
- WHEN inspecting the SECURITY TRACE instructions
- THEN the prompt instructs the reviewer to check for hardcoded credentials, tokens without TTL, sensitive data in logs/caches, and unauthed endpoints

### FORMAL CONCLUSION includes security flag rule
- GIVEN the rendered `reviewer_semiformal.txt` prompt
- WHEN inspecting the FORMAL CONCLUSION derivation rules
- THEN the rules include that any FLAGGED security finding leads to CHANGES_REQUESTED

### FLAGGED finding with APPROVED verdict detected as inconsistency
- GIVEN reviewer output containing "FORMAL CONCLUSION", "FLAGGED", and "AGENT_RESULT=APPROVED"
- WHEN `CheckSemiformalConsistency` is called with that output
- THEN it returns a Flag with code "semiformal_inconsistency" and a message mentioning "security trace"

### CLEAR findings with APPROVED verdict is not flagged
- GIVEN reviewer output containing "FORMAL CONCLUSION", only "CLEAR" security findings, and "AGENT_RESULT=APPROVED"
- WHEN `CheckSemiformalConsistency` is called with that output
- THEN it returns nil

### FLAGGED finding with CHANGES_REQUESTED is not flagged
- GIVEN reviewer output containing "FORMAL CONCLUSION", "FLAGGED", and "AGENT_RESULT=CHANGES_REQUESTED"
- WHEN `CheckSemiformalConsistency` is called with that output
- THEN it returns nil

### Output without FORMAL CONCLUSION is skipped
- GIVEN reviewer output that does not contain "FORMAL CONCLUSION"
- WHEN `CheckSemiformalConsistency` is called with that output
- THEN it returns nil
