# Scenario: Resource stats for --no-sandbox host mode

Relates to: Issue #548

## Setup
- `internal/agent/runner/` with host-mode execution path
- Agent process that can be run in `--no-sandbox` mode
- Stubbed `syscall.Getrusage` for controlled test values

## Cases

### Host mode captures resource stats
Run an agent in host mode (`--no-sandbox`). Stub `Getrusage` to return `Maxrss = 204800` (kilobytes on macOS) and `Utime + Stime = 3 seconds`.
- `StepResult.PeakMemoryBytes` equals `209715200` (normalized to bytes)
- `StepResult.CPUNanoseconds` equals `3000000000`

### Getrusage failure returns zero values
Run an agent in host mode. Stub `Getrusage` to return an error.
- `StepResult.PeakMemoryBytes` equals `0`
- `StepResult.CPUNanoseconds` equals `0`
- No error returned from the execution
- Warning logged about getrusage failure

### Platform normalization on macOS
Stub `Getrusage` with `Maxrss = 1024` (kilobytes on macOS).
- `PeakMemoryBytes` equals `1048576` (1024 * 1024, converted to bytes)

### Platform normalization on Linux
Stub `Getrusage` with `Maxrss = 1048576` (bytes on Linux).
- `PeakMemoryBytes` equals `1048576` (no conversion needed)

### Resource fields flow to stats DB
Run an agent in host mode with resource capture. Finalize the run.
- `step_results` row has non-zero `peak_memory_bytes`
- `step_results` row has non-zero `cpu_nanoseconds`
- Values match what `Getrusage` returned (after normalization)
