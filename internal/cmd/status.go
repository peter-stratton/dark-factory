package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/phs/dark-factory/internal/dashboard"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/rundata"
	"github.com/phs/dark-factory/internal/stats"
	"github.com/spf13/cobra"
)

// githubWatchQuerier adapts the github package to the dashboard.WatchQuerier
// interface. It lives in the cmd layer, which may depend on both
// presentation (dashboard) and infrastructure (github).
type githubWatchQuerier struct{}

func (githubWatchQuerier) ListWatchedPRs(repo, lbl string) ([]dashboard.WatchedPRInfo, error) {
	prs, err := github.ListWatchedPRs(repo, lbl)
	if err != nil {
		return nil, err
	}
	result := make([]dashboard.WatchedPRInfo, len(prs))
	for i, pr := range prs {
		result[i] = dashboard.WatchedPRInfo{
			Number:      pr.Number,
			Title:       pr.Title,
			Labels:      pr.Labels,
			UpdatedAt:   pr.UpdatedAt,
			HeadRefName: pr.HeadRefName,
		}
	}
	return result, nil
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Start the dashboard web server",
	Long: `Start a local web server that serves a dashboard UI.
The homepage lists all runs from ~/.godark/runs/, most recent first.

Press Ctrl-C to stop the server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := cmd.Flags().GetInt("port")
		if err != nil {
			return fmt.Errorf("getting port flag: %w", err)
		}

		logger := slog.Default()

		reader, err := rundata.NewReader(logger)
		if err != nil {
			return fmt.Errorf("initializing run reader: %w", err)
		}

		// Open the stats database for the analysis page. Errors are non-fatal:
		// the dashboard falls back to filesystem data when statsDB is nil.
		var statsDB *stats.DB
		if home, err := os.UserHomeDir(); err == nil {
			statsPath := filepath.Join(home, ".godark", "stats.db")
			db, err := stats.Open(statsPath)
			if err != nil {
				logger.Warn("stats database unavailable, analysis reads from filesystem", "error", err)
			} else {
				statsDB = db
				defer db.Close()
			}
		} else {
			logger.Warn("could not determine home directory, stats database unavailable", "error", err)
		}

		srv, err := dashboard.New(dashboard.Config{
			Port:         port,
			Logger:       logger,
			WatchQuerier: githubWatchQuerier{},
		}, reader, statsDB)
		if err != nil {
			return fmt.Errorf("creating dashboard server: %w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		if err := srv.Serve(ctx); err != nil {
			return fmt.Errorf("dashboard server: %w", err)
		}
		return nil
	},
}

func init() {
	statusCmd.Flags().Int("port", 8374, "port to listen on")
	rootCmd.AddCommand(statusCmd)
}
