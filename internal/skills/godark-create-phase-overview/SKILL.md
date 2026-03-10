---
name: godark-create-phase-overview
description: Generate a practical overview with real-world examples for a completed roadmap phase
argument-hint: "<phase-number>"
disable-model-invocation: true
---

# Create Phase Overview

Generate a practical overview document for a roadmap phase. The overview explains
what was built and illustrates each feature with concrete, real-world examples
grounded in the actual codebase -- not hypothetical descriptions of what the code
might do, but examples drawn from what it actually does.

These overviews are most valuable after a phase is implemented, when they can
reference real code, real config fields, and real command output.

## Steps

1. **Read the roadmap** -- Read `docs/ROADMAP.md` and find the specified phase.
   Extract the phase name, goal, milestone, and feature list. If the phase is
   not marked with a checkmark, warn the user that the phase appears incomplete
   and ask whether to proceed anyway.

2. **Read the planning doc** -- Check `docs/planning/` for a planning doc
   matching the phase (e.g., `phase-7-review-quality-and-dashboard.md`). If one
   exists, read it for detailed specs, constraints, and acceptance criteria.

3. **Explore the codebase** -- For each feature in the phase, find and read the
   actual code that implements it. Follow imports, check config structs, read
   prompt templates, look at command definitions. The goal is to understand what
   was really built, not just what was planned. To find the right places to look:
   - Read `docs/architecture.json` (if it exists) for layer definitions and
     their `paths` — these tell you where each layer's code lives.
   - Read `godark.yaml` for config structure, build commands, and module layout.
   - If neither file provides enough guidance, scan the project's top-level
     directories and source tree to discover where code lives.

4. **Read existing overviews** -- Check `docs/phase-overviews/` for existing
   files to match the established tone, depth, and formatting conventions. If
   overviews exist, follow their style closely.

5. **Write the overview** -- Create `docs/phase-overviews/phase-NN-<slug>.md`
   using the format below. Use zero-padded two-digit phase numbers (e.g.,
   `phase-01`, `phase-12`). The slug should match the phase name from the
   roadmap in kebab-case.

6. **Update the README** -- Check `README.md` for a "Phase overviews" table or
   similar section that lists phase overview documents. If one exists, add a row
   for the new phase matching the existing format. If no such section exists,
   skip this step.

7. **Print summary** -- Show the file path and feature count.

## Format

```markdown
# Phase N: <Phase Name>

<One paragraph summary: what this phase does, why it matters, and how it fits
into the larger project. Direct and engaging -- no filler.>

---

## <Feature Name>

**What it does:** <1-2 sentences describing the feature.>

**Example:** <A concrete, real-world scenario showing how a user experiences
this feature. Include actual commands, config snippets, log output, data
structures, or workflows as appropriate. Draw from real code -- show actual
config field names, actual CLI flags, actual log messages.>

---

## <Feature Name>
...
```

## Rules

- Every feature section must have both "What it does" and "Example" subsections.
- Examples must be grounded in the actual codebase. Use real config field names,
  real CLI flags, real struct names, real file paths. Do not invent plausible-
  sounding names -- read the code and use what is actually there.
- Show realistic scenarios: a team adopting godark, a developer running a
  command, an agent processing an issue. Give the example enough narrative
  context that a reader understands the situation, not just the syntax.
- Include code blocks for commands, config, output, and data structures.
- Do not document features that were planned but not implemented. If the code
  doesn't exist, the feature doesn't go in the overview.
- Do not add features from other phases. Each overview covers exactly one phase.
- Do not modify the roadmap or any other files besides the overview and README.
- If an overview already exists for this phase, ask the user whether to replace
  it or skip.
- No emojis.
- Keep the summary paragraph tight -- 3-5 sentences max.
- Aim for 150-300 lines per overview. Long enough to be genuinely useful, short
  enough to read in one sitting.
