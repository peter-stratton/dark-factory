package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/peter-stratton/dark-factory/internal/agent"
	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/ghapp"
	"github.com/peter-stratton/dark-factory/internal/github"
	"github.com/peter-stratton/dark-factory/internal/label"
	"github.com/peter-stratton/dark-factory/internal/lock"
	"github.com/peter-stratton/dark-factory/internal/logging"
	"github.com/peter-stratton/dark-factory/internal/notify"
	"github.com/peter-stratton/dark-factory/internal/orchestrator"
	"github.com/peter-stratton/dark-factory/internal/progress"
	"github.com/peter-stratton/dark-factory/internal/punchlist"
	"github.com/peter-stratton/dark-factory/internal/rundata"
	"github.com/peter-stratton/dark-factory/internal/sandbox"
	"github.com/peter-stratton/dark-factory/internal/stats"
	"github.com/peter-stratton/dark-factory/internal/tui"
	"github.com/spf13/cobra"
)

var implementCmd = &cobra.Command{
	Use:   "implement [issue-number...] [--issues 160,161,162]",
	Short: "Implement one or more GitHub issues",
	Long: `Fetch one or more GitHub issues by number and run the implement → review → merge
loop directly, without milestone or dependency resolution.

Issue numbers may be provided as positional arguments, via --issues, or both.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		issuesFlag, _ := cmd.Flags().GetString("issues")
		issueNums, err := collectIssueNumbers(args, issuesFlag)
		if err != nil {
			return err
		}

		configPath, _ := cmd.Flags().GetString("config")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		force, _ := cmd.Flags().GetBool("force")
		noTUI, _ := cmd.Flags().GetBool("no-tui")

		flags := parseCLIFlags(cmd)
		flags.Config = configPath

		cfg, err := config.Load(configPath, flags)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Resolve base branch: auto-generate from issue numbers when not explicitly set.
		cfg.BaseBranch = cfg.ResolveBranch("", issueNums)

		if err := config.ValidateRequiredEnv(cfg.RequiredEnv); err != nil {
			return err
		}

		if dryRun {
			for _, num := range issueNums {
				issue, err := github.FetchIssue(cfg.Repo, num)
				if err != nil {
					return fmt.Errorf("fetching issue #%d: %w", num, err)
				}
				fmt.Printf("Issue #%d: %s\n", issue.Number, issue.Title)
				fmt.Printf("Labels: %v\n", issue.Labels)
				fmt.Printf("Body:\n%s\n", issue.Body)
				fmt.Println()
			}
			return nil
		}

		// Preflight: fail fast if working tree is dirty.
		if err := checkWorkingTreeFn(); err != nil {
			return err
		}

		// Determine whether to use TUI mode: interactive terminal and --no-tui not set.
		useTUI := !noTUI && isTerminalFn(int(os.Stdout.Fd()))

		// Select the appropriate logger factory. In TUI mode the TUI owns stdout,
		// so the logger must write only to the JSON file.
		logFactory := logging.NewLogger
		if useTUI {
			logFactory = logging.NewLoggerFileOnly
		}

		// Create RunDataWriter first to get the run directory for the log file.
		var hook agent.RunDataHook
		writer, writerErr := rundata.New(cfg.Repo, "", issueNums, cfg.BaseBranch, rundata.AutoMerge{Feature: string(cfg.AutoMerge.Feature), Rollup: string(cfg.AutoMerge.Rollup)}, Version)
		var logDir string
		if writerErr != nil {
			// Fall back to a private temp directory so each run is isolated.
			tmp, tmpErr := os.MkdirTemp("", "godark-log-*")
			if tmpErr != nil {
				return fmt.Errorf("creating temp log dir: %w", tmpErr)
			}
			logDir = tmp
		} else {
			hook = writer
			logDir = writer.Dir()
		}

		logger, err := logFactory(logDir)
		if err != nil {
			return fmt.Errorf("creating logger: %w", err)
		}

		if writerErr != nil {
			logger.Warn("failed to create run data writer, run data will not be recorded", "error", writerErr)
		}

		// Pre-fetch issue titles so the TUI can display all issues upfront and
		// the dashboard can show titles before issues finish processing.
		issueTitles := make(map[string]string, len(issueNums))
		for _, num := range issueNums {
			if iss, err := github.FetchIssue(cfg.Repo, num); err == nil {
				issueTitles[strconv.Itoa(num)] = iss.Title
			}
		}
		if writer != nil {
			if err := writer.WriteIssueTitles(issueTitles); err != nil {
				logger.Warn("failed to write issue titles to run metadata", "error", err)
			}
		}

		// Initialize notifiers from config. Construction failures are logged but
		// never abort the run — notifications are best-effort.
		notifiers, notifyErr := notify.NewFromConfig(cfg.Notify)
		if notifyErr != nil {
			logger.Warn("failed to initialize notifiers, continuing without notifications", "error", notifyErr)
			notifiers = nil
		}

		authEnv, err := sandbox.CollectAuthEnv(logger, cfg.AuthPreference, cfg.RequiredEnv)
		if err != nil {
			return fmt.Errorf("collecting auth: %w", err)
		}

		prompts, err := agent.LoadPrompts(cfg)
		if err != nil {
			return fmt.Errorf("loading prompts: %w", err)
		}

		dc := sandbox.DockerConfigFromConfig(cfg.Docker, cfg.Runtime, cfg.SandboxEnv, cfg.DockerCompose)
		tag, err := sandbox.BuildImage(cmd.Context(), dc, logger)
		if err != nil {
			return fmt.Errorf("building Docker image: %w", err)
		}
		cfg.Docker.Image = tag

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		// Ensure base branch exists on the remote (no-op if empty or already exists).
		if err := orchestrator.EnsureBaseBranch(cfg.BaseBranch, logger); err != nil {
			return fmt.Errorf("ensuring base branch: %w", err)
		}

		// Acquire run lock to prevent concurrent godark executions.
		locker := lock.New(cfg.Repo, cfg.Labels().InProgress, logger)
		if err := locker.Acquire(issueNums, force); err != nil {
			return fmt.Errorf("acquiring run lock: %w", err)
		}
		defer func() {
			// Refresh the GitHub App token before releasing the lock so the
			// gh CLI call succeeds even if the original token has expired.
			if token, err := ghapp.RefreshToken(); err == nil && token != "" {
				_ = os.Setenv("GH_TOKEN", token)
			}
			if err := locker.Release(issueNums); err != nil {
				logger.Warn("failed to release run lock", "error", err)
			}
		}()

		var reporter progress.ProgressReporter
		var program *tea.Program
		var cancelRun context.CancelFunc
		if useTUI {
			ctx, cancelRun = context.WithCancel(ctx)
			model := tui.New(cfg.Repo, "", "", cfg.BaseBranch,
				string(cfg.AutoMerge.Feature), string(cfg.AutoMerge.Rollup), cancelRun)
			program = tea.NewProgram(model, tea.WithAltScreen())
			reporter = tui.NewTUIReporter(program)
		} else {
			reporter = progress.NewTextReporter(os.Stdout)
		}

		punchlistPath, _ := cmd.Flags().GetString("punchlist")

		// runLoop executes the issue processing loop and returns any error. It is
		// run directly in text mode or in a goroutine in TUI mode.
		runLoop := func() error {
			return implementIssues(ctx, issueNums, cfg, prompts, authEnv, logger, hook, writer, notifiers, reporter, punchlistPath, issueTitles)
		}

		if useTUI {
			errCh := make(chan error, 1)
			go func() {
				err := runLoop()
				errCh <- err
				program.Send(tui.RunDoneMsg{})
			}()
			_, _ = program.Run()
			return <-errCh
		}
		return runLoop()
	},
}

// implementIssues runs the implement → review → merge loop for each issue number.
// reporter receives progress events; writer persists run data (may be nil).
func implementIssues(
	ctx context.Context,
	issueNums []int,
	cfg *config.Config,
	prompts *agent.Prompts,
	authEnv map[string]string,
	logger *slog.Logger,
	hook agent.RunDataHook,
	writer *rundata.Writer,
	notifiers []notify.Notifier,
	reporter progress.ProgressReporter,
	punchlistPath string,
	issueTitles map[string]string,
) error {
	// Open stats DB early; nil on failure (errors logged, never fatal).
	statsDB := orchestrator.OpenStatsDB(logger)
	if statsDB != nil {
		defer func() {
			if err := statsDB.Close(); err != nil {
				logger.Warn("stats: failed to close database", "error", err)
			}
		}()
	}

	var implemented, readyToMerge, needsHumanReview, failed int
	var punchlistEntries []punchlist.Entry
	var implementedIssues []github.Issue

	var runTimestamp string
	if writer != nil {
		runTimestamp = filepath.Base(writer.Dir())
	}
	summaries := make([]progress.IssueSummary, len(issueNums))
	for i, num := range issueNums {
		summaries[i] = progress.IssueSummary{
			Number: num,
			Title:  issueTitles[strconv.Itoa(num)],
		}
	}
	reporter.RunStarted(cfg.Repo, "", runTimestamp, cfg.BaseBranch,
		string(cfg.AutoMerge.Feature), string(cfg.AutoMerge.Rollup), summaries)

	for i := 0; i < len(issueNums); i++ {
		issueNumber := issueNums[i]
		if ctx.Err() != nil {
			logger.Warn("context cancelled, stopping", "error", ctx.Err())
			break
		}

		issue, err := github.FetchIssue(cfg.Repo, issueNumber)
		if err != nil {
			logger.Warn("failed to fetch issue, skipping", "issue_number", issueNumber, "error", err)
			failed++
			reporter.IssueCompleted(issueNumber, "", "failed", 0, 0, err.Error(), 0.0, "")
			continue
		}

		if hasNoDarkLabel(issue.Labels) {
			logger.Info("skipping nodark issue", "issue_number", issueNumber, "title", issue.Title)
			failed++
			reporter.IssueCompleted(issueNumber, issue.Title, "failed", 0, 0,
				fmt.Sprintf("issue #%d is labeled nodark (human-only task), skipping agent processing", issueNumber), 0.0, "")
			continue
		}

		reporter.IssueStarted(issue.Number, issue.Title)
		outcome := agent.ProcessIssue(ctx, issue, cfg, prompts, authEnv, logger, hook, reporter)

		if held := handleUsageLimitHold(ctx, outcome, issue, cfg, logger, reporter, authEnv); held {
			i-- // retry this issue
			continue
		}

		di, drtm, dnhr, df := recordOutcome(ctx, issue, outcome, cfg, writer, notifiers, logger, reporter)
		implemented += di
		readyToMerge += drtm
		needsHumanReview += dnhr
		failed += df
		if outcome.Status == agent.StatusImplemented {
			implementedIssues = append(implementedIssues, issue)
		}
		if entry, ok := buildPunchlistEntry(cfg, issue, outcome, logger); ok {
			punchlistEntries = append(punchlistEntries, entry)
		}
	}

	reporter.RunFinished(implemented, readyToMerge, needsHumanReview, failed, 0)
	finalizeRunData(ctx, writer, statsDB, cfg, implemented, readyToMerge, needsHumanReview, failed, logger)
	finalizePunchlistEntries(ctx, punchlistEntries, writer, prompts, cfg, authEnv, logger, punchlistPath, reporter)

	maybeCreateRollupPR(ctx, cfg, implementedIssues, logger, reporter, writer, prompts, authEnv)

	return nil
}

// recordOutcome writes dialogue, computes cost, updates stats, fires notifications,
// and returns the counter increments for the issue outcome.
func recordOutcome(ctx context.Context, issue github.Issue, outcome agent.IssueOutcome, cfg *config.Config, writer *rundata.Writer, notifiers []notify.Notifier, logger *slog.Logger, reporter progress.ProgressReporter) (int, int, int, int) {
	writeIssueDialogue(writer, cfg.Repo, issue.Number, outcome, logger)
	var issueCost float64
	if writer != nil {
		issueCost = rundata.IssueCostUSD(writer.IssueDir(issue.Number))
	}
	di, drtm, dnhr, df := applyOutcomeStats(outcome, issue, cfg, reporter, logger, issueCost)
	logger.Info("issue outcome", "issue_number", outcome.IssueNumber, "status", outcome.Status,
		"pr_number", outcome.PRNumber, "retries", outcome.Retries, "error", outcome.Err)
	fireImplementationNotification(ctx, notifiers, cfg.Repo, issue.Number, outcome, logger)
	return di, drtm, dnhr, df
}

// handleUsageLimitHold checks if the outcome indicates a usage limit hit. If so,
// it sleeps until the reset time, refreshes auth tokens, and returns true so the
// caller can retry the issue. Returns false if no hold was needed.
func handleUsageLimitHold(ctx context.Context, outcome agent.IssueOutcome, issue github.Issue, cfg *config.Config, logger *slog.Logger, reporter progress.ProgressReporter, authEnv map[string]string) bool {
	if !outcome.UsageLimited {
		return false
	}
	resetsAt := outcome.ResetsAt
	const holdBuffer = 30 * time.Second
	const maxHold = 6 * time.Hour
	if resetsAt.IsZero() || time.Until(resetsAt) > maxHold {
		logger.Warn("usage limit reset time too far or unknown — failing issue",
			"issue_number", issue.Number, "resetsAt", resetsAt)
		reporter.IssueCompleted(issue.Number, issue.Title, "failed", 0, 0, "usage limit: reset time exceeds max hold", 0, outcome.TraceID)
		return false
	}
	sleepUntil := resetsAt.Add(holdBuffer)
	logger.Info("usage limit hit — entering hold",
		"issue_number", issue.Number, "resetsAt", resetsAt, "sleep_until", sleepUntil)
	reporter.IssueStageChanged(issue.Number, "rate-limited")
	reporter.RateLimited(resetsAt)
	timer := time.NewTimer(time.Until(sleepUntil))
	select {
	case <-ctx.Done():
		timer.Stop()
		logger.Warn("context cancelled during rate-limit hold")
		return false
	case <-timer.C:
	}
	reporter.RateLimitCleared()
	logger.Info("usage limit hold complete — resuming", "issue_number", issue.Number)
	// Refresh GitHub App token — it may have expired during the hold.
	if token, err := ghapp.RefreshToken(); err == nil && token != "" {
		_ = os.Setenv("GH_TOKEN", token)
		authEnv["GH_TOKEN"] = token
	}
	return true
}

// maybeCreateRollupPR creates a rollup PR when using a non-default base branch,
// rollup is not disabled, and at least one issue was implemented.
func maybeCreateRollupPR(ctx context.Context, cfg *config.Config, implementedIssues []github.Issue, logger *slog.Logger, reporter progress.ProgressReporter, writer *rundata.Writer, prompts *agent.Prompts, authEnv map[string]string) {
	defaultBranch := cfg.EffectiveDefaultBranch(cfg.Repo)
	if cfg.AutoMerge.Rollup == config.RollupNone ||
		cfg.BaseBranch == "" || cfg.BaseBranch == defaultBranch ||
		len(implementedIssues) == 0 {
		return
	}
	_, _, err := orchestrator.HandleRollupPR(ctx, cfg, implementedIssues, defaultBranch, logger, reporter, writer, prompts, authEnv)
	if err != nil {
		logger.Warn("rollup PR handling failed", "error", err)
		fmt.Fprintf(os.Stderr, "Rollup PR warning: %v\n", err)
	}
}

// writeIssueDialogue fetches PR comment bodies and writes dialogue to the run data writer.
func writeIssueDialogue(writer *rundata.Writer, repo string, issueNumber int, outcome agent.IssueOutcome, logger *slog.Logger) {
	if writer == nil || outcome.PRNumber <= 0 {
		return
	}
	bodies, fetchErr := fetchPRCommentBodiesFn(repo, outcome.PRNumber)
	if fetchErr != nil {
		logger.Warn("failed to fetch PR comment bodies for dialogue",
			"issue_number", issueNumber, "error", fetchErr)
		return
	}
	dialogueEntries := orchestrator.BuildDialogueEntries(bodies)
	if len(dialogueEntries) > 0 {
		if err := writer.WriteDialogue(issueNumber, dialogueEntries); err != nil {
			logger.Warn("failed to write dialogue", "issue_number", issueNumber, "error", err)
		}
	}
}

// applyOutcomeStats updates the reporter and returns counter increments
// (implemented, readyToMerge, needsHumanReview, failed).
func applyOutcomeStats(outcome agent.IssueOutcome, issue github.Issue, cfg *config.Config, reporter progress.ProgressReporter, logger *slog.Logger, costUSD float64) (int, int, int, int) {
	switch outcome.Status {
	case agent.StatusImplemented:
		reporter.IssueCompleted(issue.Number, issue.Title, "implemented", outcome.PRNumber, outcome.Retries, "", costUSD, outcome.TraceID)
		return 1, 0, 0, 0
	case agent.StatusReadyToMerge:
		reporter.IssueCompleted(issue.Number, issue.Title, "ready-to-merge", outcome.PRNumber, outcome.Retries, "", costUSD, outcome.TraceID)
		return 0, 1, 0, 0
	case agent.StatusNeedsHumanReview:
		reporter.IssueCompleted(issue.Number, issue.Title, "needs-human-review", outcome.PRNumber, 0, "", costUSD, outcome.TraceID)
		return 0, 0, 1, 0
	default:
		errMsg := ""
		if outcome.Err != nil {
			errMsg = outcome.Err.Error()
		}
		reporter.IssueCompleted(issue.Number, issue.Title, "failed", 0, 0, errMsg, costUSD, outcome.TraceID)
		return 0, 0, 0, 1
	}
}

// fireImplementationNotification sends an implementation_complete notification event.
func fireImplementationNotification(ctx context.Context, notifiers []notify.Notifier, repo string, issueNumber int, outcome agent.IssueOutcome, logger *slog.Logger) {
	notifyMsg := fmt.Sprintf("issue #%d: status=%s", issueNumber, outcome.Status)
	if outcome.PRNumber > 0 {
		notifyMsg += fmt.Sprintf(", PR #%d", outcome.PRNumber)
	}
	notify.Fire(ctx, notifiers, notify.Event{
		Type:    "implementation_complete",
		Repo:    repo,
		Message: notifyMsg,
	}, logger)
}

// buildPunchlistEntry constructs a punchlist entry for the issue/outcome pair.
// Returns false if no PR was created.
func buildPunchlistEntry(cfg *config.Config, issue github.Issue, outcome agent.IssueOutcome, logger *slog.Logger) (punchlist.Entry, bool) {
	if outcome.PRNumber <= 0 {
		return punchlist.Entry{}, false
	}
	files, err := punchlist.FetchChangedFiles(cfg.Repo, outcome.PRNumber)
	if err != nil {
		logger.Warn("failed to fetch changed files for punchlist", "pr_number", outcome.PRNumber, "error", err)
	}
	spec, err := punchlist.ReadScenarioSpec(cfg.ScenarioDir, issue.Number)
	if err != nil {
		logger.Warn("failed to read scenario spec for punchlist", "issue_number", issue.Number, "error", err)
	}
	return punchlist.Entry{
		IssueNumber:  issue.Number,
		IssueTitle:   issue.Title,
		IssueBody:    issue.Body,
		PRNumber:     outcome.PRNumber,
		Repo:         cfg.Repo,
		ScenarioSpec: spec,
		ChangedFiles: files,
	}, true
}

// finalizeRunData writes a run summary to the run data writer, then records
// the run in the stats database. statsDB and cfg may be nil; both are no-ops
// when absent.
func finalizeRunData(ctx context.Context, writer *rundata.Writer, statsDB *stats.DB, cfg *config.Config, implemented, readyToMerge, needsHumanReview, failed int, logger *slog.Logger) {
	if writer == nil {
		return
	}
	summary := rundata.RunSummary{
		Total:       implemented + readyToMerge + needsHumanReview + failed,
		Implemented: implemented,
		Failed:      failed,
	}
	if err := writer.FinalizeRun(summary); err != nil {
		logger.Warn("failed to finalize run data", "error", err)
	}
	orchestrator.WriteRunStats(ctx, statsDB, cfg, writer, summary, logger)
}

// finalizePunchlistEntries enriches, persists, and reports all accumulated punchlist entries.
func finalizePunchlistEntries(ctx context.Context, entries []punchlist.Entry, writer *rundata.Writer, prompts *agent.Prompts, cfg *config.Config, authEnv map[string]string, logger *slog.Logger, punchlistPath string, reporter progress.ProgressReporter) {
	if len(entries) == 0 {
		return
	}
	agent.EnrichPunchlistEntries(ctx, entries, prompts, cfg, authEnv, logger)
	if writer != nil {
		for _, e := range entries {
			plData := rundata.PunchlistData{
				VerificationSteps: e.ExtractVerificationSteps(),
				ScenarioCases:     e.ExtractScenarioCases(),
				AcceptanceTests:   e.AcceptanceTests,
				ChangedFiles:      e.ChangedFiles,
			}
			if err := writer.WritePunchlist(e.IssueNumber, plData); err != nil {
				logger.Warn("failed to write punchlist data", "issue_number", e.IssueNumber, "error", err)
			}
		}
	}
	text := punchlist.Generate(entries)
	reporter.PunchlistText(text)
	if punchlistPath != "" {
		if err := os.WriteFile(punchlistPath, []byte(text), 0o644); err != nil {
			logger.Warn("failed to write punchlist", "error", err)
		}
	}
}

// collectIssueNumbers merges positional args and the --issues flag value into
// a single deduplicated ordered slice. Returns an error if no issue numbers
// are provided from either source.
func collectIssueNumbers(args []string, issuesFlag string) ([]int, error) {
	seen := make(map[int]bool)
	var nums []int

	for _, a := range args {
		n, err := strconv.Atoi(strings.TrimSpace(a))
		if err != nil {
			return nil, fmt.Errorf("invalid issue number %q: %w", a, err)
		}
		if !seen[n] {
			seen[n] = true
			nums = append(nums, n)
		}
	}

	if issuesFlag != "" {
		flagNums, err := parseIssueNumbers(issuesFlag)
		if err != nil {
			return nil, fmt.Errorf("--issues: %w", err)
		}
		for _, n := range flagNums {
			if !seen[n] {
				seen[n] = true
				nums = append(nums, n)
			}
		}
	}

	if len(nums) == 0 {
		return nil, fmt.Errorf("at least one issue number is required (use positional args or --issues)")
	}
	return nums, nil
}

// parseIssueNumbers parses a comma-separated string of issue numbers into a
// slice of ints. Returns an error if any token is not a valid integer.
func parseIssueNumbers(s string) ([]int, error) {
	var nums []int
	for _, token := range strings.Split(s, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		n, err := strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("invalid issue number %q: %w", token, err)
		}
		nums = append(nums, n)
	}
	return nums, nil
}

// hasNoDarkLabel reports whether the nodark label is present in the labels slice.
func hasNoDarkLabel(labels []string) bool {
	for _, l := range labels {
		if l == label.NoDark {
			return true
		}
	}
	return false
}

// fetchPRCommentBodiesFn fetches PR comment bodies for dialogue extraction.
// Replaceable for testing.
var fetchPRCommentBodiesFn = github.FetchPRCommentBodies

// checkWorkingTreeFn checks whether the working tree has uncommitted changes.
// Replaceable for testing.
var checkWorkingTreeFn = orchestrator.CheckWorkingTree

func init() {
	f := implementCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.Int("max-retries", 3, "Maximum review/fix retry cycles")
	f.Bool("dry-run", false, "Print issue details and exit")
	f.Bool("no-judge", false, "Disable the container health judge")
	f.String("model", "", "Claude model to use: sonnet (default) or opus")
	f.Bool("no-tui", false, "Disable TUI and use plain-text output")
	f.String("auto-merge-feature", "none", "Feature branch merge strategy after approval: none (human merges), low_risk (auto-merge small/safe PRs), all (auto-merge everything)")
	f.String("auto-merge-rollup", "manual", "Rollup merge strategy after run: none (no rollup PR), manual (create PR but don't merge), auto (create and merge rollup PR)")
	f.Bool("force", false, "Clear any existing run lock before starting (override stale lock)")
	f.String("config", "godark.yaml", "Path to configuration file")
	f.String("punchlist", "", "Write manual testing punchlist to this file (always printed to stdout)")
	f.String("issues", "", "Comma-separated list of issue numbers (e.g. 160,161,162)")
	f.String("base-branch", "", "Base branch for PRs (overrides repo default branch)")
	f.String("default-branch", "", "Default branch of the repository (auto-detected if omitted)")

	rootCmd.AddCommand(implementCmd)
}
