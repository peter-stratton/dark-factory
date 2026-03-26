---
name: godark-harness-claude-md
description: Compress and organize CLAUDE.md into a minimal directory of pointers
argument-hint: "[project-path]"
disable-model-invocation: true
---

# Harness CLAUDE.md

Compress an existing CLAUDE.md (or create a minimal one) so that it serves as a
directory of pointers — not an encyclopedia. Content that belongs in subordinate
docs is extracted and moved there. What remains is a project identity line and a
routing table.

This skill should be run **after** `/godark-define-architecture` and
`/godark-define-conventions` so that the subordinate docs are already populated.

## Steps

1. **Read subordinate docs** — Read the following files (if they exist) to
   understand what content is already covered outside CLAUDE.md:
   - `docs/architecture.md` and `docs/architecture.json`
   - `docs/conventions.md`
   - `docs/ROADMAP.md`
   - `godark.yaml`

   If `docs/architecture.md` and `docs/conventions.md` are missing or still
   contain only template placeholders, stop and recommend running
   `/godark-define-architecture` and `/godark-define-conventions` first.

2. **Read the existing CLAUDE.md** — If one exists, read it in full. Identify
   every block of content and classify it:
   - **Duplicated** — content that already exists in a subordinate doc,
     prompt template, or tool config (e.g. error handling rules that match
     `docs/conventions.md`, git workflow already in prompt templates, protected
     paths already in `godark.yaml`). Action: drop without
     moving — the content is already where it belongs.
   - **Misplaced** — content that belongs in a subordinate doc or prompt
     template but isn't there yet (e.g. inline coding conventions, architecture
     descriptions, test patterns). Action: move to the destination, then drop
     from CLAUDE.md.
   - **Routing** — content that is already a pointer to another file (keep it)
   - **Unique** — content that doesn't fit any standard subordinate doc
     (project-specific notes, unusual workflow rules, team preferences)

3. **Handle misplaced content** — For each block classified as misplaced:
   - Identify the natural destination. Use the default harness structure as a
     guide, but respect the project's existing layout:
     - Architecture content → `docs/architecture.md` (or wherever the project
       keeps architecture docs, e.g. `docs/ADR/`)
     - Coding conventions → `docs/conventions.md` (or `CONTRIBUTING.md`,
       `docs/STYLE.md`, etc.)
     - Build/test/lint commands → `godark.yaml` or the project's CI config
     - Git workflow (branching, commit conventions) → prompt templates
     - Definition of done / completion criteria → prompt templates
     - Protected path lists → `godark.yaml`
   - Ask the user where each block should go. Suggest the default but accept
     alternatives.
   - Move the content to the agreed destination. If the destination file already
     exists, append or merge — do not overwrite.

4. **Handle unique content** — For each block classified as unique:
   - Ask the user whether it should stay in CLAUDE.md (as a pointer or brief
     note) or move to a specific file.
   - If the user wants it in a separate file, write it there and add a pointer
     in CLAUDE.md.
   - If the user wants it inline, keep it — but flag if CLAUDE.md is getting
     long.

5. **Discuss the project identity** — Ask the user for a one-line description
   of what the project does and who it's for. This becomes the opening line of
   CLAUDE.md.

6. **Write the new CLAUDE.md** — Assemble the compressed CLAUDE.md using the
   format below. The file should be a directory of pointers, not a repository of
   rules.

7. **Verify conciseness** — Count the lines. If over 30 lines, review with the
   user and look for content that can be moved to a subordinate doc. The target
   is under 20 lines for most projects.

## Format

```markdown
# CLAUDE.md

<One-line project description.>

## Where to look

- <Topic> — <path>
- <Topic> — <path>
- <Topic> — <path>
```

Additional sections are permitted for truly unique content that has no natural
home, but each should be at most a few lines. If a section grows beyond 5 lines,
it probably belongs in a subordinate doc.

## Example

```markdown
# CLAUDE.md

<One-line project description.>

## Where to look

- Architecture and layer definitions — docs/architecture.md
- Coding conventions — docs/conventions.md
- Roadmap and phasing — docs/ROADMAP.md
- Build, test, and runtime config — godark.yaml
```

## Rules

- This skill is **re-runnable**. Always read the existing CLAUDE.md first.
- Do not skip the discussion steps — the user decides where content moves.
- Never silently delete content. Every block from the old CLAUDE.md must be
  accounted for: moved to a subordinate doc, kept as a pointer, or explicitly
  dropped by the user.
- The default harness structure (`docs/architecture.md`, `docs/conventions.md`)
  is a suggestion, not a requirement. If the project already has conventions in
  `CONTRIBUTING.md`, point there instead.
- Do not modify subordinate docs without the user's approval — you are
  proposing moves, not executing them unilaterally.
- CLAUDE.md must not contain:
  - File paths that duplicate what's in `godark.yaml` or prompt templates
  - Code examples or function signatures (they go stale)
  - Style rules enforced by linters (redundant)
  - Build/test commands available in `godark.yaml` or detectable from the project
  - Protected path lists (enforced mechanically by `godark.yaml`)
  - Git workflow rules (belong in prompt templates injected per-run)
  - Definition of done / completion checklists (belong in prompt templates)
  - Content that duplicates a subordinate doc
