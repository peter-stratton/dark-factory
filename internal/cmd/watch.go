package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/peter-stratton/dark-factory/internal/agent"
	"github.com/peter-stratton/dark-factory/internal/config"
	"github.com/peter-stratton/dark-factory/internal/github"
	"github.com/peter-stratton/dark-factory/internal/label"
	"github.com/peter-stratton/dark-factory/internal/logging"
	"github.com/peter-stratton/dark-factory/internal/sandbox"
	"github.com/peter-stratton/dark-factory/internal/tui"
	"github.com/peter-stratton/dark-factory/internal/watch"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Poll for PRs awaiting human review and handle CHANGES_REQUESTED or APPROVED reviews",
	Long: `Polls GitHub for pull requests labeled godark:awaiting-human-review and
detects when a human submits a review. When CHANGES_REQUESTED is detected, the
implementer agent is invoked to address the feedback using session resumption.
After the fix is pushed, the PR is re-labeled godark:awaiting-human-review.
When APPROVED is detected, the PR is merged (squash + delete branch) and the
linked issue is closed.

Runs as a long-lived foreground process. Press Ctrl+C to stop.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		noTUI, _ := cmd.Flags().GetBool("no-tui")
		flags := config.CLIFlags{Config: configPath}
		if cmd.Flags().Changed("repo") {
			v, _ := cmd.Flags().GetString("repo")
			flags.Repo = &v
		}

		cfg, err := config.Load(configPath, flags)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Determine whether to use TUI mode: interactive terminal and --no-tui not set.
		useTUI := !noTUI && isTerminalFn(int(os.Stdout.Fd()))

		// Select the appropriate logger factory. In TUI mode the TUI owns stdout,
		// so the logger must write only to the JSON file.
		logFactory := logging.NewLogger
		if useTUI {
			logFactory = logging.NewLoggerFileOnly
		}

		logDir, err := os.MkdirTemp("", "godark-watch-*")
		if err != nil {
			return fmt.Errorf("creating temp log dir: %w", err)
		}

		logger, err := logFactory(logDir)
		if err != nil {
			return fmt.Errorf("creating logger: %w", err)
		}
		logger.Info("logging to", "dir", logDir)

		prompts, err := agent.LoadPrompts(cfg)
		if err != nil {
			return fmt.Errorf("loading prompts: %w", err)
		}

		authEnv, err := sandbox.CollectAuthEnv(logger, cfg.AuthPreference, cfg.OAuthTokenEnv, cfg.RequiredEnv)
		if err != nil {
			return fmt.Errorf("collecting auth: %w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		watcher := watch.New(cfg, prompts, authEnv, logger, Version)

		if !useTUI {
			return watcher.Run(ctx)
		}

		// TUI mode: run the watch loop in a goroutine and feed events into the
		// Bubble Tea program.
		interval := watchPollInterval(cfg)
		ctx, cancelWatch := context.WithCancel(ctx)
		model := tui.NewWatchModel(cfg.Repo, interval, cancelWatch)
		program := tea.NewProgram(model, tea.WithAltScreen())

		errCh := make(chan error, 1)

		// Goroutine 1: run the watch action loop (handles review events).
		go func() {
			errCh <- watcher.Run(ctx)
			program.Send(tui.WatchDoneMsg{})
		}()

		// Goroutine 2: poll for PR display data and feed PRUpdateMsg / ActivityMsg.
		go watchTUIPoller(ctx, program, cfg.Repo, interval, logger)

		_, tuiErr := program.Run()
		watchErr := <-errCh
		// Join the two errors so that a TUI initialisation failure (e.g. terminal
		// not writable) is not silently discarded behind the watcher result.
		return errors.Join(tuiErr, watchErr)
	},
}

const watchDefaultPollInterval = 60 * time.Second

// watchPollInterval returns the poll interval from config, falling back to
// the default when cfg.Watch is nil or PollInterval is empty.
func watchPollInterval(cfg *config.Config) time.Duration {
	if cfg.Watch == nil || cfg.Watch.PollInterval == "" {
		return watchDefaultPollInterval
	}
	d, err := time.ParseDuration(cfg.Watch.PollInterval)
	if err != nil || d <= 0 {
		return watchDefaultPollInterval
	}
	return d
}

// msgSender is the minimal interface for sending messages to a Bubble Tea
// program. *tea.Program satisfies this interface.
type msgSender interface {
	Send(tea.Msg)
}

// watchTUIPoller runs a separate tick loop that fetches the current watched PR
// list and sends PRUpdateMsg to sender on each tick. It also sends an initial
// update immediately so the table is populated before the first tick fires.
func watchTUIPoller(ctx context.Context, sender msgSender, repo string, interval time.Duration, logger interface{ Info(string, ...any) }) {
	sendPRUpdate := func() {
		prs, err := listWatchedPRsFn(repo, label.AwaitingHumanReview)
		if err != nil {
			logger.Info("tui poller: failed to list watched PRs", "err", err)
			return
		}
		rows := make([]tui.WatchPRRow, len(prs))
		for i, pr := range prs {
			rows[i] = tui.WatchPRRow{
				Number: pr.Number,
				Title:  pr.Title,
				Labels: pr.Labels,
			}
		}
		sender.Send(tui.PRUpdateMsg{PRs: rows})
	}

	// Poll immediately before the first tick.
	sendPRUpdate()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sendPRUpdate()
		}
	}
}

// listWatchedPRsFn is a testability seam for github.ListWatchedPRs.
var listWatchedPRsFn = func(repo, lbl string) ([]github.WatchedPR, error) {
	return github.ListWatchedPRs(repo, lbl)
}

func init() {
	f := watchCmd.Flags()
	f.String("repo", "", "GitHub repository (owner/repo)")
	f.String("config", "godark.yaml", "Path to configuration file")
	f.Bool("no-tui", false, "Disable TUI and use plain-text output")

	rootCmd.AddCommand(watchCmd)
}
