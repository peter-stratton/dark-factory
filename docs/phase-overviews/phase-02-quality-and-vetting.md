# Phase 2: Quality & Vetting

The `godark vet` command validates that GitHub issues, scenario specs, and planning docs are structured well enough for autonomous agents to act on them. It was built early in the project so that every subsequent phase's issues could be vetted before agents touched them. The command fetches live data from GitHub, cross-references it against local files, and produces a report with findings at three severity levels: error, warning, and info. Errors cause a non-zero exit code; warnings and infos do not.

---

## Validation Framework and Report Format

Every `godark vet` subcommand produces a `Report` -- a collection of `Finding` values, each with a severity, a location (like `#14` or `phase-2/config-parsing.md`), and a human-readable message. Reports render as aligned tables on stdout or as structured JSON with `--json`.

**Example: table output**

```
$ godark vet issues --repo phs/dark-factory --tag phase-4

error  #29  missing ## Acceptance criteria section
error  #29  missing ## Test cases section
warning  #30  blocker references non-existent issue #999

2 error(s), 1 warning(s), 0 info(s)
```

**Example: JSON output**

```
$ godark vet issues --repo phs/dark-factory --milestone "Phase 4" --json
{
  "findings": [
    {
      "Severity": "error",
      "Message": "missing ## Acceptance criteria section",
      "Location": "#29"
    }
  ],
  "summary": {
    "errors": 1,
    "warnings": 0,
    "infos": 0
  }
}
```

The JSON format is useful for piping into other tools or CI checks. The exit code is 1 when any finding has error severity, 0 otherwise -- so you can gate a pipeline on `godark vet` the same way you gate on `go test`.

---

## Issue Structure Validation (`godark vet issues`)

Checks every open issue in a GitHub milestone for the structural elements that agents need to do useful work. Each issue must have an `## Acceptance criteria` section with checkbox items (`- [ ]`) and a `## Test cases` section with named entries (`- **Name**:`). Without these, an agent has no clear definition of done and no test plan to validate against.

Beyond structure, the validator also checks blocker references. Lines like `Blocked by: #14, #15` are parsed and each referenced issue number is verified to actually exist in the repo. Malformed blocker notation (missing the colon) gets a warning.

If the milestone title follows the `Phase N` convention, each issue is also checked for a matching `phase-N` label.

**Example: catching a missing acceptance criteria section**

You write a new issue for Phase 5, assign it to the milestone, but forget the acceptance criteria:

```
$ godark vet issues --repo phs/dark-factory --milestone "Phase 5"

error  #36  missing ## Acceptance criteria section
error  #36  missing ## Test cases section
warning  #37  blocker references non-existent issue #35
warning  #38  missing phase label "phase-5"

2 error(s), 2 warning(s), 0 info(s)
```

You fix issues #36 and #38, re-run, and get:

```
$ godark vet issues --repo phs/dark-factory --milestone "Phase 5"

warning  #37  blocker references non-existent issue #35

0 error(s), 1 warning(s), 0 info(s)
```

Exit code is 0 now. The warning about #35 is informational -- maybe that issue was closed or renumbered. The milestone is ready for agent execution.

---

## Scenario Spec Validation (`godark vet scenarios`)

Validates the markdown scenario spec files that tell the reviewer agent what to test. Each spec must have a `# Scenario:` title, a `Relates to: Issue #N` line, `## Setup` and `## Cases` sections, and every `### Case` must include at least one bullet-point outcome (`- `).

The validator also cross-references in both directions: issue refs in `Relates to:` lines are checked against the repo (do those issues actually exist?), and milestone issues are checked for coverage (does every issue have at least one scenario spec pointing at it?).

**Example: validating scenario specs for a milestone**

```
$ godark vet scenarios --repo phs/dark-factory --tag phase-4 --scenario-dir tests/scenarios/phase-4/

error    phase-4/sandbox-auth.md      missing ## Setup section
error    phase-4/sandbox-auth.md      case "Token forwarding" has no outcome bullets
warning  phase-4/sandbox-auth.md      Relates to references non-existent issue #999
warning  #31                          milestone issue has no matching scenario spec

2 error(s), 2 warning(s), 0 info(s)
```

The `--scenario-dir` flag defaults to `tests/scenarios/` but you can point it at a phase-specific subdirectory. You can also run scenario validation without a milestone -- just omit `--milestone` and `--tag` to check file structure only, without the coverage cross-reference.

---

## Roadmap Validation (`godark vet roadmap`)

Checks planning docs (markdown files in a planning directory) against the live GitHub milestone. It looks for `## Issue N:` or `## Issue #N:` headings in the planning docs and cross-references them against the milestone's issues in both directions:

- **Orphaned issues**: a milestone issue that doesn't appear in any planning doc gets a warning. This catches issues that were created on GitHub but never written up in the planning doc.
- **Phantom references**: a planning doc references an issue number that doesn't exist in the repo. This catches stale references after issues are renumbered or deleted.
- **Label mismatches**: an issue in a "Phase 4" milestone has a `phase-3` label. This is an error, not a warning, because it will confuse the orchestrator's filtering.

**Example: catching orphaned and phantom issues**

```
$ godark vet roadmap --repo phs/dark-factory --milestone "Phase 4" --planning-dir docs/planning/

warning  phase-4-agent-execution.md  references non-existent issue #28 (phantom)
warning  #32                         milestone issue not in any planning doc (orphaned)
error    #33                         has label "phase-3" but milestone is "Phase 4"

1 error(s), 2 warning(s), 0 info(s)
```

The phantom warning tells you that issue #28 was probably deleted or merged into another issue -- update the planning doc. The orphan warning tells you that #32 was added to the milestone but never documented in the planning doc. The label error is a hard failure: fix the label before proceeding.

---

## Shared Flags and Config Resolution

All three subcommands share a consistent flag set:

| Flag | Purpose |
|---|---|
| `--repo owner/repo` | GitHub repository to validate against |
| `--milestone "Phase N"` | Exact milestone title |
| `--tag phase-n` | Shorthand that resolves to a milestone title via the GitHub API |
| `--json` | Output findings as JSON instead of a table |
| `--config godark.yaml` | Config file path (repo is read from config if `--repo` is omitted) |

The `--tag` and `--milestone` flags are mutually exclusive. Using `--tag phase-4` is equivalent to looking up which milestone has the tag "phase-4" and passing its title to `--milestone`. The repo can come from either the `--repo` flag or the `repo:` field in `godark.yaml`, so in a project that already has a config file, you can just run:

```
$ godark vet issues --tag phase-4
```
