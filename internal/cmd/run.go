package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/logging"
	"github.com/phs/dark-factory/internal/orchestrator"
	"github.com/phs/dark-factory/internal/progress"
	"github.com/phs/dark-factory/internal/pypi"
	"github.com/phs/dark-factory/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the development loop for a milestone or single issue",
	Long: `Fetch issues from a GitHub milestone, resolve dependencies, and process
each unblocked issue through the implement → review → merge loop.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		force, _ := cmd.Flags().GetBool("force")
		noTUI, _ := cmd.Flags().GetBool("no-tui")

		flags := parseCLIFlags(cmd)
		flags.Config = configPath

		// Parse milestone/issue locally — these are per-run params, not config.
		var issue int

		milestone, err := resolveTag(cmd)
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("issue") {
			issue, _ = cmd.Flags().GetInt("issue")
		}

		if milestone == "" && issue == 0 {
			return fmt.Errorf("milestone or issue is required (pass --milestone, --tag, or --issue)")
		}

		cfg, err := config.Load(configPath, flags)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if err := config.ValidateRequiredEnv(cfg.RequiredEnv); err != nil {
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

		pypi.WarnIfSDKOutdated(os.Stderr, logger)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		// Ensure base branch exists on the remote before starting the run (no-op if empty or dry-run).
		if !dryRun {
			if err := orchestrator.EnsureBaseBranch(cfg.BaseBranch, logger); err != nil {
				return fmt.Errorf("ensuring base branch: %w", err)
			}
		}

		punchlistPath, _ := cmd.Flags().GetString("punchlist")

		if useTUI {
			// Metadata fields (milestone, timestamp, etc.) are populated later
			// via RunStartedMsg once the orchestrator creates the run directory.
			model := tui.New(cfg.Repo, milestone, "", cfg.BaseBranch,
				string(cfg.AutoMerge.Feature), string(cfg.AutoMerge.Rollup))
			program := tea.NewProgram(model, tea.WithAltScreen())
			reporter := tui.NewTUIReporter(program)

			errCh := make(chan error, 1)
			go func() {
				err := orchestrator.Run(ctx, cfg, logger, reporter, logFactory, milestone, issue, dryRun, force, punchlistPath)
				errCh <- err
				program.Send(tea.QuitMsg{})
			}()

			_, _ = program.Run()
			return <-errCh
		}

		reporter := progress.NewTextReporter(os.Stdout)
		return orchestrator.Run(ctx, cfg, logger, reporter, logFactory, milestone, issue, dryRun, force, punchlistPath)
	},
}

// isTerminalFn reports whether the given file descriptor is connected to a
// terminal. Replaceable for testing.
var isTerminalFn = term.IsTerminal

func init() {
	f := runCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.String("milestone", "", "GitHub milestone to process (exact title)")
	f.String("tag", "", "Milestone tag (e.g., phase-3) — resolved to full milestone title")
	f.Int("issue", 0, "Single issue number to process (instead of milestone)")
	f.Int("max-retries", 3, "Maximum review/fix retry cycles per issue")
	f.Bool("dry-run", false, "Print execution plan without taking action")
	f.Bool("force", false, "Clear any existing run lock before starting (override stale lock)")
	f.Bool("no-sandbox", false, "Run agents on host instead of in Docker")
	f.Bool("no-tui", false, "Disable TUI and use plain-text output")
	f.String("auto-merge-feature", "none", "Feature branch merge strategy after approval: none (human merges), low_risk (auto-merge small/safe PRs), all (auto-merge everything)")
	f.String("auto-merge-rollup", "none", "Rollup merge strategy after run: none (no rollup PR), manual (create PR but don't merge), auto (create and merge rollup PR)")
	f.String("config", "godark.yaml", "Path to configuration file")
	f.String("punchlist", "", "Write manual testing punchlist to this file (always printed to stdout)")
	f.String("base-branch", "", "Base branch for PRs (overrides repo default branch)")
	f.String("default-branch", "", "Default branch of the repository (auto-detected if omitted)")

	rootCmd.AddCommand(runCmd)
}
