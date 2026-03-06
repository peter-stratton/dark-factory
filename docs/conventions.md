# Dark Factory — Coding Conventions

> Go CLI tool for orchestrating agentic software development workflows.

---

## Error Handling

Wrap errors with context using `fmt.Errorf` and `%w`. Do not create errors with
`errors.New()` or define custom error types unless there is a strong reason.

Use `errors.Is()` for expected conditions like missing files.

Never silently ignore errors. If an error is intentionally discarded, add a
comment explaining why (e.g. "best-effort; ignore error").

**Pattern**: `fmt.Errorf("doing X: %w", err)`

---

## Logging

Use `log/slog` exclusively. No `fmt.Println` for operational output, no
third-party logging libraries.

Pass the logger as a `*slog.Logger` parameter via constructor or function
signature. Fall back to `slog.Default()` when a nil logger is received.

Use structured fields for all log calls.

**Pattern**: `logger.Info("starting orchestration", "repo", cfg.Repo, "milestone", milestone)`

---

## Testing

Use the standard library `testing` package only. No testify, gomock, or other
test frameworks.

Write individual test functions. Use manual assertions with
`if got != want { t.Errorf(...) }`. Mark test helpers with `t.Helper()`.

Stub external commands (shell, `gh`, `docker`) by replacing package-level
function variables in tests, restoring the original with `defer`.

**Pattern**:
```go
orig := github.CommandRunner
github.CommandRunner = fakeGH(raw)
defer func() { github.CommandRunner = orig }()
```

---

## Naming

Packages are short, lowercase, single-word nouns: `config`, `sandbox`, `lock`.

Files use logical grouping within a package: `agent/implementer.go`,
`agent/launcher.go` — not `agent_implementer.go`.

Exported identifiers use PascalCase. Unexported identifiers use camelCase. Do
not prefix interfaces with `I`.

---

## Dependency Injection

Pass dependencies via constructor injection (`New()` functions) or function
parameters. Do not use DI frameworks (wire, fx, etc.).

Package-level function variables are permitted only as testability seams for
external command execution. All other dependencies must be passed explicitly.

**Pattern**:
```go
func New(repo string, logger *slog.Logger) *Locker { ... }
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, ...) error { ... }
```

---

## Agent-Friendliness Notes

All conventions in this project are designed for agentic development:

- **Explicit over implicit** — no magic resolution, no service locators, no
  annotation scanning. Dependencies are visible in function signatures.
- **Local over global** — dependencies passed as parameters. Package-level
  variables only for external command stubs.
- **Clear boundaries** — packages have narrow contracts visible in a few lines
  of code. See `docs/architecture.md` for layer definitions.
- **Discoverable** — any source file is a valid example of these conventions.
  Agents learn by reading representative files.

No anti-patterns flagged. The project avoids code generation, convention-over-configuration
magic, and implicit runtime behavior.
