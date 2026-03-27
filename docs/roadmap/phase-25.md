## Phase 25: Docker Socket Mount & Compose Lifecycle ✅

**Goal**: Projects with `docker-compose` test infrastructure can run integration
tests inside the sandbox. godark manages the compose lifecycle (up before agent,
down after) via host Docker socket mount. The agent runs tests against
already-running infrastructure without managing containers itself.

**Milestone**: `Phase 25` | **Label**: `phase-25`

- Add `docker_compose` config block to `godark.yaml` (`file`, `project_name`)
- Mount host Docker socket into sandbox container when `docker_compose` is
  configured
- Install Docker CLI in sandbox image when socket mount is enabled
- Run `docker-compose up -d` before agent execution starts
- Run `docker-compose down` in deferred cleanup (even on crash/timeout)
- Unique project names per run to avoid port/name collisions (prefix with run
  ID or issue number)
- Forward `required_env` to compose containers (database URLs, emulator hosts,
  etc.)
- Update `godark doctor` to check Docker socket accessibility

**Issues**: #556–#564

**Planning doc**: `docs/planning/phase-25-docker-socket-mount-and-compose-lifecycle.md`

