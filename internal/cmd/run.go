package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/peter-stratton/dark-factory/internal/agent"
	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
	"github.com/peter-stratton/dark-factory/internal/label"
	"github.com/peter-stratton/dark-factory/internal/logging"
	"github.com/peter-stratton/dark-factory/internal/orchestrator"
	"github.com/peter-stratton/dark-factory/internal/progress"
	"github.com/peter-stratton/dark-factory/internal/sandbox"
	"github.com/peter-stratton/dark-factory/internal/tui"
	"github.com/peter-stratton/dark-factory/internal/watch"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the development loop for a milestone",
	Long: `Fetch issues from a GitHub milestone, resolve dependencies, and process
each unblocked issue through the implement → review → merge loop.
Use "godark implement" to process individual issues by number.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		force, _ := cmd.Flags().GetBool("force")
		noTUI, _ := cmd.Flags().GetBool("no-tui")
		watchFlag, _ := cmd.Flags().GetBool("watch")

		flags := parseCLIFlags(cmd)
		flags.Config = configPath

		milestone, err := resolveTag(cmd)
		if err != nil {
			return err
		}

		if milestone == "" {
			return fmt.Errorf("milestone is required (pass --milestone or --tag)")
		}

		cfg, err := config.Load(configPath, flags)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Resolve base branch: auto-generate from milestone when not explicitly set.
		cfg.BaseBranch = cfg.ResolveBranch(milestone, nil)

		if err := config.ValidateRequiredEnv(cfg.RequiredEnv); err != nil {
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

		// Use a private temp directory for bootstrap logging. The orchestrator will
		// create a logger in the run directory once the RunDataWriter is set up.
		// Always remove the temp dir on exit so bootstrap logs don't accumulate.
		logDir, err := os.MkdirTemp("", "godark-log-*")
		if err != nil {
			return fmt.Errorf("creating temp log dir: %w", err)
		}
		defer os.RemoveAll(logDir)
		logger, err := logFactory(logDir)
		if err != nil {
			return fmt.Errorf("creating logger: %w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		// Preflight checks before TUI starts — errors after TUI launch are
		// invisible until the user quits.
		if !dryRun {
			if err := orchestrator.CheckWorkingTree(); err != nil {
				return err
			}
			if err := orchestrator.EnsureBaseBranch(cfg.BaseBranch, logger); err != nil {
				return fmt.Errorf("ensuring base branch: %w", err)
			}
		}

		punchlistPath, _ := cmd.Flags().GetString("punchlist")

		if useTUI {
			// Wrap the signal context with a cancel we can hand to the TUI
			// so ctrl+c in the TUI cancels the orchestrator and watch loop
			// gracefully. Both orchestrator.Run and runEnterWatch run under
			// tuiCtx so the TUI ctrl+c propagates to the watch loop.
			tuiCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			// Metadata fields (milestone, timestamp, etc.) are populated later
			// via RunStartedMsg once the orchestrator creates the run directory.
			model := tui.New(cfg.Repo, milestone, "", cfg.BaseBranch,
				string(cfg.AutoMerge.Feature), string(cfg.AutoMerge.Rollup), cancel)
			program := tea.NewProgram(model, tea.WithAltScreen())
			reporter := tui.NewTUIReporter(program)

			errCh := make(chan error, 1)
			go func() {
				err := orchestrator.Run(tuiCtx, cfg, logger, reporter, logFactory, milestone, dryRun, force, punchlistPath)
				if err != nil || !watchFlag {
					errCh <- err
					program.Send(tui.RunDoneMsg{})
					return
				}
				// Signal the TUI that we are entering watch mode; the spinner keeps
				// running and the hint changes to "watching for merges".
				program.Send(tui.WatchingMsg{})
				watchErr := runEnterWatch(tuiCtx, cfg, logger, milestone, reporter)
				errCh <- watchErr
				program.Send(tui.RunDoneMsg{})
			}()

			_, _ = program.Run()
			return <-errCh
		}

		reporter := progress.NewTextReporter(os.Stdout)
		runErr := orchestrator.Run(ctx, cfg, logger, reporter, logFactory, milestone, dryRun, force, punchlistPath)
		if runErr != nil || !watchFlag {
			return runErr
		}

		// --watch: enter polling loop if any PRs are awaiting review.
		return runEnterWatch(ctx, cfg, logger, milestone, reporter)
	},
}

// isTerminalFn reports whether the given file descriptor is connected to a
// terminal. Replaceable for testing.
var isTerminalFn = term.IsTerminal

// runEnterWatch checks for pending review PRs and, if any exist, loads prompts
// and auth then enters the RunUntilDone polling loop. Called after
// orchestrator.Run when --watch is set. milestone is used for re-resolution
// runs triggered by external merges. reporter is used to surface re-resolution
// issue progress (TUI reporter in TUI mode, text reporter otherwise).
func runEnterWatch(ctx context.Context, cfg *config.Config, logger *slog.Logger, milestone string, reporter progress.ProgressReporter) error {
	// Check for awaiting PRs first to avoid loading prompts/auth unnecessarily.
	prs, err := runListPRsFn(cfg.Repo, label.AwaitingHumanReview)
	if err != nil {
		logger.Warn("checking for awaiting PRs before watch", "err", err)
	}

	if err == nil && len(prs) == 0 {
		logger.Info("no PRs awaiting review after run, skipping watch loop")
		return nil
	}

	prompts, err := agent.LoadPrompts(cfg)
	if err != nil {
		return fmt.Errorf("loading prompts: %w", err)
	}

	authEnv, err := sandbox.CollectAuthEnv(logger, cfg.AuthPreference, cfg.RequiredEnv)
	if err != nil {
		return fmt.Errorf("collecting auth: %w", err)
	}

	// Re-fetch the current open milestone issues so re-resolution has the full
	// dependency graph. Issues that were blocked in the first pass are still
	// open; their dependencies (now merged) appear in FetchClosedIssueNumbers.
	allIssues, noDarkNums, err := runFetchMilestoneIssuesFn(cfg.Repo, milestone, logger)
	if err != nil {
		logger.Warn("failed to fetch milestone issues for watch re-resolution, proceeding without daemon re-resolve", "err", err)
		allIssues = nil
	}

	// seen tracks issue numbers already processed (in the initial run or prior
	// daemon cycles) so re-resolution does not reprocess completed issues.
	// Pre-populate from PRs created by the initial run so that issues whose
	// PRs are already awaiting review or ready to merge are not reprocessed.
	seen := seedSeenFromProcessedPRs(cfg.Repo, logger)

	w := watch.New(cfg, prompts, authEnv, logger)

	if len(allIssues) > 0 {
		w.SetMergeCallback(func(mCtx context.Context, mergedNums []int) {
			logger.Info("daemon re-resolve triggered", "merged_issue_count", len(mergedNums))
			if _, reErr := orchestrator.ReResolveAndProcess(
				mCtx, allIssues, noDarkNums, seen, cfg, milestone, logger, reporter, nil,
			); reErr != nil {
				logger.Error("daemon re-resolve failed", "err", reErr)
			}
		})
	}

	return w.RunUntilDone(ctx)
}

// seedSeenFromProcessedPRs queries open PRs that carry PR lifecycle labels and
// returns a map pre-populated with the issue numbers associated with those PRs.
// This ensures that daemon re-resolution skips issues already handled by the
// initial orchestrator.Run call (e.g. PRs awaiting human review or ready to
// merge). Failed issues are intentionally excluded so the daemon can retry them.
func seedSeenFromProcessedPRs(repo string, logger *slog.Logger) map[int]bool {
	seen := make(map[int]bool)
	for _, lbl := range []string{label.AwaitingHumanReview, label.ReadyToMerge, label.FixingReviewFeedback} {
		prs, err := runListPRsFn(repo, lbl)
		if err != nil {
			logger.Warn("watch: failed to fetch PRs for seen seeding", "label", lbl, "err", err)
			continue
		}
		for _, pr := range prs {
			if n := issueNumFromBranch(pr.HeadRefName); n != 0 {
				seen[n] = true
			}
		}
	}
	return seen
}

// issueNumFromBranch parses the issue number prefix from a PR head branch name.
// Branch names follow the "<issue-number>-<slug>" convention (e.g. "42-fix-bug").
// Returns 0 if the branch name does not match the convention.
func issueNumFromBranch(name string) int {
	idx := strings.Index(name, "-")
	if idx <= 0 {
		return 0
	}
	n, err := strconv.Atoi(name[:idx])
	if err != nil {
		return 0
	}
	return n
}

// runFetchMilestoneIssuesFn fetches open milestone issues and splits out
// nodark-labeled ones. Replaceable for testing.
var runFetchMilestoneIssuesFn = func(repo, milestone string, logger *slog.Logger) ([]github.Issue, []int, error) {
	issues, err := github.FetchMilestoneIssues(repo, milestone)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching milestone issues: %w", err)
	}
	var noDarkNums []int
	var filtered []github.Issue
	for _, iss := range issues {
		hasNoDark := false
		for _, l := range iss.Labels {
			if l == label.NoDark {
				hasNoDark = true
				break
			}
		}
		if hasNoDark {
			logger.Info("watch: skipping nodark issue", "issue_number", iss.Number)
			noDarkNums = append(noDarkNums, iss.Number)
		} else {
			filtered = append(filtered, iss)
		}
	}
	return filtered, noDarkNums, nil
}

// runListPRsFn is a testability seam for listing PRs by label. Replaced in tests.
var runListPRsFn = func(repo, lbl string) ([]github.PRInfo, error) {
	return github.ListPRsWithLabel(repo, lbl)
}

func init() {
	f := runCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.String("milestone", "", "GitHub milestone to process (exact title)")
	f.String("tag", "", "Milestone tag (e.g., phase-3) — resolved to full milestone title")
	f.Int("max-retries", 3, "Maximum review/fix retry cycles per issue")
	f.Bool("dry-run", false, "Print execution plan without taking action")
	f.Bool("force", false, "Clear any existing run lock before starting (override stale lock)")
	f.Bool("no-judge", false, "Disable the container health judge")
	f.Bool("no-tui", false, "Disable TUI and use plain-text output")
	f.Bool("watch", false, "After first pass, keep running and poll for PR reviews until no more await review")
	f.String("auto-merge-feature", "none", "Feature branch merge strategy after approval: none (human merges), low_risk (auto-merge small/safe PRs), all (auto-merge everything)")
	f.String("auto-merge-rollup", "manual", "Rollup merge strategy after run: none (no rollup PR), manual (create PR but don't merge), auto (create and merge rollup PR)")
	f.String("config", "godark.yaml", "Path to configuration file")
	f.String("punchlist", "", "Write manual testing punchlist to this file (always printed to stdout)")
	f.String("base-branch", "", "Base branch for PRs (overrides repo default branch)")
	f.String("default-branch", "", "Default branch of the repository (auto-detected if omitted)")

	rootCmd.AddCommand(runCmd)
}
