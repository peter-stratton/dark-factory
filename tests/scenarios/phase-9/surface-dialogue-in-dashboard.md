# Scenario: Surface agent dialogue in dashboard

Relates to: Issue #151

## Setup
- The dashboard server is running locally via `godark status`
- A run directory exists with at least one issue that has a `dialogue.json` file
- A second issue exists without a `dialogue.json` file for comparison
- No external services required (dashboard reads from local run data)

## Cases

### Dialogue section rendered when entries exist
Navigate to the issue detail page for an issue with dialogue entries.
- The page contains a "Dialogue" section
- Each entry shows the role (implementer or reviewer)
- Each entry shows the round number
- Entry bodies are present in expandable elements

### Dialogue section hidden when no entries
Navigate to the issue detail page for an issue without dialogue entries.
- The page does not contain a "Dialogue" section
- Existing timeline and punchlist sections are still displayed

### Implementer and reviewer visually distinct
Navigate to the issue detail page with both implementer and reviewer entries.
- Implementer entries have a different visual style (border color or icon)
  than reviewer entries

### Bodies are expandable
Navigate to the issue detail page with dialogue entries.
- Each dialogue body is wrapped in a `<details>` element
- Bodies are collapsed by default

### Multiple rounds render in order
Navigate to the issue detail page with three rounds of dialogue (six entries).
- Entries are displayed in chronological order (round 1 impl, round 1 review,
  round 2 impl, round 2 review, round 3 impl, round 3 review)
