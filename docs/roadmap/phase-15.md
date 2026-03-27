## Phase 15: Server Mode & Centralized Operation

**Goal**: `godark` can run as a centralized service orchestrating agent work
across many repos, while preserving the local CLI-first workflow for
individual developers. The same core engine powers both modes. Designed for
org-scale deployment where hundreds of developers across hundreds of
microservices need shared visibility, centralized scheduling, and
service-account auth.

**Milestone**: `Phase 15` | **Label**: `phase-15`

### Design principle: same engine, two frontends
- The orchestrator, agent loop, review cycle, verify pipeline, and sandbox
  execution are mode-agnostic — they don't know or care who invoked them
- `godark.yaml` stays in each repo (config travels with code, not with the
  server)
- Mode is determined by how the engine is invoked (CLI vs. server), not by
  a fork in the core logic

### Pluggable run data storage
- Introduce a `RunStore` interface behind `rundata.Writer` / `rundata.Reader`
- `LocalStore` — current filesystem implementation (default for CLI mode)
- `RemoteStore` — shared storage backend (S3-compatible, database, or
  shared filesystem) for server mode
- CLI mode can optionally push to a remote store for shared visibility
  (`run_store: s3://bucket/godark-runs` in config)
- Dashboard reads from whichever store is configured

### Server mode (`godark serve`)
- HTTP/gRPC API server that accepts run requests and reports status
- Endpoints: trigger run (repo + milestone/issue), query run status, list
  runs, stream logs
- Job queue for dispatching runs to worker nodes (initially in-process,
  later pluggable: Redis, SQS, NATS)
- Composes with Phase 13 concurrency — server manages a pool of workers
  across multiple repos simultaneously
- Health checks, graceful shutdown, and run recovery on restart

### Trigger mechanisms
- API call (CI/CD integration, chatops, internal tooling)
- GitHub webhook listener — trigger on issue label, milestone assignment,
  or scheduled event
- Cron/schedule — periodic sweeps of milestones across configured repos
- CLI remains the local trigger (`godark run` unchanged)

### Multi-repo configuration
- Server config file lists managed repos and their overrides:
  ```yaml
  # godark-server.yaml
  server:
    listen: ":8443"
    run_store: "s3://company-godark/runs"
    auth: github-app  # or personal-token
  repos:
    - org/service-a    # uses repo's own godark.yaml
    - org/service-b
    - org/service-c:
        auto_merge: none           # server-level override
        concurrency.max_workers: 2
  ```
- Per-repo `godark.yaml` is authoritative for project-specific config
  (prompts, architecture, conventions)
- Server config provides org-level defaults and overrides (merge policy,
  concurrency limits, risk thresholds)

### Auth model
- CLI mode: developer's personal tokens (current behavior, unchanged)
- Server mode: GitHub App installation (per-org, scoped permissions)
- API keys for external triggers (CI/CD, chatops)
- Per-repo permission scoping — the GitHub App's installation permissions
  limit which repos the server can touch

### Shared dashboard
- Same dashboard code, served by `godark serve` instead of `godark status`
- Aggregates runs across all repos and teams
- Team/repo filtering, org-wide quality metrics
- Role-based views: developer sees their repos, platform team sees
  everything
- Composes with Phase 11 analysis — cross-repo trend data becomes
  meaningful at org scale

### CLI ↔ server interop
- `godark run` can optionally delegate to a running server instead of
  executing locally (`server: https://godark.internal` in config)
- `godark status` can point at the shared dashboard
- Developers can still run fully local for experimentation and testing
- Local runs can push results to the shared store for visibility

**Issues**: TBD

**Planning doc**: `docs/planning/phase-15-server-mode.md`

