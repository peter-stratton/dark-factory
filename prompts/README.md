# Prompt Templates

These are the prompt templates used by godark's agents. They are installed and
updated by `godark init` — **any edits in this directory will be overwritten**
when you upgrade godark and re-run `godark init`.

## Customizing prompts

To override a prompt without losing your changes:

1. Copy the prompt to a custom location (e.g. `custom-prompts/reviewer.txt`)
2. Update `godark.yaml` to point to your copy:
   ```yaml
   prompts:
     reviewer: custom-prompts/reviewer.txt
   ```
3. Your custom path will be used instead of the default, and `godark init`
   won't touch it.

## Template variables

Prompts are Go templates with access to variables like `{{.IssueNumber}}`,
`{{.IssueTitle}}`, `{{.IssueBody}}`, `{{.BaseBranch}}`, `{{.BuildCommand}}`,
`{{.TestCommand}}`, and more. See the existing prompts for examples.
