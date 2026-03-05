# CLAUDE.md

<!-- Guidance: This file contains standing orders for all agents (human or
     AI) working on this project. Keep it short and authoritative. Do not
     include file paths, code examples, or style rules — those belong in
     docs/conventions.md and docs/architecture.md. -->

## Project

<!-- Guidance: One or two sentences describing what this project does and
     who it is for. State the core purpose clearly. -->

## Build and Test

<!-- Guidance: List the commands needed to build, test, and lint the
     project. Use plain prose or a short list — no code blocks needed.
     Include the test command, build command, and any required environment
     setup. -->

## Architecture

<!-- Guidance: Write a short conceptual summary of the architecture. Do not
     repeat the layer definitions from docs/architecture.md — instead,
     point agents there for the formal definitions. Describe the overall
     shape of the system in a few sentences. -->

See docs/architecture.md for layer definitions and dependency rules.

## Principles

<!-- Guidance: List the non-negotiable engineering principles for this
     project. These are the rules agents must follow even when not
     explicitly reminded. Examples: no global state, fail fast on
     misconfiguration, errors must always be handled. -->

## Protected Paths

<!-- Guidance: List the files and directories that must never be modified
     by implementing agents. Humans only. At minimum, include this file. -->

- `CLAUDE.md` — this file

## Git Workflow

<!-- Guidance: Describe the branching strategy, commit message conventions,
     and PR requirements. Keep it concrete — agents will follow these rules
     literally. -->

- Never commit directly to main. Use feature branches.
- Branch names: `<issue-number>-<short-slug>`
- Commit messages: include `Closes #N` or `Fixes #N`
- PRs must include `Closes #N` in the body

## Definition of Done

<!-- Guidance: List the conditions an issue must meet before it can be
     merged. Be explicit — agents use this checklist to decide when their
     work is complete. -->

- All acceptance criteria in the issue are satisfied
- Unit tests pass
- Build succeeds
- No protected paths were modified
