package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// RunOpts configures a container run.
type RunOpts struct {
	Image   string
	Cmd     []string
	Env     map[string]string
	Timeout time.Duration
	Mount   string // host:container bind mount
}

// RunResult holds the outcome of a container run.
type RunResult struct {
	ExitCode       int
	Stdout         string
	Stderr         string
	TimedOut       bool
	PeakMemoryBytes int64
	CPUNanoseconds  int64
}

// containerStats is the minimal shape of the Docker stats API JSON used to
// extract resource usage stats after a container stops. Field names match
// the snake_case schema returned by GET /containers/{id}/stats?stream=false.
type containerStats struct {
	MemoryStats struct {
		MaxUsage int64 `json:"max_usage"`
	} `json:"memory_stats"`
	CpuStats struct {
		CpuUsage struct {
			TotalUsage int64 `json:"total_usage"`
		} `json:"cpu_usage"`
	} `json:"cpu_stats"`
}

// SplitRunner executes a command and returns stdout and stderr separately.
// Replaceable for testing.
var SplitRunner = func(name string, args ...string) (stdout, stderr []byte, err error) {
	cmd := exec.Command(name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.Bytes(), errBuf.Bytes(), err
}

// generateContainerName returns a unique container name with a random suffix.
func generateContainerName() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return fmt.Sprintf("godark-%s", string(b))
}

// RunContainer creates a Docker container from the given image, runs a command
// inside it, captures stdout/stderr and exit code, enforces a timeout, and
// always cleans up the container.
func RunContainer(ctx context.Context, opts RunOpts, logger *slog.Logger) (*RunResult, error) {
	name := generateContainerName()
	logger.Info("running container", "name", name, "image", opts.Image)

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Always clean up the container.
	defer func() {
		logger.Info("removing container", "name", name)
		_, _ = CommandRunner("docker", "rm", "-f", name)
	}()

	// 1. docker create
	createArgs := []string{"create", "--name", name}
	for k, v := range opts.Env {
		createArgs = append(createArgs, "-e", k+"="+v)
	}
	if opts.Mount != "" {
		createArgs = append(createArgs, "-v", opts.Mount)
	}
	createArgs = append(createArgs, opts.Image)
	createArgs = append(createArgs, opts.Cmd...)

	out, err := CommandRunner("docker", createArgs...)
	if err != nil {
		return nil, fmt.Errorf("docker create: %w\noutput: %s", err, out)
	}
	containerID := strings.TrimSpace(string(out))
	logger.Info("container created", "id", containerID)

	// 2. docker start
	out, err = CommandRunner("docker", "start", name)
	if err != nil {
		return nil, fmt.Errorf("docker start: %w\noutput: %s", err, out)
	}

	// 3. docker wait (with context timeout)
	result := &RunResult{}
	waitStart := time.Now()
	out, err = CommandRunnerWithContext(ctx, "docker", "wait", name)
	wallElapsed := time.Since(waitStart)
	if ctx.Err() != nil || (opts.Timeout > 0 && wallElapsed > opts.Timeout+30*time.Second) {
		// Timed out or cancelled — stop the container. The wall-clock check
		// is a safety net for cases where the context deadline is unreliable
		// (e.g. macOS sleep suspends the monotonic clock used by timers but
		// wall-clock time advances, so docker wait can return successfully
		// after the intended timeout has long passed).
		logger.Warn("container timed out, stopping", "name", name, "wall_elapsed", wallElapsed)
		result.TimedOut = true
		_, _ = CommandRunner("docker", "stop", name)
	} else if err != nil {
		return nil, fmt.Errorf("docker wait: %w\noutput: %s", err, out)
	} else {
		trimmed := strings.TrimSpace(string(out))
		code, parseErr := strconv.Atoi(trimmed)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing exit code %q: %w", trimmed, parseErr)
		}
		result.ExitCode = code
	}

	// 4. docker logs (separate stdout/stderr)
	stdoutBytes, stderrBytes, err := SplitRunner("docker", "logs", name)
	if err != nil {
		logger.Warn("failed to retrieve container logs", "name", name, "error", err)
	}
	result.Stdout = string(stdoutBytes)
	result.Stderr = string(stderrBytes)

	// 5. Docker stats API (best-effort resource stats — must run before deferred rm -f).
	// Uses the Docker socket directly via curl because docker inspect does not expose
	// runtime memory/CPU statistics; those come from GET /containers/{id}/stats?stream=false.
	statsURL := "http://localhost/containers/" + name + "/stats?stream=false"
	statsOut, statsErr := CommandRunner("curl", "--silent", "--unix-socket", "/var/run/docker.sock", statsURL)
	if statsErr != nil {
		logger.Warn("container stats api failed", "name", name, "error", statsErr)
	} else {
		var stats containerStats
		if jsonErr := json.Unmarshal(statsOut, &stats); jsonErr != nil {
			logger.Warn("failed to parse container stats", "name", name, "error", jsonErr)
		} else {
			result.PeakMemoryBytes = stats.MemoryStats.MaxUsage
			result.CPUNanoseconds = stats.CpuStats.CpuUsage.TotalUsage
		}
	}

	logger.Info("container finished", "name", name, "exit_code", result.ExitCode, "timed_out", result.TimedOut)
	return result, nil
}

// CommandRunnerWithContext executes a command that respects context cancellation.
// Unlike CommandRunner, this monitors the context and kills the process if the
// context is cancelled. Replaceable for testing.
var CommandRunnerWithContext = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}
