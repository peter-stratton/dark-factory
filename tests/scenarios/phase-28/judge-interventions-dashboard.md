# Scenario: Dashboard rendering of judge interventions

Relates to: Issue #649

## Setup
- `internal/dashboard/` templates render `IssueDetail.JudgeInterventions`
- Run data contains issues with and without judge interventions
- Dashboard handlers load data via the rundata Reader

## Cases

### Intervention displayed on issue detail page
Load the issue detail page for an issue with one judge intervention.
- The page contains a "Judge Interventions" section
- The section shows the rule name, judgment, detail message, and timestamp
- The affected step is displayed

### Multiple interventions rendered in order
Load the issue detail page for an issue with two judge interventions.
- Both interventions are rendered
- They appear in chronological order

### No interventions hides section
Load the issue detail page for an issue with empty `JudgeInterventions`.
- No "Judge Interventions" section is rendered
- No empty container or placeholder shown

### Overview shows indicator for judge-affected issues
Load the run overview page with one judge-affected issue and one unaffected issue.
- The judge-affected issue row has a visual indicator (icon or badge)
- The unaffected issue row does not have the indicator
