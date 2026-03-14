---
name: godark-define-conventions
description: Define or update coding conventions for the project
argument-hint: "[project-path]"
disable-model-invocation: true
---

# Define Conventions

Help the user define or update coding conventions for their project. For
existing projects, read source files to identify patterns in use and propose
conventions to standardize on. For new projects, recommend idiomatic conventions
filtered for agent-friendliness. Writes `docs/conventions.md`.

## Steps

1. **Read project configuration** — Read `godark.yaml` to find the `repo`,
   runtime info (language, framework), and any configured paths.

2. **Read existing conventions** — Check whether `docs/conventions.md` already
   exists. If it does, read it so you can propose updates rather than starting
   from scratch.

3. **Read CLAUDE.md** — Read `CLAUDE.md` (if it exists) for any existing
   convention references, coding standards, or constraints already documented
   for agents.

4. **Assess the project state** — Determine whether this is an existing project
   with code or a new/empty project:
   - **Existing project**: Source directories (e.g. `internal/`, `src/`, `lib/`,
     `pkg/`, `app/`) and source files are present.
   - **New/empty project**: No significant source directories or files yet.

5. **For existing projects — analyse source files**:
   - Read a representative sample of source files (aim for 10–20 files across
     different packages or modules).
   - Identify patterns in use across the following dimensions:
     - **Error handling**: How are errors created, wrapped, and propagated?
       (e.g. sentinel values, wrapped errors, custom types, panic/recover)
     - **Logging**: What logging library or approach is used? How are log calls
       structured? (e.g. `slog`, `zap`, `logrus`, `fmt.Println`)
     - **Test style**: What testing approach is used? (e.g. table-driven tests,
       BDD frameworks, test helpers, mock libraries)
     - **Naming**: What naming conventions are followed? (e.g. exported vs
       unexported, interface naming, file naming)
     - **Dependency injection**: How are dependencies passed between components?
       (e.g. constructor injection, function parameters, DI containers, globals)
   - Note which patterns are used consistently, which are used inconsistently,
     and which are absent or unclear.

6. **For new/empty projects — gather requirements**:
   - Ask the user about the language and framework they plan to use.
   - Ask about the project type (e.g. web API, CLI tool, library, event-driven
     service, monorepo).
   - Propose idiomatic conventions for the chosen language and framework
     (e.g. Go standard library patterns, Python PEP 8, Rails conventions).

7. **Apply the agent-friendliness filter** — For each proposed or observed
   convention, assess how well it works with agentic development. Recommend
   patterns that agents can apply reliably:
   - **Explicit over implicit**: Prefer named dependencies and explicit
     configuration over dependency injection containers, service locators, or
     magic resolution. Agents can follow explicit wiring; they struggle with
     implicit resolution.
   - **Local over global**: Prefer function signatures and constructor parameters
     over shared mutable state or package-level globals. Agents can reason about
     local data flow; global side-effects are harder to track.
   - **Clear boundaries**: Prefer well-defined interfaces between layers over
     deep inheritance hierarchies or mixed-concern objects. Agents work best when
     the contract between components is visible in a few lines of code.
   - **Discoverable**: Prefer patterns that are visible and consistent in a few
     representative files over patterns that require whole-system understanding
     to apply correctly. Agents learn conventions by reading examples.

8. **Flag conventions that impede agentic development** — Identify any patterns
   in use (or being considered) that are likely to cause problems for agents:
   - **Heavy code generation**: Frameworks that require running generators to
     produce boilerplate (e.g. ORM migrations, protobuf stubs, dependency
     injection wiring) make it hard for agents to add new code without running
     external tools.
   - **Convention-over-configuration magic**: Frameworks that use implicit
     naming, directory structure, or annotation scanning (e.g. Rails auto-load,
     Spring component scan) make it hard for agents to understand what is
     registered and what is not.
   - **Implicit behavior**: Patterns where behaviour depends on runtime
     environment, feature flags, or ambient context that is not visible in the
     code (e.g. `context.Value`, thread-local storage, dynamic dispatch by
     string name).
   - For each flagged pattern, explain the specific risk and suggest an
     agent-friendly alternative.

9. **Discuss with the user** — Present the identified patterns, proposed
   conventions, agent-friendliness assessments, and any flagged anti-patterns.
   Ask the user:
   - Which conventions to adopt as project standards
   - How to handle any inconsistencies found in the existing codebase
   - Whether to accept or reject the agent-friendliness recommendations

   Iterate until the user is satisfied with the agreed conventions.

10. **Write `docs/conventions.md`** — Write the agreed conventions to
    `docs/conventions.md`. Create the `docs/` directory if needed. If the file
    already exists, update it with the agreed changes. Use the format below.

11. **Suggest next steps for inconsistencies** — If the existing codebase uses
    inconsistent conventions (e.g. two different logging libraries, mixed error
    handling styles), suggest running `/godark-create-milestone` to plan a
    standardization phase that migrates the codebase to the agreed conventions.

## Format

```markdown
# <Project Name> — Coding Conventions

> One-line description of the project and its language/framework.

---

## Error Handling

<Description of the agreed error handling convention.>

**Pattern**: `<example snippet or rule>`

---

## Logging

<Description of the agreed logging convention.>

**Pattern**: `<example snippet or rule>`

---

## Test Style

<Description of the agreed test style.>

**Pattern**: `<example snippet or rule>`

---

## Naming

<Description of the agreed naming conventions.>

---

## Dependency Injection

<Description of the agreed dependency injection approach.>

---

## Agent-Friendliness Notes

<Summary of recommendations applied and any flagged patterns to avoid.>
```

## Rules

- This skill is **re-runnable**. Always read existing conventions first and
  propose updates rather than overwriting without review.
- Do not skip the discussion step — the user must approve the conventions before
  they are written.
- Every section in `docs/conventions.md` must include at least a description and
  a concrete example or rule.
- If the project has no code yet and the user declines to answer questions,
  propose a minimal set of conventions covering error handling, logging, and
  test style as a starting point.
- Do not modify `CLAUDE.md` — that file is managed by humans. If the user wants
  to add conventions there, note what to add and ask them to do it manually.
