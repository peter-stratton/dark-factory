## Phase 1: Skeleton + Orchestration ✅

**Goal**: `godark run --milestone "Phase 1" --repo owner/repo --dry-run` works
end-to-end. Fetches issues, resolves dependencies, sorts by priority, and
prints the execution plan. No agent execution in this phase.

**Milestone**: `Phase 1` | **Label**: `phase-1`

- Project scaffold and CLI skeleton (Cobra, subcommand stubs)
- YAML config parsing with CLI flag overrides
- GitHub issue fetching with priority sorting (p1 → p2 → p3 → unlabeled)
- Dependency resolution from issue bodies (`Blocked by`, `Depends on`)
- Structured logging (JSON file + human-readable stdout)
- Orchestration loop with dry-run mode
- CLAUDE.md and scenario specs for Phase 2 validation
- `godark init` command — installs skills and default config into a project
- Planning skills: `/godark-create-milestone`, `/godark-create-planning-doc`, `/godark-create-issues`, `/godark-create-scenarios`

**Issues**: #1–#7 (all closed), init + skills added post-milestone

