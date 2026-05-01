# Godark is Claude-Code-Native by Design

Godark is not a generic agent orchestrator. It is a workflow product built on
top of Claude Code, and that is a deliberate bet, not a limitation to apologize
for. This document records why, so future architectural decisions can be made
against a shared understanding rather than re-litigated each time.

## The two loops

Godark has two distinct loops, and they have very different relationships to
the harness:

### 1. Design loop (host, human + Claude Code)

This is where the quality of godark's output is actually determined:

- Authoring architecture docs and the layer graph
- Writing scenario specs in `tests/scenarios/`
- Defining conventions in `docs/conventions.md`
- Planning roadmap and phase docs
- Tuning prompts in `prompts/`
- Building and refining slash commands and skills

The design loop runs in the engineer's local Claude Code session. It depends
on Claude Code's:

- Slash command system (`/init`, `/review`, custom project commands)
- Skills system and skill auto-loading
- `CLAUDE.md` hierarchy (user, project, subdirectory)
- Interactive iteration model (the engineer-and-agent dialogue that produces
  scenarios and conventions)
- File-aware tool taxonomy (Read, Edit, Grep, etc.) that prompt authors assume

None of this is portable. Codex, opencode, and other harnesses do not have
equivalent systems. Reimplementing them inside godark would be a separate
project of comparable scope to godark itself.

### 2. Execution loop (sandboxed, agent in container)

This is where godark's automation runs:

- Implementer agent writes code and opens a PR
- Quality reviewer audits for security/perf
- Functional reviewer generates tests from the scenario specs
- Punchlist agent generates manual acceptance tests

The execution loop currently runs the `claude` CLI inside a Docker container.
It is the only layer where harness or model pluggability is even
conceptually meaningful, because it is consuming artifacts produced by the
design loop rather than producing them.

## What this means for pluggability

**Model pluggability** (the same Claude Code harness pointed at non-Anthropic
models via a translating proxy) is consistent with the Claude-Code-native bet.
The design loop still uses Claude Code; only the execution loop swaps the
backend. Phase 41's LiteLLM/proxy approach lives entirely inside loop 2 and
preserves the loop 1 surface unchanged. This is the right shape.

**Harness pluggability** (running Codex, opencode, or other CLIs in place of
Claude Code) is inconsistent with the bet. Even a maximally pluggable
execution loop cannot free a user from needing Claude Code, because the
design loop has no other home. The user story remains "Claude Code on the
front, anything on the back." Building a pluggable harness layer in the
execution loop would be substantial effort for an outcome that does not change
the user's actual install requirements.

## Decision rules

When evaluating future architectural changes, ask first which loop the change
lives in:

- **Design loop changes** (new slash commands, new skill conventions, new
  scenario format, prompt restructuring): assume Claude Code is present.
  Optimize for the engineer-and-Claude-Code interaction. Do not abstract.
- **Execution loop changes** (provider abstractions, sandbox abstractions,
  cost/quality routing): pluggability is on the table, but only if the
  motivating problem cannot be solved by swapping the model behind a fixed
  harness.

The `Provider.Harness()` field added in phase 41 is a cheap hedge against
future surprise. It is not a roadmap commitment to multi-harness support.

## Adoption framing

Godark is best described to new users as "the right way to run Claude Code
unattended on real engineering work." This is a more specific pitch than
"AI agent orchestrator" and a much smaller competitor set. Users who are
already Claude Code users have low marginal adoption cost. Users who are
not Claude Code users are not the target audience, and trying to serve them
would dilute the product.

## Related

- `docs/philosophy/engineering-roles.md` - what humans do in a godark project
- `docs/architecture.md` - layer rules and the design loop's outputs
- `docs/planning/phase-41-provider-abstraction.md` - the proxy approach this
  document justifies
