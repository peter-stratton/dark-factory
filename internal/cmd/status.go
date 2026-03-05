package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/phs/dark-factory/internal/dashboard"
	"github.com/phs/dark-factory/internal/rundata"
	"github.com/spf13/cobra"
)

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

		srv, err := dashboard.New(dashboard.Config{
			Port:   port,
			Logger: logger,
		}, reader)
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
