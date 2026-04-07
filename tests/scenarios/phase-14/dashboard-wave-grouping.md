# Scenario: dashboard wave grouping

Relates to: Issue #754

## Setup
- A `rundata.Writer` with a run directory
- A dashboard server reading run data via `LoadRun`

## Cases

### Wave data round-trip
- GIVEN 2 wave results written via `WriteWaveResult` with different issue numbers
- WHEN `LoadRun` reads the run directory
- THEN `RunDetail.Waves` contains 2 entries with correct issue numbers and timing

### Dashboard groups issues by wave
- GIVEN a run with 2 waves (wave 1: issues 1,2; wave 2: issues 3,4)
- WHEN the run detail page is rendered
- THEN the HTML contains wave-1 and wave-2 section headings with their respective issues

### Serial run shows no wave grouping
- GIVEN a run with no wave data files (serial mode)
- WHEN the run detail page is rendered
- THEN issues display without wave grouping sections

### Wall-clock savings displayed
- GIVEN a run where 2 concurrent issues each took 5 minutes but wall-clock was 6 minutes
- WHEN the run detail page is rendered
- THEN a savings line shows approximately 4 minutes saved
