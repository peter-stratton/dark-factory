# Design: Harness Types

This document defines the harness types that godark supports, how each is
defined, updated over the project lifecycle, and leveraged during agent runs.

## Design Principles

These principles are drawn from external harness engineering patterns
(Anthropic, HumanLayer, Trail of Bits, OpenAI) and our own experience.

1. **CLAUDE.md is a short control document, not an encyclopedia.**
   Under 200 lines. It loads into every agent session, so bloat degrades
   all performance uniformly. Rules and principles only — no code examples,
   no file paths, no style rules that a linter handles.

2. **Prefer pointers to copies.**
   CLAUDE.md references subordinate docs by topic ("architecture docs live
   in docs/"), not by exact path. Agents discover relevant files and read
   them as needed. Prompt templates (managed by godark) inject specific
   paths at run time.

3. **Never send an LLM to do a linter's job.**
   Style rules, formatting, and import restrictions belong in deterministic
   tools (linters, formatters, CI). CLAUDE.md says "run the linter" not
   "indent with 2 spaces."

4. **Separate concerns by owner.**
   Some harness files are human-authored (CLAUDE.md, architecture docs,
   scenario specs). Some are agent-written at runtime (progress files,
   trace logs). Some are system-managed (godark config, prompt templates).
   Each file has one owner.

5. **Validate at definition time, enforce at run time.**
   Bad architecture or vague specs are caught by `godark vet` during
   planning, before any agent executes. Mechanical enforcement (linters,
   hooks, guard rails) catches violations during runs.

6. **Consider JSON for structured harness data.**
   Agents are less likely to corrupt JSON than Markdown. Structured data
   like layer definitions may be better as JSON than markdown tables.

---

## 1. Context Harness

**Purpose:** Give agents the background knowledge they need to make good
decisions — what the project does, why it's built this way, and what
conventions to follow.

**Files:**

| File | Owner | Content |
|------|-------|---------|
| `CLAUDE.md` | Human | Short control document: rules, workflow, principles, conceptual pointers to subordinate docs |
| `docs/architecture.md` | Human | Dependency layers, module boundaries, allowed dependencies |
| `docs/conventions.md` | Human | Coding patterns, error handling, testing approach |

### CLAUDE.md structure

CLAUDE.md is the highest-leverage file in the harness. It should contain:

- **Project identity** — one-line description of what this project does
- **Build and test commands** — the two or three commands agents need
- **Principles** — stable rules (error handling philosophy, no global
  state, dependencies flow downward)
- **Protected paths** — what agents must not modify
- **Conceptual pointers** — "architecture docs and coding conventions
  live in docs/" (no exact paths)
- **Git workflow** — branching and commit conventions
- **Definition of Done** — what "finished" means for a PR

It should NOT contain:

- File paths that can drift (use prompt templates for specific paths)
- Code examples or function signatures (they go stale)
- Style rules enforced by linters (redundant and noisy)
- Task-specific instructions (they distract during unrelated work)
- Anything longer than ~200 lines total

### Subordinate docs

`docs/architecture.md` and `docs/conventions.md` contain the detail that
CLAUDE.md points to. Agents read these when relevant to their task — the
implementer reads architecture before creating new packages, the reviewer
reads it when checking for layer violations.

These files can be longer and more detailed because they're only read when
needed, not loaded into every session.

### Defined

- `godark new` scaffolds CLAUDE.md with section headers and brief
  guidance comments. Scaffolds empty `docs/architecture.md` and
  `docs/conventions.md` with section templates.
- For existing projects, the human writes these by hand or uses the
  planning skills to develop them through conversation.

### Updated

- **By the human**, as decisions are made.
- **By the planning skills** — `/godark-create-roadmap` and
  `/godark-create-planning-doc` read these files and prompt the user to
  update them when the conversation reveals new context.

### Leveraged during a run

- **Agents:** CLAUDE.md is read first in every session (built into prompt
  templates). Subordinate docs are read when the prompt template or
  CLAUDE.md indicates they're relevant.
- **Prompt templates** (managed by godark) inject specific file paths to
  subordinate docs at run time, so CLAUDE.md doesn't need to hardcode them.

---

## 2. Architectural Constraint Harness

**Purpose:** Define structural boundaries — which parts of the codebase can
depend on which other parts. Prevents circular dependencies, layering
violations, and "everything imports everything" entropy.

**Files:**

| File | Owner | Content |
|------|-------|---------|
| `docs/architecture.md` | Human | Layer definitions, module boundaries, dependency rules |
| `docs/architecture.json` | Human (optional) | Machine-readable layer definitions for tooling |

### Layer definition format

In `docs/architecture.md`, human-readable:

```markdown
## Layers

Dependencies flow top-to-bottom. Each layer may only import layers below it.

| Layer | Directory | May import |
|-------|-----------|------------|
| cmd | cmd/ | service, config, types |
| service | internal/service/ | storage, config, types |
| storage | internal/storage/ | config, types |
| config | internal/config/ | types |
| types | internal/types/ | (none) |
```

Optionally, in `docs/architecture.json`, machine-readable:

```json
{
  "layers": [
    {"name": "types", "dir": "internal/types/", "imports": []},
    {"name": "config", "dir": "internal/config/", "imports": ["types"]},
    {"name": "storage", "dir": "internal/storage/", "imports": ["types", "config"]},
    {"name": "service", "dir": "internal/service/", "imports": ["types", "config", "storage"]},
    {"name": "cmd", "dir": "cmd/", "imports": ["types", "config", "service"]}
  ]
}
```

The JSON format enables mechanical validation by `godark vet` and potential
future linter config generation. The markdown format is for human and agent
readability. If both exist, they should be kept in sync (godark could
generate one from the other).

### Defined

- `godark new` scaffolds `docs/architecture.md` with a commented-out
  example and explanation.
- The human fills it in during planning. Planning skills prompt for layer
  definitions when the conversation reveals architectural structure.

### Updated

- **By the human**, as architecture evolves.
- Planning skills prompt for updates when new phases introduce packages
  that don't fit the current layer structure.

### Validated

- `godark vet` validates the layer definition:
  - No cycles in the dependency graph (the only universally wrong property)
- Structural concerns like directory existence, import distance, and
  "utils smell" are project-specific — better handled by project linting.
- Validation happens at **definition time**, before agents execute.

### Leveraged during a run

- **Agent 1 (implementer):** reads architecture doc. Knows which packages
  it can import from where. Places new files in the correct layer.
- **Agent 2 (reviewer):** explicitly prompted to check layer compliance.
- **Lint integration (future):** godark could generate language-specific
  linter config from the JSON definition (`depguard` for Go, ESLint
  `import/no-restricted-paths` for Node).
- **Linters over documentation** — where a language ecosystem has import
  restriction tooling, the linter is the enforcement mechanism. The
  architecture doc is for agent understanding, not the primary enforcement.

---

## 3. Behavioral Contract Harness

**Purpose:** Define what "done" looks like for each feature in a way that
agents can verify. Human-authored specifications that bridge intent and
implementation.

**Files:**

| File | Owner | Content |
|------|-------|---------|
| `tests/scenarios/*.md` | Human | Scenario specs — behavioral contracts |
| `docs/planning/phase-N-*.md` | Human (via skill) | Planning docs with acceptance criteria |
| GitHub issue bodies | Human (via skill) | Structured requirements per issue |

### Defined

- `godark new` scaffolds empty `tests/scenarios/` directory.
- `/godark-create-planning-doc` creates planning docs with acceptance
  criteria and test cases per issue.
- `/godark-create-issues` creates GitHub issues with structured format.
- `/godark-create-scenario` creates scenario spec files for issues.

### Updated

- **By the human** — scenario specs are protected paths. Only humans
  edit them.
- Planning docs are updated by `/godark-create-issues` (adds issue
  numbers) but content is human-approved.

### Validated

- `godark vet scenarios` — validates format (required sections, outcome
  bullets, `Relates to` links).
- `godark vet issues` — validates issue body structure (description,
  acceptance criteria, test cases).

### Leveraged during a run

- **Agent 1 (implementer):** reads issue body for requirements and test
  cases. Writes unit tests covering the specified cases.
- **Agent 2 (reviewer):** reads matching scenario specs. Generates
  ephemeral integration tests from specs + actual code. Approves only
  if all tests pass and acceptance criteria are met.
- **godark itself:** warns if no scenario spec exists for an issue.

---

## 4. Workflow Harness

**Purpose:** Define how agents execute — what commands to run, what tools
they can use, what the retry/review cycle looks like.

**Files:**

| File | Owner | Content |
|------|-------|---------|
| `godark.yaml` | Human | Orchestrator config: timeouts, retries, lint commands, quality thresholds |
| `prompts/*.txt` | Human (scaffolded by godark) | Prompt templates for each agent role |
| `.claude/skills/` | System (managed by godark) | Planning skills |

### Defined

- `godark new` scaffolds all of these. `godark init` retrofits skills
  and config into existing projects.
- Prompt templates are copied from godark's built-in defaults but are
  project-owned files that can be customized.
- **Prompt templates are where specific file paths live** — they
  reference `docs/architecture.md`, scenario spec paths, build commands,
  etc. via template variables. This keeps CLAUDE.md path-free.

### Updated

- **By the human** — config and prompts are edited as needs change.
- `godark init` re-runs safely: overwrites skills (godark-managed),
  skips config (project-owned).

### Leveraged during a run

- **godark orchestrator:** reads config for timeouts, retries, milestone,
  repo, lint command, quality thresholds.
- **Agents:** receive prompts from template files with variables
  substituted (issue number, PR number, scenario paths, build/test
  commands, paths to architecture docs).
- **Guard rails:** configured thresholds and checks run between agent
  steps.

---

## 5. Protected Path Harness

**Purpose:** Prevent agents from modifying human-owned files. A hard
boundary between "agent-writable" and "human-only" areas of the codebase.

**Files:**

| File | Owner | Content |
|------|-------|---------|
| `CLAUDE.md` | Human | Lists protected paths (agent awareness) |
| `godark.yaml` | Human | Lists protected paths (mechanical enforcement) |

### Defined

- `godark new` pre-populates with `CLAUDE.md` and `tests/scenarios/`.
- Human adds project-specific paths as needed.

### Leveraged during a run

- **Agent prompt:** "do not modify protected paths" (from CLAUDE.md).
- **PreToolUse hook (Phase 5):** blocks Write/Edit to protected paths
  in real time, with a system message explaining why.
- **Guard rail (post-hoc):** `CheckProtectedDrift` inspects PR diff.
  Fails the PR if protected paths were changed.
- Two enforcement layers: preventive (hook) and detective (guard rail).
  This is "never send an LLM to do a linter's job" in practice — the
  prompt tells agents the rule, the hook mechanically enforces it.

---

## 6. Agent Dialogue Harness

**Purpose:** Enable structured communication between Agent 1 (implementer)
and Agent 2 (reviewer) via PR comments. The implementer explains its
reasoning; the reviewer challenges it. The PR comment thread becomes a
visible record of the adversarial process.

**Medium:** GitHub PR comments (not files in the repo).

### Why PR comments, not files

- No cleanup needed — comments live on the PR, not in the codebase
- Already visible via `gh pr view --comments`
- The reviewer's existing flow picks them up naturally
- Humans can read the thread when spot-checking in the dashboard
- No protected path or gitignore considerations

### Why not cross-issue learnings

We considered a shared learnings file that accumulates knowledge across
issues within a milestone. We rejected this because:

- **Institutional bias** — one agent's choices propagate as assumed truth;
  future agents are less likely to challenge them
- **Undermines adversarial model** — the two-agent separation exists so
  agents don't grade their own homework. Shared learnings create implicit
  agreement between agents that should be independently evaluating
- **Staleness** — learnings from issue #3 may be wrong by issue #20
- **Fresh perspective has value** — an agent that explores from scratch
  may catch patterns that a "learned" agent would accept uncritically.
  The token cost of re-exploration is worth the independent perspective

### Implementer → Reviewer: Implementation Notes

After opening the PR, the implementer posts a structured comment:

```markdown
## Implementation Notes

### Approach
Brief description of the approach taken and why.

### Key Decisions
- **Decision**: Rationale for the choice
- **Alternative considered**: Why it was rejected

### Known Limitations
- What this implementation doesn't handle and why

### Architecture
- What layers/packages were touched and how they relate
```

This gives the reviewer context it can't get from the diff alone —
the "why" behind the "what."

### Reviewer → Implementer: Review Notes

When requesting changes, the reviewer posts structured feedback:

```markdown
## Review Notes

### Approved
- What passed review and why

### Changes Requested
- Specific issue with specific reasoning
- Challenge to implementation notes reasoning where applicable

### Architecture Compliance
- Layer violations found (or confirmation of compliance)
```

This gives the implementer on retry specific, actionable feedback tied
to explicit reasoning it can agree with or push back on.

### Retry rounds

Each retry adds to the PR comment thread. The implementer posts updated
implementation notes explaining what changed and why. The reviewer
evaluates the new approach. By the end, the PR has a complete record:

```
Implementer: opens PR + implementation notes
Reviewer: changes requested + review notes
Implementer: pushes fixes + updated implementation notes
Reviewer: approved + final review notes
```

### Defined

- Built into the prompt templates. The implementer prompt includes
  instructions to post implementation notes as a PR comment. The reviewer
  prompt includes instructions to post structured review notes.
- `godark new` scaffolds prompt templates with these instructions.

### Leveraged during a run

- **Agent 1 (implementer):** posts implementation notes after opening PR.
  On retry, reads previous review notes and its own prior implementation
  notes from the comment thread.
- **Agent 2 (reviewer):** reads implementation notes to understand
  reasoning. Posts structured review notes with specific challenges.
- **Humans:** read the PR comment thread in the dashboard or on GitHub
  to understand agent decision-making and spot-check quality.
- **godark telemetry:** could parse comment structure to track decision
  quality metrics (how often does the reviewer challenge implementation
  notes? how often do challenges lead to changes?).

---

## Summary

### Ownership model

| File / Medium | Owner | Mutability |
|---------------|-------|-----------|
| `CLAUDE.md` | Human | Edited by hand, stable principles |
| `docs/architecture.md` | Human | Updated during planning |
| `docs/architecture.json` | Human | Optional, mirrors architecture.md |
| `docs/conventions.md` | Human | Updated during planning |
| `docs/ROADMAP.md` | Human (via skill) | Updated per phase |
| `docs/planning/*.md` | Human (via skill) | Created per phase |
| `tests/scenarios/*.md` | Human | Protected, created per issue |
| `godark.yaml` | Human | Edited by hand |
| `prompts/*.txt` | Human (scaffolded) | Customized per project |
| `.claude/skills/` | System (godark) | Overwritten by `godark init` |
| PR comments (impl notes) | Agent 1 | Written per issue, updated on retry |
| PR comments (review notes) | Agent 2 | Written per review cycle |

### Lifecycle

```
godark new              scaffolds all harness files (templates)
  |
  v
/godark-create-roadmap  fills ROADMAP.md, prompts for architecture
  |                     and conventions updates
  v
/godark-create-planning-doc   fills planning docs, updates architecture
  |                           when new layers emerge
  v
godark vet              validates architecture (no cycles), scenarios,
  |                     issues — catches problems before execution
  v
/godark-create-issues   creates GitHub issues from planning docs
  |
  v
/godark-create-scenario creates scenario specs for issues
  |
  v
godark vet              re-validate before execution
  |
  v
godark run              agents execute within all harness constraints
                        - CLAUDE.md read first (principles)
                        - prompt templates inject specific file paths
                        - linters enforce style mechanically
                        - hooks enforce protected paths mechanically
                        - implementer posts reasoning as PR comments
                        - reviewer reads reasoning, challenges decisions
                        - reviewer checks architectural compliance
                        - retry rounds build visible dialogue on PR
                        - guard rails catch anything that slips through
```
