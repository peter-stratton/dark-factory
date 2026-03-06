# Phase 9: Harness-Aware Agent Execution

> **Goal:** The harness files scaffolded and validated in Phase 8 are wired
> into actual agent runs. Agents read architecture and conventions docs,
> post structured dialogue on PRs, and the reviewer checks layer compliance
> — all driven by the orchestrator, not just prompt template text.

## Milestone

`Phase 9`

---

## Issue 146: Update architecture.json for dialogue package

### Description

Add `internal/dialogue/` to the domain layer paths in `docs/architecture.json`
and update the corresponding entry in `docs/architecture.md`. This prepares the
architecture definition for the new dialogue parsing package introduced in the
next issue.

### Key constraints

- Modify `docs/architecture.json` — add `"internal/dialogue/"` to the domain
  layer's `paths` array
- Modify `docs/architecture.md` — add `internal/dialogue/` to the domain layer's
  **Paths** list

### Acceptance criteria

- [ ] `docs/architecture.json` domain layer includes `internal/dialogue/`
- [ ] `docs/architecture.md` domain layer lists `internal/dialogue/`
- [ ] `godark vet architecture` still passes (no cycles introduced)

### Test cases

- **JSON valid**: `docs/architecture.json` parses without errors
- **Domain paths updated**: Domain layer `paths` array contains `internal/dialogue/`
- **Vet passes**: `godark vet architecture` produces no findings

---

## Issue 147: Populate harness template variables in launcher

### Description

Wire the `ArchitectureDoc` and `ConventionsDoc` fields in `PromptData` so they
are populated with file contents during agent runs. The prompt templates already
reference these variables with `{{- if .ArchitectureDoc}}` conditionals (added
in Phase 8), but the fields are never assigned in `newPromptData()`. This issue
adds config fields for the file paths and reads file contents at launch time.

### Key constraints

- Modify `internal/config/config.go`:
  - Add `ArchitectureDoc string` field with yaml tag `architecture_doc`
  - Add `ConventionsDoc string` field with yaml tag `conventions_doc`
  - Add defaults in `defaults()`: `ArchitectureDoc: "docs/architecture.md"`,
    `ConventionsDoc: "docs/conventions.md"`
- Modify `internal/agent/implementer.go`:
  - In `newPromptData()`, read file contents from `cfg.ArchitectureDoc` and
    `cfg.ConventionsDoc` using `os.ReadFile`
  - If the file does not exist, set the field to empty string (graceful
    degradation — the template conditionals already handle this)
  - Do not log an error for missing files — harness docs are optional

### Acceptance criteria

- [ ] Config struct has `ArchitectureDoc` and `ConventionsDoc` fields
- [ ] Defaults are `docs/architecture.md` and `docs/conventions.md`
- [ ] `newPromptData()` reads file contents into `PromptData` fields
- [ ] Missing files result in empty strings, not errors

### Test cases

- **Config defaults**: New config with no overrides has correct default paths
- **Config override**: Setting `architecture_doc: custom/arch.md` in YAML is
  reflected in parsed config
- **File exists**: `newPromptData()` populates `ArchitectureDoc` with file contents
  when the file exists
- **File missing**: `newPromptData()` sets `ArchitectureDoc` to empty string when
  the file does not exist
- **Both populated**: When both files exist, both fields are populated

---

## Issue 148: Structured PR comment parser

**Blocked by**: #146

### Description

Create a new `internal/dialogue/` package that parses structured Implementation
Notes and Review Notes from PR comment text. The parser extracts section content
from the markdown format defined in `docs/design/harnesses.md`. Used by the
orchestrator for telemetry and by the dashboard for display.

This is pure new code — no existing files are modified.

### Key constraints

- New package: `internal/dialogue/`
- Architecture layer: domain (pure text parsing, no external dependencies)
- Exported types:
  ```go
  type ImplementationNotes struct {
      Approach        string
      KeyDecisions    string
      KnownLimitations string
      Architecture    string
      Raw             string // full comment text
  }

  type ReviewNotes struct {
      Approved              string
      ChangesRequested      string
      ArchitectureCompliance string
      Raw                    string // full comment text
  }
  ```
- Exported functions:
  ```go
  // ParseImplementationNotes extracts Implementation Notes sections from
  // a PR comment body. Returns nil if the comment does not contain an
  // "## Implementation Notes" header.
  func ParseImplementationNotes(body string) *ImplementationNotes

  // ParseReviewNotes extracts Review Notes sections from a PR comment
  // body. Returns nil if the comment does not contain a
  // "## Review Notes" header.
  func ParseReviewNotes(body string) *ReviewNotes

  // ParseComments scans a slice of comment bodies and returns all
  // implementation notes and review notes found, in order.
  func ParseComments(bodies []string) ([]ImplementationNotes, []ReviewNotes)
  ```
- Parsing approach:
  - Look for `## Implementation Notes` or `## Review Notes` as the top-level
    header
  - Extract `### Approach`, `### Key Decisions`, etc. subsections
  - Content between subsection headers is the section text (trimmed)
  - If a subsection is missing, its field is empty string
  - Parsing is best-effort — agents may not follow the format exactly

### Acceptance criteria

- [ ] `ParseImplementationNotes` extracts all four subsections
- [ ] `ParseReviewNotes` extracts all three subsections
- [ ] Missing subsections result in empty strings
- [ ] Non-matching comments return nil
- [ ] `ParseComments` returns notes in order from multiple comments

### Test cases

- **Full implementation notes**: Comment with all four subsections parses correctly
- **Partial implementation notes**: Comment missing "Known Limitations" parses
  remaining sections, empty string for missing
- **Full review notes**: Comment with all three subsections parses correctly
- **Not a notes comment**: Regular PR comment returns nil
- **Multiple comments**: `ParseComments` with three comments (impl, review, impl)
  returns two implementation notes and one review notes in order
- **Whitespace handling**: Sections with leading/trailing whitespace are trimmed
- **Raw preserved**: `Raw` field contains the full original comment text

---

## Issue 150: Wire agent dialogue into run data

**Blocked by**: #148

### Description

Extend the run data structures to store parsed agent dialogue alongside
existing telemetry. The orchestrator fetches PR comments after each review
cycle, parses them with the dialogue package, and stores them in the per-issue
outcome.

### Key constraints

- Modify `internal/rundata/writer.go`:
  - Add `Dialogue` field to the issue-level data (new struct):
    ```go
    type DialogueEntry struct {
        Role  string `json:"role"`  // "implementer" or "reviewer"
        Round int    `json:"round"` // 1-indexed retry round
        Body  string `json:"body"`  // raw comment text
    }
    ```
  - Add `WriteDialogue(issueNum int, entries []DialogueEntry) error` method
    that writes `issues/<issueNum>/dialogue.json`
- Modify `internal/rundata/reader.go`:
  - Add `Dialogue []DialogueEntry` field to `IssueDetail`
  - Read `dialogue.json` in `ReadIssueDetail`
  - If `dialogue.json` does not exist, set `Dialogue` to nil (backwards
    compatible with existing run data)
- Modify `internal/orchestrator/orchestrator.go`:
  - After each review cycle, fetch PR comments via the GitHub client
  - Parse comments with `dialogue.ParseComments`
  - Map parsed notes to `DialogueEntry` structs
  - Call `WriteDialogue` with the accumulated entries

### Acceptance criteria

- [ ] `DialogueEntry` struct is defined in rundata
- [ ] `WriteDialogue` writes entries to `dialogue.json`
- [ ] `ReadIssueDetail` includes dialogue entries when file exists
- [ ] Missing `dialogue.json` results in nil dialogue (no error)
- [ ] Orchestrator writes dialogue after review cycles

### Test cases

- **Write dialogue**: `WriteDialogue` creates valid JSON with entries
- **Read dialogue**: `ReadIssueDetail` returns dialogue entries from file
- **Missing file**: `ReadIssueDetail` returns nil dialogue when file absent
- **Round trip**: Write then read produces identical entries
- **Multiple rounds**: Two retry rounds produce four entries (impl, review,
  impl, review)

---

## Issue 151: Surface agent dialogue in dashboard

**Blocked by**: #150

### Description

Add a dialogue timeline section to the dashboard's issue detail view. Shows
implementation notes and review notes as expandable sections, ordered by
round.

### Key constraints

- Modify `internal/dashboard/handlers.go`:
  - Add `Dialogue []rundata.DialogueEntry` field to `IssueDetailData`
  - Populate from `IssueDetail.Dialogue` in `handleIssueDetail()`
- Modify the issue detail template (embedded HTML):
  - Add a "Dialogue" section after the existing timeline
  - Each entry shows role (implementer/reviewer), round number, and body
  - Body is rendered in a collapsible/expandable `<details>` element
  - Entries with role "implementer" and "reviewer" have distinct visual
    styling (different border color or icon)
  - If no dialogue entries exist, the section is hidden

### Acceptance criteria

- [ ] Issue detail page shows dialogue section when entries exist
- [ ] Each entry displays role, round, and expandable body
- [ ] Implementer and reviewer entries are visually distinct
- [ ] Section is hidden when no dialogue exists
- [ ] Existing issue detail features are unchanged

### Test cases

- **Dialogue displayed**: Issue detail with dialogue entries renders the section
- **No dialogue**: Issue detail without dialogue does not render the section
- **Roles styled**: Implementer and reviewer entries have different visual markers
- **Expandable**: Dialogue body is inside a `<details>` element
- **Multiple rounds**: Three rounds of dialogue render six entries in order

---

## Issue 149: Architecture JSON context for reviewer

**Blocked by**: #147

### Description

Add an `{{.ArchitectureJSON}}` template variable that passes the machine-
readable layer definitions from `architecture.json` to the reviewer prompt.
This gives the reviewer structured data for layer compliance checking,
complementing the prose `{{.ArchitectureDoc}}`.

### Key constraints

- Modify `internal/config/config.go`:
  - Add `ArchitectureJSON string` field with yaml tag `architecture_json`
  - Add default in `defaults()`: `ArchitectureJSON: "docs/architecture.json"`
- Modify `internal/agent/prompt.go`:
  - Add `ArchitectureJSON string` to `PromptData`
- Modify `internal/agent/implementer.go`:
  - In `newPromptData()`, read file contents from `cfg.ArchitectureJSON`
  - Missing file results in empty string (same pattern as `ArchitectureDoc`)
- Modify `prompts/reviewer.txt`:
  - Add conditional block: `{{- if .ArchitectureJSON}}` that injects the
    JSON layer definitions and instructs the reviewer to check that imports
    in changed files respect the `may_depend_on` / `must_not_depend_on`
    rules
  - Update the embedded reviewer template in
    `internal/harness/templates/prompts/reviewer.txt` to match

### Acceptance criteria

- [ ] Config struct has `ArchitectureJSON` field with default path
- [ ] `PromptData` includes `ArchitectureJSON` field
- [ ] `newPromptData()` reads file contents (empty string if missing)
- [ ] Reviewer prompt conditionally includes JSON layer definitions
- [ ] Embedded reviewer template matches project prompt

### Test cases

- **Config default**: Default `architecture_json` is `docs/architecture.json`
- **File exists**: `PromptData.ArchitectureJSON` contains file contents
- **File missing**: `PromptData.ArchitectureJSON` is empty string
- **Reviewer prompt with JSON**: Rendered reviewer prompt includes layer
  definitions and compliance instructions when JSON exists
- **Reviewer prompt without JSON**: Rendered reviewer prompt omits the
  compliance block when JSON is absent

---

## Issue 152: Configurable architecture enforcement

**Blocked by**: #149

### Description

Add a configuration option that controls whether the reviewer treats
architecture layer violations as blocking (must request changes) or
informational (can still approve). This is a prompt-level directive,
similar to the existing `StrictnessDirective` pattern.

### Key constraints

- Modify `internal/config/config.go`:
  - Add `EnforceArchitecture bool` field with yaml tag `enforce_architecture`
  - Default: `false` (informational only)
- Modify `internal/agent/prompt.go`:
  - Add `EnforceArchitecture bool` to `PromptData`
- Modify `internal/agent/implementer.go`:
  - In `newPromptData()`, set `EnforceArchitecture` from `cfg.EnforceArchitecture`
  - Only meaningful when `ArchitectureJSON` is non-empty
- Modify `prompts/reviewer.txt`:
  - Inside the `{{- if .ArchitectureJSON}}` block, add conditional:
    - If `{{.EnforceArchitecture}}`: "Layer violations MUST result in
      CHANGES_REQUESTED. Do not approve PRs with layer violations."
    - If not: "Flag any layer violations in your Review Notes under
      Architecture Compliance, but they do not block approval."
  - Update embedded reviewer template to match

### Acceptance criteria

- [ ] Config struct has `EnforceArchitecture` field, default false
- [ ] `PromptData` includes `EnforceArchitecture` field
- [ ] Reviewer prompt includes blocking directive when enforcement is on
- [ ] Reviewer prompt includes informational directive when enforcement is off
- [ ] Directive is only included when `ArchitectureJSON` is non-empty

### Test cases

- **Default off**: New config has `EnforceArchitecture: false`
- **Config override**: Setting `enforce_architecture: true` in YAML is reflected
- **Enforce on**: Rendered reviewer prompt contains "MUST result in
  CHANGES_REQUESTED" when both enforce and JSON are set
- **Enforce off**: Rendered reviewer prompt contains "do not block approval"
  when enforce is false and JSON is set
- **No JSON**: Neither directive appears when `ArchitectureJSON` is empty
