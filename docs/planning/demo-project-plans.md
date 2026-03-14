# Demo Project Plans

Template repos under `peter-stratton/` that users can create their own repo from, then follow tutorials demonstrating real-world Dark Factory usage.

Each repo is a GitHub template repo. Users click "Use this template" to get their own copy with a clean commit history.

## Repos

| Repo | Language/Framework | Toolchain |
|------|--------------------|-----------|
| `dark-factory-demo-go` | Go standard library HTTP server | `go build`, `go test`, `golangci-lint` |
| `dark-factory-demo-python` | Python / FastAPI | `pytest`, `ruff`, `poetry` or `uv` |
| `dark-factory-demo-node` | Node.js / TypeScript / Express | `npm`, `eslint`, `vitest`, `tsc` |

## Scenarios

| # | Scenario | Description |
|---|----------|-------------|
| 1 | **Basic PR workflow** | Dark Factory creates a branch, opens a PR, gets it merged |
| 2 | **Manual vs auto merge** | When to require human approval vs let it land automatically |
| 3 | **Watch mode** | Iterating on changes with `--watch`, seeing live feedback |
| 4 | **Mechanical verification: lint** | Pre-merge linting gates, how Dark Factory handles failures and retries |
| 5 | **Mechanical verification: test** | Test suites as merge gates |
| 6 | **Mechanical verification: build** | Compiled languages where build must pass before merge |
| 7 | **CI pipeline integration** | GitHub Actions required status checks, Dark Factory waiting on CI |
| 8 | **Branch protection rules** | Required reviews, status checks, how Dark Factory works within constraints |
| 9 | **Monorepo considerations** | Multiple packages/services in one repo |
| 10 | **Dependency updates** | Adding/upgrading deps, lock file changes |
| 11 | **Database migrations** | Schema changes alongside code |
| 12 | **Hotfix workflow** | Fast-tracking a fix to production |
| 13 | **Multi-step refactoring** | Breaking a large change into sequential PRs |
| 14 | **Full godark setup walkthrough** | Walking through the complete setup: `godark init` → `/godark-define-architecture` → `/godark-define-conventions` → `/godark-configure-project` → `/godark-harness-claude-md` |

## Scenario Matrix

Each repo includes the basic PR workflow (scenario 1) as a starting point. Users are expected to pick one demo and work through it end-to-end.

| Repo | Scenarios | Rationale |
|------|-----------|-----------|
| **Go** | 1, 2, 3, 5, 6, 7 | Great starter — fast builds, simple CI, shows basics well. Build verification is natural for compiled languages. |
| **Python** | 1, 3, 4, 5, 10, **14** | Linting is critical in dynamic languages, dep management is common pain. Also the best fit for the full godark setup walkthrough since Python doesn't have an established project structure — shows how godark's opinionated setup adds clear value. |
| **Node/TS** | 1, 4, 5, 7, 8, 9 | Most users will relate to this stack. Good for demonstrating CI + branch protection + monorepo patterns. |

## Repo Settings (applied to all template repos)

- **Template:** enabled
- **Wiki:** disabled
- **Projects:** disabled
- **Squash merge only** (merge commit and rebase disabled)
- **Delete branch on merge:** enabled
- **Issues:** enabled

## User Flow

1. User browses the demo index (on docs site or main repo README)
2. Picks a demo project that matches their preferred language
3. Clicks "Use this template" → creates their own repo
4. Clones locally and follows the tutorial
5. Each tutorial walks through the listed scenarios step by step, with Dark Factory pushing to their repo

## Notes

- GitHub template repos copy files only — not issues, PRs, branches, settings, or git history
- Any repo-level settings needed for tutorials (branch protection, required status checks) must be documented as setup steps in the tutorial
- The `dark-factory-demo-go` repo already exists: https://github.com/peter-stratton/dark-factory-demo-go
