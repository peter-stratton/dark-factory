# Scenario: Prompt and permission audit

Relates to: Issue #635

## Setup
- All 8 prompt files in `prompts/` are the files under audit
- `_ROLE_PERMISSIONS` in `internal/agent/runner/agent_runner.py` defines each
  role's allowed and disallowed tools
- `RenderPrompt()` renders each template with standard `PromptData`

## Cases

### No prompt references disallowed tools
For each prompt file, render with standard `PromptData` and cross-reference
against its role's `disallowed_tools`.
- `spec_generator.txt` does not instruct the agent to use Bash
- `reviewer.txt` does not instruct the agent to use Write or Edit tools
- `quality_reviewer.txt` does not instruct the agent to use Write or Edit tools
- `punchlist.txt` does not instruct the agent to use Write, Edit, or Bash
- `recon.txt` does not instruct the agent to use Write, Edit, or Bash

### All prompts render without errors
Render all 8 prompt files with a standard `PromptData` containing all fields.
- No template parse errors
- No template execution errors
- All prompts produce non-empty output

### Read CLAUDE.md instruction evaluated
Check `implementer.txt` for the "Read CLAUDE.md" instruction.
- Instruction is either removed or has documented justification
- If kept, it does not block on missing CLAUDE.md (e.g. "if it exists")

### Reviewer uses Bash for test file creation
Check `reviewer.txt` for test file creation instructions.
- Instructions reference Bash/heredoc for file creation, not Write tool
- The phrase "Do NOT use the Write or Edit tools" is present
