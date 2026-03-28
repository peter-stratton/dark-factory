# Scenario: Dashboard trace ID display on issue detail page

Relates to: Issue #703

## Setup
- A run data directory with two issues:
  - Issue #10 with `Outcome.TraceID` set to "aaaabbbb-cccc-4ddd-eeee-ffffffffffff" and step results containing the same trace ID
  - Issue #20 with no trace ID (simulating a pre-Phase-32 run)
- The dashboard server running and serving the issue detail pages

## Cases

### Trace ID displayed in issue metadata
- GIVEN issue #10 has a trace ID in its outcome
- WHEN the issue detail page is loaded at `/runs/org/repo/timestamp/issues/10`
- THEN the page contains the text "aaaabbbb-cccc-4ddd-eeee-ffffffffffff"
- THEN it appears in the metadata section near the status and PR link

### Trace ID styled as monospace with copy button
- GIVEN issue #10 has a trace ID
- WHEN the issue detail page is loaded
- THEN the trace ID is rendered in a monospace font
- THEN a clickable copy-to-clipboard element is present adjacent to the trace ID

### Old runs without trace ID show no trace row
- GIVEN issue #20 has no trace ID (empty string)
- WHEN the issue detail page is loaded at `/runs/org/repo/timestamp/issues/20`
- THEN there is no "Trace ID" row in the metadata section
- THEN no empty placeholder or label is shown

### Timeline steps carry trace ID
- GIVEN issue #10 has step results each with the same trace ID
- WHEN the issue detail page timeline is rendered
- THEN each `TimelineStepView` includes the trace ID value
