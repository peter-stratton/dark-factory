# Scenario: Capture container resource stats via docker inspect

Relates to: Issue #543

## Setup
- `internal/sandbox/container.go` with `RunContainer` function
- Stubbed `CommandRunner` returning controlled `docker inspect` JSON
- Stubbed `CommandRunnerWithContext` for `docker wait`

## Cases

### Inspect returns valid resource stats
Run a container that exits successfully. Stub `docker inspect` to return JSON with `MemoryStats.MaxUsage = 104857600` and `CpuStats.CpuUsage.TotalUsage = 5000000000`.
- `RunResult.PeakMemoryBytes` equals `104857600`
- `RunResult.CPUNanoseconds` equals `5000000000`

### Inspect returns malformed JSON
Run a container that exits successfully. Stub `docker inspect` to return `{invalid`.
- `RunResult.PeakMemoryBytes` equals `0`
- `RunResult.CPUNanoseconds` equals `0`
- No error returned from `RunContainer`
- Warning logged about parse failure

### Inspect command fails
Run a container that exits successfully. Stub `docker inspect` to return an error.
- `RunResult.PeakMemoryBytes` equals `0`
- `RunResult.CPUNanoseconds` equals `0`
- No error returned from `RunContainer`
- Warning logged about inspect failure

### Inspect after timeout
Run a container that times out. Stub `docker inspect` to return valid stats.
- `RunResult.TimedOut` is true
- `RunResult.PeakMemoryBytes` is populated (inspect runs on stopped container)
- `RunResult.CPUNanoseconds` is populated

### Inspect after OOM kill
Run a container that exits with code 137 (OOM). Stub `docker inspect` to return valid stats with high memory usage.
- `RunResult.ExitCode` equals `137`
- `RunResult.PeakMemoryBytes` is populated
- `RunResult.CPUNanoseconds` is populated
