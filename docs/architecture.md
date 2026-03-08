# Architecture

This document describes the architectural layers of the project. Formal layer
definitions (names, descriptions, allowed dependencies) are maintained in
`docs/architecture.json`.

## Dependency Diagram

```
cmd
 |
orchestration
 |
service
 |          \
domain    infrastructure
 |          |
foundation--+
 |
content
```

All layers may depend on layers below them in the diagram. No layer may depend
on a layer above it. `foundation` and `content` are leaf layers with no internal
dependencies.

## Layers

### cmd

**Paths:** `cmd/`, `internal/cmd/`

**Purpose:** CLI entry point and Cobra command wiring. This is the top of the
dependency tree — it wires commands to orchestration, services, and domain logic.

**May depend on:** all other layers.

### orchestration

**Paths:** `internal/orchestrator/`, `internal/agent/`

**Purpose:** Workflow coordination and agent lifecycle. Runs multi-issue
workflows, manages agent execution, and coordinates sandbox environments.

**May depend on:** service, domain, infrastructure, foundation, content.

**Must not depend on:** cmd, presentation.

### service

**Paths:** `internal/deps/`, `internal/vet/`

**Purpose:** Business logic that requires access to both domain models and
infrastructure services. Dependency analysis and architecture validation live
here because they combine pure business rules with external service calls
(e.g. GitHub API).

**May depend on:** domain, infrastructure, foundation, content.

**Must not depend on:** cmd, orchestration, presentation.

### presentation

**Paths:** `internal/dashboard/`

**Purpose:** Web dashboard for monitoring runs. Consumes domain models for
display but does not orchestrate workflows or call external services directly.

**May depend on:** domain, foundation.

**Must not depend on:** cmd, orchestration, service, infrastructure, content.

### domain

**Paths:** `internal/analysis/`, `internal/detect/`, `internal/dialogue/`,
`internal/doctor/`, `internal/patterns/`, `internal/punchlist/`,
`internal/quality/`, `internal/rundata/`

**Purpose:** Pure business logic, data models, and validation rules. These
packages define the core concepts of the system without depending on external
services or infrastructure.

**May depend on:** foundation, content.

**Must not depend on:** cmd, orchestration, service, infrastructure,
presentation.

### infrastructure

**Paths:** `internal/github/`, `internal/lock/`, `internal/sandbox/`,
`internal/pypi/`, `internal/agent/runner/`

**Purpose:** External service clients and process management. GitHub API
integration, distributed locking, sandbox execution, and package index access.

**May depend on:** foundation.

**Must not depend on:** cmd, orchestration, service, domain, presentation,
content.

### foundation

**Paths:** `internal/config/`, `internal/logging/`, `internal/label/`

**Purpose:** Zero-dependency utilities that any layer may import. Configuration
loading and structured logging.

**May depend on:** (none).

**Must not depend on:** all other layers.

### content

**Paths:** `internal/skills/`, `internal/harness/`, `prompts/`

**Purpose:** Embedded skill definitions, prompt templates, and harness
scaffolding templates. These are static content with no runtime dependencies on
other packages.

**May depend on:** (none).

**Must not depend on:** all other layers.

## Cross-cutting Concerns

**Error handling:** Errors are returned, not panicked. Wrap errors with context
using `fmt.Errorf("context: %w", err)`.

**Configuration:** All configuration flows through `internal/config/`. Packages
must not read environment variables or files directly — they receive
configuration as function parameters or struct fields.

**Logging:** Use `internal/logging/` for structured logging. Do not use the
standard library `log` package directly.
