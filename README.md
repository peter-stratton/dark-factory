```
     _            _           __            _
  __| | __ _ _ __| | __      / _| __ _  ___| |_ ___  _ __ _   _
 / _` |/ _` | '__| |/ /_____| |_ / _` |/ __| __/ _ \| '__| | | |
| (_| | (_| | |  |   <______|  _| (_| | (__| || (_) | |  | |_| |
 \__,_|\__,_|_|  |_|\_\     |_|  \__,_|\___|\__\___/|_|   \__, |
                                                           |___/
```

A Go CLI built for [Claude Code](https://docs.anthropic.com/en/docs/claude-code)
that orchestrates autonomous AI agents to implement GitHub issues, review their
own work, and merge — without human intervention.

**[Documentation](https://godarkfactory.com)** ·
**[Getting Started](https://godarkfactory.com/docs/getting-started)** ·
**[Releases](https://github.com/peter-stratton/dark-factory/releases)**

### Philosophy

The hard part of software engineering isn't typing code — it's deciding what to
build and how it fits. Dark Factory keeps those decisions with humans. Engineers
write the roadmap, define architecture layers, design conventions, and author
issue specs. Agents operate within those constraints. The harness *is* the
design.

This is a collaborative architecture tool, not a "throw a ticket at an AI and
hope for the best" system. The adversarial review model reinforces this: a
separate reviewer agent checks whether the code respects the architecture a
human defined, follows conventions a human wrote, and meets acceptance criteria
a human specified. Every judgment call that shapes a codebase stays with the
humans who understand it.

Dark Factory has been built entirely by its own agent pipeline — every feature
was implemented, reviewed, and merged by `godark run`. The humans write specs
and design harnesses; the agents write code.

## Install

**Homebrew** (macOS):

```bash
brew install peter-stratton/dark-factory/godark
```

**Go install**:

```bash
go install github.com/peter-stratton/dark-factory/cmd/godark@latest
```

**Binary download**: grab a pre-built binary from
[GitHub Releases](https://github.com/peter-stratton/dark-factory/releases).

### Platform support

Dark Factory is built for Claude Code and GitHub. The architecture is designed
around Claude Code's specific capabilities — session resumption, CLAUDE.md as a
control surface, slash command skills, and sandboxed execution.

| Layer | Supported |
|-------|-----------|
| AI agent | Claude Code (Anthropic) |
| Version control | GitHub |

### Features

- **Three-agent pipeline** — implementer, quality reviewer, and functional reviewer are independent agents with isolated permissions; reviewers literally cannot edit files
- **Specification-driven quality gates** — human-authored scenario specs define "done"; the functional reviewer generates ephemeral integration tests from specs, not just rubber-stamping the diff
- **Architecture-as-code enforcement** — machine-readable layer definitions validated by `godark vet`; reviewers check architectural compliance, not just correctness
- **Structured agent dialogue** — implementer posts reasoning as PR comments, reviewers challenge it; the PR thread is an auditable record of adversarial design review
- **Full run observability** — local web dashboard with review chain timelines, quality flags, tool traces, and agent dialogue history for every issue
- **Harness engineering lifecycle** — scaffold, validate, and enforce project constraints with `godark new`, `godark init`, `godark vet`, and six harness types
- **Auto-detected multi-language support** — detects project type from marker files and configures the sandbox, build, and test commands automatically
- **Fully sandboxed agent runs by default** — agents execute inside ephemeral Docker containers with no access to the host filesystem or network beyond what's explicitly configured
- **Single binary, runs on a laptop** — no infrastructure fleet, no MCP server farm; just a Go binary, and Docker

## How it works

Given a GitHub repo and a milestone, `godark` runs a three-agent development loop:

1. **Fetch** open issues from the milestone, sorted by priority (`p1` → `p2` → `p3` → unlabeled)
2. **Resolve dependencies** — issues declare `Blocked by: #N` or `Depends on: #N` in their body; skip any whose dependencies are still open
3. **Implementer** — Claude Code implements the issue, writes unit tests, and opens a PR
4. **Guard rails** — verify the PR exists, contains `Closes #N`, and didn't touch protected files
5. **Quality reviewer** — a separate Claude Code instance audits the PR for security, performance, and code quality issues; if it requests changes, the implementer retries before functional review begins
6. **Functional reviewer** — another Claude Code instance reviews the PR against human-authored scenario specs, generates ephemeral integration tests, and approves or requests changes
7. **Retry loop** — if either reviewer rejects, the implementer reads the review comments and pushes fixes (max N retries per gate)
8. **Merge or escalate** — approved PRs are squash-merged; failed PRs are labeled `needs-human-review`
9. **Punchlist** — for each merged PR, a tool-less punchlist agent generates 3-5 concrete manual acceptance tests (specific config values, commands, expected outcomes) rendered as checkboxes alongside the existing punchlist output
10. **Repeat** — move to the next unblocked issue

## Quick start

```bash
# New project
godark new my-project --repo owner/my-project

# Existing project
godark init --repo owner/my-project
```

Then open the project in Claude Code and use the built-in skills to define your
architecture, conventions, and roadmap. See the
[Getting Started guide](https://godarkfactory.com/docs/getting-started) for a
full walkthrough.

## Documentation

Full documentation is available at **[godarkfactory.com](https://godarkfactory.com)**:

- [Getting Started](https://godarkfactory.com/docs/getting-started) — installation, setup, and tutorial
- [CLI Reference](https://godarkfactory.com/docs/cli) — all commands, flags, and usage examples
- [Configuration](https://godarkfactory.com/docs/configuration) — `godark.yaml` deep dive
- [Skills](https://godarkfactory.com/docs/skills) — slash commands for roadmaps, planning, issues, and more
- [Licensing & Adoption](https://godarkfactory.com/docs/licensing) — commercial use, data privacy, and FAQ

## Phase overviews

Each completed phase has a practical overview with real-world examples showing
what was built and how users experience it. These live in
[`docs/phase-overviews/`](docs/phase-overviews/):

| Phase | Overview |
|-------|----------|
| 1 | [Skeleton & Orchestration](docs/phase-overviews/phase-01-skeleton-and-orchestration.md) — CLI scaffold, config, deps, dry-run |
| 2 | [Quality & Vetting](docs/phase-overviews/phase-02-quality-and-vetting.md) — `godark vet` validation framework |
| 3 | [Docker Sandbox](docs/phase-overviews/phase-03-docker-sandbox.md) — container isolation, auth, cloning |
| 4 | [Agent Execution](docs/phase-overviews/phase-04-agent-execution.md) — implementer, reviewer, guard rails, retry loop |
| 5 | [Agent SDK Migration](docs/phase-overviews/phase-05-agent-sdk-migration.md) — SDK wrapper, role permissions, session resumption |
| 6 | [Multi-Language Support](docs/phase-overviews/phase-06-multi-language-support.md) — auto-detect, runtime config, pluggable Dockerfiles |
| 7 | [Review Quality & Dashboard](docs/phase-overviews/phase-07-review-quality-and-dashboard.md) — run data, quality flags, web dashboard |
| 8 | [Harness Engineering](docs/phase-overviews/phase-08-harness-engineering.md) — harness templates, `godark new`, vet architecture |
| 9 | [Harness-Aware Agent Execution](docs/phase-overviews/phase-09-harness-aware-agent-execution.md) — harness injection, dialogue, enforcement |
| 10 | [Deterministic Verification Pipeline](docs/phase-overviews/phase-10-deterministic-verification-pipeline.md) — verify step, auto-fix, bash deny-list |
| 11 | [Run Analysis & Prompt Feedback](docs/phase-overviews/phase-11-run-analysis-and-prompt-feedback.md) — `godark analyze`, trends, prompt gaps |
| 12 | [Complex Project Support](docs/phase-overviews/phase-12-complex-project-support.md) — multi-module, codegen, secrets, CI checks |
| 13 | [Human-in-the-Loop Review](docs/phase-overviews/phase-13-human-in-the-loop-review.md) — graduated auto-merge, watch command, risk classifier, notifications |
| 14 | *Deferred* — Bounded Concurrency |
| 15 | *Deferred* — Server Mode & Centralized Operation |
| 16 | [Public Release](docs/phase-overviews/phase-16-public-release.md) — ELv2 license, GoReleaser, Homebrew tap, release workflow, CONTRIBUTING.md |
| 17 | [Configurable Base Branch](docs/phase-overviews/phase-17-configurable-base-branch.md) — base branch config, PR targeting, prompt safety, run data tracking |
| 18 | [Adaptive Agent Loop](docs/phase-overviews/phase-18-adaptive-agent-loop.md) — recon agent, hybrid retry strategy, handoff context |
| 19 | [Spring Cleaning](docs/phase-overviews/phase-19-spring-cleaning.md) — unified verdict parsing, typed constants, shared helpers, CLI consolidation |
| 20 | [Terminal UI](docs/phase-overviews/phase-20-terminal-ui.md) — Bubble Tea TUI, progress reporter, adaptive colors, hybrid output mode |

To generate an overview for a newly completed phase, use `/godark-create-phase-overview <phase-number>`.

## Building

```bash
go build -o bin/godark ./cmd/godark
go test ./...
```

## Status

See [docs/ROADMAP.md](docs/ROADMAP.md) for the full development roadmap.

## License

Dark Factory is licensed under the [Elastic License 2.0](LICENSE). Free for
commercial use — the only restriction is you can't resell it as a hosted
service. See the [Licensing & Adoption](https://godarkfactory.com/docs/licensing)
page for details.
