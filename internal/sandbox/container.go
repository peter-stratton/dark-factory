package sandbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RunOpts configures a container run.
type RunOpts struct {
	Image             string
	Cmd               []string
	Env               map[string]string
	Timeout           time.Duration
	Mount             string // host:container bind mount
	MountDockerSocket bool   // mount /var/run/docker.sock into the container
	// LogCallback, when non-nil, is called for each line emitted by the
	// container while it is running. Lines are delivered via a buffered
	// channel; if the callback is slow, excess lines are dropped rather
	// than backpressuring the container process. The callback must be
	// safe to call from a goroutine and must not block; a blocking
	// callback may cause RunContainer to hang until the context is
	// cancelled.
	LogCallback func(line string)
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
//
// Memory reporting differs between cgroup versions:
//   - cgroups v1: memory_stats.max_usage (lifetime high-water mark)
//   - cgroups v2: memory_stats.usage only (no max_usage field)
//
// The poller tracks the high-water mark of whichever field is available,
// preferring max_usage when present, falling back to usage for cgroups v2.
type containerStats struct {
	MemoryStats struct {
		MaxUsage int64 `json:"max_usage"`
		Usage    int64 `json:"usage"`
	} `json:"memory_stats"`
	CpuStats struct {
		CpuUsage struct {
			TotalUsage int64 `json:"total_usage"`
		} `json:"cpu_usage"`
	} `json:"cpu_stats"`
}

// StatsInterval controls how often the background goroutine polls the Docker
// stats API while a container is running. Replaceable for testing.
var StatsInterval = 5 * time.Second

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

// statsPoller polls the Docker stats API in a background goroutine and tracks
// peak memory and CPU usage. Call stop() to signal the goroutine, wait for it
// to finish, and retrieve the high-water marks.
type statsPoller struct {
	done chan struct{}
	wg   sync.WaitGroup
	mu   sync.Mutex
	mem  int64
	cpu  int64
}

func startStatsPoller(containerName string) *statsPoller {
	sp := &statsPoller{done: make(chan struct{})}
	sp.wg.Add(1)
	go sp.run(containerName)
	return sp
}

func (sp *statsPoller) run(containerName string) {
	defer sp.wg.Done()
	statsURL := "http://localhost/containers/" + containerName + "/stats?stream=false"
	ticker := time.NewTicker(StatsInterval)
	defer ticker.Stop()
	sp.poll(statsURL) // immediate first sample
	for {
		select {
		case <-sp.done:
			sp.poll(statsURL) // one last sample before exiting
			return
		case <-ticker.C:
			sp.poll(statsURL)
		}
	}
}

func (sp *statsPoller) poll(url string) {
	out, err := CommandRunner("curl", "--silent", "--unix-socket", "/var/run/docker.sock", url)
	if err != nil {
		return
	}
	var s containerStats
	if json.Unmarshal(out, &s) != nil {
		return
	}
	// Prefer max_usage (cgroups v1 lifetime high-water mark) when available;
	// fall back to usage (cgroups v2 current usage) and let the poller track
	// the peak across samples.
	memSample := s.MemoryStats.MaxUsage
	if memSample == 0 {
		memSample = s.MemoryStats.Usage
	}
	sp.mu.Lock()
	if memSample > sp.mem {
		sp.mem = memSample
	}
	if s.CpuStats.CpuUsage.TotalUsage > sp.cpu {
		sp.cpu = s.CpuStats.CpuUsage.TotalUsage
	}
	sp.mu.Unlock()
}

func (sp *statsPoller) stop() (peakMem, peakCPU int64) {
	close(sp.done)
	sp.wg.Wait()
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return sp.mem, sp.cpu
}

// startLogStreamer spawns producer/consumer goroutines that tail the named
// container's logs and forward each line to callback. If callback is nil the
// returned WaitGroup is already done. Lines are forwarded via a buffered
// channel so a slow callback drops lines rather than backpressuring the
// docker logs process.
func startLogStreamer(ctx context.Context, name string, callback func(string), logger *slog.Logger) *sync.WaitGroup {
	var wg sync.WaitGroup
	if callback == nil {
		return &wg
	}
	ch := make(chan string, 256)
	wg.Add(2)
	// Producer: tail container stdout via docker logs --follow.
	go func() {
		defer wg.Done()
		defer close(ch)
		rc, wait, err := LogFollower(ctx, name)
		if err != nil {
			return
		}
		scanner := bufio.NewScanner(rc)
		// Raise the per-line limit to 1 MB to handle realistic Docker log output
		// that can exceed the default 64 KB limit. Without this, a long line
		// causes Scan() to return false with ErrTooLong, silently stopping
		// streaming mid-run.
		scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			select {
			case ch <- line:
			default: // drop line if consumer is behind
			}
		}
		if err := scanner.Err(); err != nil {
			logger.Warn("log streamer scanner error", "container", name, "error", err)
		}
		_ = wait()
	}()
	// Consumer: deliver lines to the caller's callback. The select on
	// ctx.Done() ensures RunContainer can always return when the context
	// is cancelled, even if callback blocks unexpectedly.
	go func() {
		defer wg.Done()
		for line := range ch {
			select {
			case <-ctx.Done():
				return
			default:
				callback(line)
			}
		}
	}()
	return &wg
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
	if opts.MountDockerSocket {
		createArgs = append(createArgs, "-v", "/var/run/docker.sock:/var/run/docker.sock")
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

	// 3. Start log-streaming goroutine if a callback was provided.
	logWg := startLogStreamer(ctx, name, opts.LogCallback, logger)

	// 4. Start background stats poller. The Docker stats API only returns
	// non-zero cgroup counters while the container is running; once it exits
	// the kernel tears down the cgroup and the counters reset to zero.
	// We poll periodically and keep the high-water marks.
	sp := startStatsPoller(name)

	// 5. docker wait (with context timeout)
	result := &RunResult{}
	waitStart := time.Now()
	out, err = CommandRunnerWithContext(ctx, "docker", "wait", name)
	wallElapsed := time.Since(waitStart)

	// Signal the stats poller to stop and take a final sample.
	peakMem, peakCPU := sp.stop()

	// Wait for the log-streaming goroutines to finish draining. The
	// exec.CommandContext above will have killed the docker logs process
	// when ctx was cancelled, so the producer goroutine exits promptly.
	logWg.Wait()

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

	// 6. docker logs (separate stdout/stderr)
	stdoutBytes, stderrBytes, err := SplitRunner("docker", "logs", name)
	if err != nil {
		logger.Warn("failed to retrieve container logs", "name", name, "error", err)
	}
	result.Stdout = string(stdoutBytes)
	result.Stderr = string(stderrBytes)

	// 7. Apply collected resource stats.
	result.PeakMemoryBytes = peakMem
	result.CPUNanoseconds = peakCPU

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

// LogFollower starts "docker logs --follow --timestamps <name>" and returns a
// ReadCloser of its stdout, a wait function, and any startup error.
// Replaceable for testing.
var LogFollower = func(ctx context.Context, name string) (io.ReadCloser, func() error, error) {
	cmd := exec.CommandContext(ctx, "docker", "logs", "--follow", "--timestamps", name)
	// Combine stdout and stderr so the judge sees tool audit lines (which the
	// agent runner writes to stderr) alongside assistant text (stdout).
	// Use os.Pipe (kernel-buffered) rather than io.Pipe (synchronous) to avoid
	// deadlocks when docker writes to both streams concurrently.
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, nil, fmt.Errorf("creating log pipe: %w", err)
	}
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return nil, nil, err
	}
	// Close the write end when the command exits so the reader sees EOF.
	wait := func() error {
		err := cmd.Wait()
		pw.Close()
		return err
	}
	return pr, wait, nil
}
