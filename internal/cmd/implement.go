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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/detect"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/lock"
	"github.com/phs/dark-factory/internal/logging"
	"github.com/phs/dark-factory/internal/notify"
	"github.com/phs/dark-factory/internal/orchestrator"
	"github.com/phs/dark-factory/internal/progress"
	"github.com/phs/dark-factory/internal/punchlist"
	"github.com/phs/dark-factory/internal/pypi"
	"github.com/phs/dark-factory/internal/rundata"
	"github.com/phs/dark-factory/internal/sandbox"
	"github.com/phs/dark-factory/internal/tui"
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

		if cfg.NoSandbox {
			fmt.Fprintln(os.Stderr, "WARNING: running without sandbox — agent execution is not containerized")
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
		writer, writerErr := rundata.New(cfg.Repo, "", issueNums, cfg.BaseBranch, rundata.AutoMerge{Feature: string(cfg.AutoMerge.Feature), Rollup: string(cfg.AutoMerge.Rollup)})
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

		// Pre-fetch issue titles and write them to run.json so the dashboard can
		// display titles for all issues before they finish processing.
		if writer != nil {
			issueTitles := make(map[string]string, len(issueNums))
			for _, num := range issueNums {
				if iss, err := github.FetchIssue(cfg.Repo, num); err == nil {
					issueTitles[strconv.Itoa(num)] = iss.Title
				}
			}
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

		pypi.WarnIfSDKOutdated(os.Stderr, logger)

		// Auto-detect project type when no runtime/commands are explicitly configured.
		detect.ApplyToConfig(cfg, ".", logger)

		authEnv, err := sandbox.CollectAuthEnv(logger, cfg.AuthPreference, cfg.RequiredEnv)
		if err != nil {
			return fmt.Errorf("collecting auth: %w", err)
		}

		prompts, err := agent.LoadPrompts(cfg)
		if err != nil {
			return fmt.Errorf("loading prompts: %w", err)
		}

		if !cfg.NoSandbox {
			dc := sandbox.DockerConfigFromConfig(cfg.Docker, cfg.Runtime, cfg.SandboxEnv)
			tag, err := sandbox.BuildImage(cmd.Context(), dc, logger)
			if err != nil {
				return fmt.Errorf("building Docker image: %w", err)
			}
			cfg.Docker.Image = tag
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		// Ensure base branch exists on the remote (no-op if empty or already exists).
		if err := orchestrator.EnsureBaseBranch(cfg.BaseBranch, logger); err != nil {
			return fmt.Errorf("ensuring base branch: %w", err)
		}

		// Acquire run lock to prevent concurrent godark executions.
		locker := lock.New(cfg.Repo, logger)
		if err := locker.Acquire(issueNums, force); err != nil {
			return fmt.Errorf("acquiring run lock: %w", err)
		}
		defer func() {
			if err := locker.Release(issueNums); err != nil {
				logger.Warn("failed to release run lock", "error", err)
			}
		}()

		var reporter progress.ProgressReporter
		var program *tea.Program
		if useTUI {
			model := tui.New(cfg.Repo, "", "", cfg.BaseBranch,
				string(cfg.AutoMerge.Feature), string(cfg.AutoMerge.Rollup))
			program = tea.NewProgram(model, tea.WithAltScreen())
			reporter = tui.NewTUIReporter(program)
		} else {
			reporter = progress.NewTextReporter(os.Stdout)
		}

		punchlistPath, _ := cmd.Flags().GetString("punchlist")

		// runLoop executes the issue processing loop and returns any error. It is
		// run directly in text mode or in a goroutine in TUI mode.
		runLoop := func() error {
			return implementIssues(ctx, issueNums, cfg, prompts, authEnv, logger, hook, writer, notifiers, reporter, punchlistPath)
		}

		if useTUI {
			errCh := make(chan error, 1)
			go func() {
				err := runLoop()
				errCh <- err
				program.Send(tea.QuitMsg{})
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
) error {
	var implemented, readyToMerge, needsHumanReview, failed int
	var punchlistEntries []punchlist.Entry

	var runTimestamp string
	if writer != nil {
		runTimestamp = filepath.Base(writer.Dir())
	}
	reporter.RunStarted(cfg.Repo, "", runTimestamp, cfg.BaseBranch,
		string(cfg.AutoMerge.Feature), string(cfg.AutoMerge.Rollup), len(issueNums))

	for _, issueNumber := range issueNums {
		if ctx.Err() != nil {
			logger.Warn("context cancelled, stopping", "error", ctx.Err())
			break
		}

		issue, err := github.FetchIssue(cfg.Repo, issueNumber)
		if err != nil {
			logger.Warn("failed to fetch issue, skipping", "issue_number", issueNumber, "error", err)
			failed++
			reporter.IssueCompleted(issueNumber, "", "failed", 0, 0, err.Error())
			continue
		}

		reporter.IssueStarted(issue.Number, issue.Title)
		outcome := agent.ProcessIssue(ctx, issue, cfg, prompts, authEnv, logger, hook)
		writeIssueDialogue(writer, cfg.Repo, issueNumber, outcome, logger)

		di, drtm, dnhr, df := applyOutcomeStats(outcome, issue, cfg, reporter, logger)
		implemented += di
		readyToMerge += drtm
		needsHumanReview += dnhr
		failed += df

		logger.Info("issue outcome",
			"issue_number", outcome.IssueNumber,
			"status", outcome.Status,
			"pr_number", outcome.PRNumber,
			"retries", outcome.Retries,
			"error", outcome.Err,
		)

		fireImplementationNotification(ctx, notifiers, cfg.Repo, issueNumber, outcome, logger)

		if entry, ok := buildPunchlistEntry(cfg, issue, outcome, logger); ok {
			punchlistEntries = append(punchlistEntries, entry)
		}
	}

	reporter.RunFinished(implemented, readyToMerge, needsHumanReview, failed, 0)
	finalizeRunData(writer, implemented, readyToMerge, needsHumanReview, failed, logger)
	finalizePunchlistEntries(ctx, punchlistEntries, writer, prompts, cfg, authEnv, logger, punchlistPath, reporter)

	return nil
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
func applyOutcomeStats(outcome agent.IssueOutcome, issue github.Issue, cfg *config.Config, reporter progress.ProgressReporter, logger *slog.Logger) (int, int, int, int) {
	switch outcome.Status {
	case agent.StatusImplemented:
		reporter.IssueCompleted(issue.Number, issue.Title, "implemented", outcome.PRNumber, outcome.Retries, "")
		if err := orchestrator.PullAfterMerge(cfg.EffectiveBaseBranch(), logger); err != nil {
			logger.Warn("could not sync local repo after merge", "error", err)
		}
		return 1, 0, 0, 0
	case agent.StatusReadyToMerge:
		reporter.IssueCompleted(issue.Number, issue.Title, "ready-to-merge", outcome.PRNumber, outcome.Retries, "")
		return 0, 1, 0, 0
	case agent.StatusNeedsHumanReview:
		reporter.IssueCompleted(issue.Number, issue.Title, "needs-human-review", outcome.PRNumber, 0, "")
		return 0, 0, 1, 0
	default:
		errMsg := ""
		if outcome.Err != nil {
			errMsg = outcome.Err.Error()
		}
		reporter.IssueCompleted(issue.Number, issue.Title, "failed", 0, 0, errMsg)
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

// finalizeRunData writes a run summary to the run data writer.
func finalizeRunData(writer *rundata.Writer, implemented, readyToMerge, needsHumanReview, failed int, logger *slog.Logger) {
	if writer == nil {
		return
	}
	if err := writer.FinalizeRun(rundata.RunSummary{
		Total:       implemented + readyToMerge + needsHumanReview + failed,
		Implemented: implemented,
		Failed:      failed,
	}); err != nil {
		logger.Warn("failed to finalize run data", "error", err)
	}
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
	f.Bool("no-sandbox", false, "Run agents on host instead of in Docker")
	f.Bool("no-tui", false, "Disable TUI and use plain-text output")
	f.String("auto-merge-feature", "none", "Feature branch merge strategy after approval: none (human merges), low_risk (auto-merge small/safe PRs), all (auto-merge everything)")
	f.String("auto-merge-rollup", "none", "Rollup merge strategy after run: none (no rollup PR), manual (create PR but don't merge), auto (create and merge rollup PR)")
	f.Bool("force", false, "Clear any existing run lock before starting (override stale lock)")
	f.String("config", "godark.yaml", "Path to configuration file")
	f.String("punchlist", "", "Write manual testing punchlist to this file (always printed to stdout)")
	f.String("issues", "", "Comma-separated list of issue numbers (e.g. 160,161,162)")
	f.String("base-branch", "", "Base branch for PRs (overrides repo default branch)")
	f.String("default-branch", "", "Default branch of the repository (auto-detected if omitted)")

	rootCmd.AddCommand(implementCmd)
}
