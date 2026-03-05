---
name: godark-define-architecture
description: Define or update architecture layer definitions for the project
argument-hint: "[project-path]"
disable-model-invocation: true
---

# Define Architecture

Help the user define or update the architecture layer definitions for their
project. For existing projects, scan the codebase to identify packages and
import relationships and propose layers. For new projects, ask about the
language and framework and propose idiomatic layers. Writes
`docs/architecture.json` and `docs/architecture.md`.

## Steps

1. **Read project configuration** — Read `godark.yaml` to find the `repo`,
   runtime info, and any configured paths. Note the language and framework from
   `runtime.name` if set.

2. **Read existing definitions** — Check whether `docs/architecture.json` and
   `docs/architecture.md` already exist. If they do, read both files so you can
   propose updates rather than starting from scratch.

3. **Assess the project state** — Determine whether this is an existing project
   with code or a new/empty project:
   - **Existing project**: Look for source directories (e.g. `internal/`,
     `src/`, `lib/`, `pkg/`, `app/`) and source files.
   - **New/empty project**: No significant source directories or files yet.

4. **For existing projects — scan the codebase**:
   - Identify package or module directories (e.g. Go packages under
     `internal/`, Python modules, Node.js packages, Java source trees).
   - Identify import relationships between packages — which packages import
     which others, and any circular import chains.
   - Group packages into candidate layers based on their role
     (e.g. `cmd`, `config`, `domain`, `infrastructure`, `api`, `storage`).
   - Note any packages that are unclear, overlap multiple layers, or participate
     in circular imports.

5. **For new/empty projects — gather requirements**:
   - Ask the user about the language and framework they plan to use.
   - Ask about the project type (e.g. web API, CLI tool, library, event-driven
     service, monorepo).
   - Propose idiomatic layers based on the language and framework (e.g. Clean
     Architecture for Go services, MVC for Rails, hexagonal architecture for
     Spring Boot apps).

6. **Discuss with the user**:
   - Present the proposed layers and ask what to keep, remove, rename, or change.
   - For existing projects: highlight any inconsistencies found — circular
     imports, packages that don't fit any layer, or unclear boundaries.
   - Iterate until the user is satisfied with the layer definitions.

7. **Write `docs/architecture.json`** — Write the agreed layer definitions in
   JSON format. Create the `docs/` directory if needed. The format is:

   ```json
   {
     "layers": [
       {
         "name": "layer-name",
         "description": "What this layer contains and its responsibility.",
         "paths": ["internal/example/"],
         "may_depend_on": ["other-layer"],
         "must_not_depend_on": ["higher-layer"]
       }
     ]
   }
   ```

   If `docs/architecture.json` already exists, update it with the agreed
   changes rather than replacing it wholesale.

8. **Update `docs/architecture.md`** — Write or update prose documentation
   describing each layer: what it contains, what it may depend on, and what it
   must not depend on. Include a section on cross-cutting concerns if relevant.

9. **Validate the result** — Run:
   ```
   godark vet architecture
   ```
   Review the output. If the command reports errors or warnings, explain them to
   the user and offer to fix them before finishing.

10. **Check for discrepancies** — If `godark vet architecture` reports
    discrepancies between the defined layers and the actual codebase (e.g.
    packages in the wrong layer, imports that violate layer rules), suggest
    running `/godark-create-roadmap` to plan a codebase alignment phase that
    migrates the code to match the architecture.

## Rules

- This skill is **re-runnable**. Always read existing definitions first and
  propose updates rather than overwriting without review.
- Do not skip the discussion step — the user must approve the layer definitions
  before they are written.
- `docs/architecture.json` is the authoritative machine-readable definition.
  `docs/architecture.md` is the human-readable companion — keep them in sync.
- Layer names must be kebab-case (e.g. `domain`, `storage`, `api-gateway`).
- Every layer must have at least a `name`, `description`, and `paths` field.
- If the project has no code yet and the user declines to answer questions,
  propose a minimal two-layer structure (e.g. `cmd` and `internal`) as a
  starting point.
