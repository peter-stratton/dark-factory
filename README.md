```
     _            _           __            _
  __| | __ _ _ __| | __      / _| __ _  ___| |_ ___  _ __ _   _
 / _` |/ _` | '__| |/ /_____| |_ / _` |/ __| __/ _ \| '__| | | |
| (_| | (_| | |  |   <______|  _| (_| | (__| || (_) | |  | |_| |
 \__,_|\__,_|_|  |_|\_\     |_|  \__,_|\___|\__\___/|_|   \__, |
                                                           |___/
```

A Go CLI that orchestrates autonomous AI agents to implement GitHub issues,
review their own work, and merge — without human intervention.

## How it works

Given a GitHub repo and a milestone, `godark` runs a two-agent development loop:

1. **Fetch** open issues from the milestone, sorted by priority (`p1` → `p2` → `p3` → unlabeled)
2. **Resolve dependencies** — issues declare `Blocked by: #N` or `Depends on: #N` in their body; skip any whose dependencies are still open
3. **Agent 1 (Implementer)** — Claude Code implements the issue, writes unit tests, and opens a PR
4. **Agent 2 (Reviewer)** — a separate Claude Code instance reviews the PR against human-authored scenario specs, generates ephemeral integration tests, and approves or requests changes
5. **Retry loop** — if the reviewer rejects, Agent 1 reads the review comments and pushes fixes (max N retries)
6. **Merge or escalate** — approved PRs are squash-merged; failed PRs are labeled `needs-human-review`
7. **Repeat** — move to the next unblocked issue

## Usage

```
godark run --milestone "Phase 1" --repo owner/repo
godark run --milestone "Phase 1" --repo owner/repo --dry-run
godark run --issue 42 --repo owner/repo

godark vet issues    --repo owner/repo --milestone "Phase 2"
godark vet scenarios --repo owner/repo --milestone "Phase 2"
godark vet roadmap   --repo owner/repo --milestone "Phase 2"

godark init
```

## Building

```bash
go build -o bin/godark ./cmd/godark
go test ./...
```

## Status

See [docs/ROADMAP.md](docs/ROADMAP.md) for the full development roadmap.
