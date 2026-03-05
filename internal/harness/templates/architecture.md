# Architecture

<!-- Guidance: Describe the high-level layers of this project. Each layer
     should map to one or more entries in architecture.json. Explain what
     belongs in each layer, what it may depend on, and what it must not
     depend on. Keep this prose short — the formal definitions live in
     architecture.json. -->

This document describes the architectural layers of the project. Formal layer
definitions (names, descriptions, allowed dependencies) are maintained in
`docs/architecture.json`.

## Layers

<!-- Guidance: Add a subsection per layer. Use the same name as in
     architecture.json. Describe the layer's purpose, responsibilities,
     and dependency constraints in plain English. -->

### Example Layer

<!-- Replace this with your first real layer. -->

**Purpose:** Describe what this layer is responsible for.

**Contains:** What kind of code lives here (e.g. CLI commands, HTTP handlers,
domain logic, persistence adapters).

**Depends on:** List which other layers this layer may import.

**Must not depend on:** List which layers are off-limits (e.g. no importing
infrastructure packages from the domain layer).

## Cross-cutting Concerns

<!-- Guidance: Document concerns that apply across all layers, such as error
     handling strategy, logging conventions, and context propagation. -->
