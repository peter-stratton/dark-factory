# Contributing to Dark Factory

Thanks for your interest in contributing! This project has an unusual
development model — most implementation work is done by autonomous AI agents —
but there are plenty of ways to get involved.

## Opening issues

Bug reports and feature requests are welcome. When opening an issue:

- Describe the problem or use case clearly
- Include steps to reproduce for bugs
- For feature requests, explain what you're trying to accomplish

The team triages issues and schedules them into milestones.

## Pull requests

PRs are welcome for bug fixes and small improvements.

This project uses autonomous agents for most implementation work, so large
feature PRs may not align with the current roadmap. If you're considering a
significant change, **open an issue first** to discuss the approach before
investing time in a PR. This helps avoid duplicate effort and ensures alignment
with the project direction.

## How development works

Dark Factory is built by its own pipeline:

1. A human writes the roadmap and specs (architecture, conventions, issue
   descriptions)
2. `godark run` picks up issues and assigns three agents — implementer,
   reviewer, and merger — that work autonomously
3. A human spot-checks the results

This is a deliberate model: humans make design decisions, agents execute them.
See [docs/roadmap/](docs/roadmap/) for the full roadmap and phasing.

## Local development

### Prerequisites

- Go 1.23+
- Docker (for sandboxed agent runs)
- [gh CLI](https://cli.github.com/) (GitHub operations)

### Build and test

```sh
go build ./cmd/godark
go test ./...
```
