# Phase 9: Harness-Aware Agent Execution

Phase 8 created the harness files -- architecture docs, conventions docs, layer
definitions -- and taught `godark vet` to validate them. Phase 9 closes the
loop: these files now flow into live agent runs. The implementer reads your
architecture and conventions docs before writing code. The reviewer gets
machine-readable layer definitions and checks import compliance. Structured
dialogue between agents is captured, persisted, and surfaced in the dashboard.
Everything is wired through config defaults and graceful degradation, so
existing projects that lack harness files keep working without changes.


## Harness doc injection into agent prompts

The launcher reads `docs/architecture.md` and `docs/conventions.md` at run
time and injects their contents into the prompt templates via template
variables (`{{.ArchitectureDocContent}}`, `{{.ConventionsDocContent}}`). If
the files do not exist, the variables resolve to empty strings and the
conditional blocks in the templates are silently skipped.

**In practice:** You scaffold a project with `godark init`, which creates
`docs/architecture.md` and `docs/conventions.md`. When you run:

```
godark run --milestone "Phase 3" --repo myorg/myapp
```

the implementer agent sees your architecture layers and coding conventions
directly in its prompt -- it knows which layers exist, what can depend on
what, and what style rules to follow. If you later move your architecture doc
to a non-default location, override the path in `godark.yaml`:

```yaml
architecture_doc: design/layers.md
conventions_doc: design/style-guide.md
```

The config defaults (`docs/architecture.md`, `docs/conventions.md`,
`docs/architecture.json`) mean zero configuration for projects that follow
the standard layout.


## Architecture JSON for the reviewer

The reviewer prompt receives a new `{{.ArchitectureJSON}}` variable
containing the machine-readable layer graph from `docs/architecture.json`.
This gives the reviewer structured `may_depend_on` and `must_not_depend_on`
rules for each layer, so it can check whether imports in changed files
respect layer boundaries.

**In practice:** Suppose your `architecture.json` defines a domain layer that
must not depend on the infrastructure layer. When the reviewer examines a PR
diff and sees that `internal/dialogue/parser.go` imports
`internal/sandbox/docker.go`, it flags the violation in its Review Notes under
"Architecture Compliance."

The reviewer prompt renders like this when the JSON is present:

```
Machine-readable layer definitions -- for each changed file, verify that
its imports respect the `may_depend_on` and `must_not_depend_on` rules
for its layer:
{
  "layers": [
    {
      "name": "domain",
      "paths": ["internal/dialogue/", "internal/deps/", ...],
      "may_depend_on": [],
      "must_not_depend_on": ["infrastructure", "cmd"]
    },
    ...
  ]
}
```

When the file is missing, the entire block disappears from the prompt.


## Configurable architecture enforcement

A single config flag controls whether layer violations block PR approval or
are informational only. When `enforce_architecture` is `true`, the reviewer
prompt contains a hard directive: "Layer violations MUST result in
CHANGES_REQUESTED." When `false` (the default), violations are noted but do
not block approval.

**In practice:** A team adopting godark on an existing codebase with known
layer violations can leave enforcement off:

```yaml
# godark.yaml
enforce_architecture: false  # default -- violations are advisory
```

Once the codebase is clean, flip the switch:

```yaml
enforce_architecture: true
```

Now any PR that introduces a new layer violation gets rejected by the
reviewer automatically. The enforcement only takes effect when
`architecture.json` exists -- without the JSON, there is nothing to enforce.


## Structured PR comment format (agent dialogue)

The implementer and reviewer prompts now include structured comment formats.
The implementer posts an "## Implementation Notes" comment with subsections
for Approach, Key Decisions, Known Limitations, and Architecture. The
reviewer posts "## Review Notes" with subsections for the verdict and
Architecture Compliance. These are not just for humans reading the PR --
the orchestrator parses them.

**In practice:** After the implementer finishes, the PR gets a comment like:

```markdown
## Implementation Notes

### Approach
Added a new `dialogue` package in the domain layer with pure text parsing
functions. No external dependencies.

### Key Decisions
Used line-by-line scanning rather than regex to handle malformed markdown
gracefully.

### Known Limitations
Subsection headers must be exact matches (e.g., "### Approach" not
"### approach").

### Architecture
Touched domain layer only (internal/dialogue/). No new cross-layer
dependencies.
```

The reviewer reads this before starting its review, then posts its own
structured comment.


## Dialogue parser (`internal/dialogue/`)

A new domain-layer package that extracts structured notes from PR comment
text. `ParseImplementationNotes` and `ParseReviewNotes` scan a comment body
for the expected markdown headers and return typed structs with each
subsection's content. `ParseComments` processes a batch of comment bodies
and returns all notes found, in order. The parser also handles
`QualityReviewNotes` for the quality reviewer role.

**In practice:** The orchestrator calls `github.FetchPRCommentBodies` to get
all comments on a PR, then passes them through `dialogue.ParseComments`:

```go
implNotes, reviewNotes, qualityNotes := dialogue.ParseComments(bodies)
```

Parsing is best-effort. If an agent posts a comment without the expected
headers, the parser returns nil for that comment and moves on. Missing
subsections within a valid comment produce empty strings, not errors.


## Dialogue persistence in run data

After each issue is processed, the orchestrator fetches the PR comments,
parses them into dialogue entries, and writes a `dialogue.json` file
alongside the existing telemetry in the run directory. Each entry records
the role (implementer, quality_reviewer, or reviewer), the round number,
and the raw comment body.

**In practice:** A run directory for issue #148 looks like:

```
~/.godark/runs/myorg/myapp/20260307-143022/
  run.json
  issues/
    148/
      implement.json
      quality-review.json
      functional-review.json
      outcome.json
      dialogue.json          <-- new in Phase 9
```

The `dialogue.json` contains:

```json
[
  {"role": "implementer", "round": 1, "body": "## Implementation Notes\n\n### Approach\n..."},
  {"role": "quality_reviewer", "round": 1, "body": "## Quality Review Notes\n..."},
  {"role": "reviewer", "round": 1, "body": "## Review Notes\n..."}
]
```

Runs from before Phase 9 that lack `dialogue.json` are handled gracefully --
the reader returns nil for the dialogue field.


## Dialogue timeline in the dashboard

The `godark status` dashboard's issue detail view now includes a "Dialogue"
section showing the conversation between agents. Each entry appears as a
collapsible card with the role and round number in the summary line.
Implementer entries have a distinct accent-colored left border; quality
reviewer entries use a red border; functional reviewer entries use a warning
color. The section is hidden entirely when no dialogue exists.

**In practice:** Open the dashboard with `godark status`, navigate to a run,
click into an issue, and scroll past the timeline. You see:

```
Dialogue
  > Implementer - Round 1       [click to expand]
  > Quality Reviewer - Round 1  [click to expand]
  > Reviewer - Round 1          [click to expand]
  > Implementer - Round 2       [click to expand]
  > Reviewer - Round 2          [click to expand]
```

Expanding an entry shows the full comment body. This gives you a quick way
to audit the agent conversation without leaving the dashboard or opening
GitHub.
