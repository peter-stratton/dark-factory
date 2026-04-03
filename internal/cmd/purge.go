package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peter-stratton/dark-factory/internal/rundata"
	"github.com/peter-stratton/dark-factory/internal/stats"
	"github.com/spf13/cobra"
)

// newPurgeDB is a testability seam: replaced in tests to inject a custom DB.
var newPurgeDB = func() (*stats.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}
	return stats.Open(filepath.Join(home, ".godark", "stats.db"))
}

// newPurgeReader is a testability seam: replaced in tests to inject a custom reader.
var newPurgeReader = func(logger *slog.Logger) (*rundata.Reader, error) {
	return rundata.NewReader(logger)
}

var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Remove test run data from stats DB and filesystem",
	Long: `Delete runs matching --repo or --milestone from both the stats database
(~/.godark/stats.db) and the filesystem run history (~/.godark/runs/).

By default, removes runs where repo = 'owner/repo' or milestone = 'test-milestone'.
Use --dry-run to preview what would be deleted without making changes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		milestone, _ := cmd.Flags().GetString("milestone")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		if repo == "" && milestone == "" {
			return fmt.Errorf("at least one of --repo or --milestone is required")
		}

		logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

		db, err := newPurgeDB()
		if err != nil {
			return fmt.Errorf("opening stats DB: %w", err)
		}
		defer db.Close()

		reader, err := newPurgeReader(logger)
		if err != nil {
			return fmt.Errorf("creating reader: %w", err)
		}

		ctx := context.Background()

		// Query matching runs using OR semantics: repo matches OR milestone matches.
		tx, err := db.BeginTx(ctx)
		if err != nil {
			return fmt.Errorf("beginning transaction: %w", err)
		}
		defer func() {
			// best-effort rollback; no-op if already committed
			_ = tx.Rollback()
		}()

		// Build WHERE clause for matching runs.
		var conds []string
		var queryArgs []any
		if repo != "" {
			conds = append(conds, "repo = ?")
			queryArgs = append(queryArgs, repo)
		}
		if milestone != "" {
			conds = append(conds, "milestone = ?")
			queryArgs = append(queryArgs, milestone)
		}
		where := " WHERE " + strings.Join(conds, " OR ")

		// Collect matching run IDs and filesystem info.
		rows, err := tx.QueryContext(ctx,
			"SELECT id, repo, started_at FROM runs"+where, queryArgs...) // #nosec G202 -- hardcoded clauses with ? placeholders
		if err != nil {
			return fmt.Errorf("querying runs: %w", err)
		}
		defer rows.Close()

		type runInfo struct {
			id        string
			repo      string
			startedAt string
		}
		var runs []runInfo
		for rows.Next() {
			var ri runInfo
			if scanErr := rows.Scan(&ri.id, &ri.repo, &ri.startedAt); scanErr != nil {
				return fmt.Errorf("scanning run: %w", scanErr)
			}
			runs = append(runs, ri)
		}
		if err = rows.Err(); err != nil {
			return fmt.Errorf("iterating runs: %w", err)
		}

		if len(runs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No matching runs found — nothing to purge.")
			return nil
		}

		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would purge %d run(s).\n", len(runs))
			return nil
		}

		// Delete in batches of 500 to stay within SQLite variable limits.
		const batchSize = 500
		var stepsDeleted, outcomesDeleted, runsDeleted int64

		for i := 0; i < len(runs); i += batchSize {
			end := i + batchSize
			if end > len(runs) {
				end = len(runs)
			}
			batch := runs[i:end]

			placeholders := make([]string, len(batch))
			batchArgs := make([]any, len(batch))
			for j, ri := range batch {
				placeholders[j] = "?"
				batchArgs[j] = ri.id
			}
			inClause := strings.Join(placeholders, ",")

			res, err := tx.ExecContext(ctx,
				"DELETE FROM step_results WHERE run_id IN ("+inClause+")", batchArgs...) // #nosec G202
			if err != nil {
				return fmt.Errorf("deleting step results: %w", err)
			}
			n, _ := res.RowsAffected()
			stepsDeleted += n

			res, err = tx.ExecContext(ctx,
				"DELETE FROM issue_outcomes WHERE run_id IN ("+inClause+")", batchArgs...) // #nosec G202
			if err != nil {
				return fmt.Errorf("deleting issue outcomes: %w", err)
			}
			n, _ = res.RowsAffected()
			outcomesDeleted += n

			res, err = tx.ExecContext(ctx,
				"DELETE FROM runs WHERE id IN ("+inClause+")", batchArgs...) // #nosec G202
			if err != nil {
				return fmt.Errorf("deleting runs: %w", err)
			}
			n, _ = res.RowsAffected()
			runsDeleted += n
		}

		if err = tx.Commit(); err != nil {
			return fmt.Errorf("committing transaction: %w", err)
		}

		// Filesystem cleanup.
		var dirsRemoved int
		for _, ri := range runs {
			parts := strings.SplitN(ri.repo, "/", 2)
			if len(parts) != 2 {
				continue
			}
			// Validate path components to prevent directory traversal.
			if parts[0] == "" || parts[1] == "" || parts[0] == ".." || parts[1] == ".." ||
				strings.ContainsAny(parts[0], "/\\") || strings.ContainsAny(parts[1], "/\\") {
				logger.Warn("skipping filesystem cleanup for run with invalid repo", "id", ri.id, "repo", ri.repo)
				continue
			}
			// Parse the DB timestamp to the filesystem format.
			ts, parseErr := parseTimestampForDir(ri.startedAt)
			if parseErr != nil {
				logger.Warn("skipping filesystem cleanup for run", "id", ri.id, "error", parseErr)
				continue
			}
			runDir := reader.RunDir(parts[0], parts[1], ts)
			if err := os.RemoveAll(runDir); err != nil {
				logger.Warn("removing run directory", "path", runDir, "error", err)
				continue
			}
			dirsRemoved++

			// Clean up empty parent directories (repo dir, then owner dir).
			repoDir := reader.RunDir(parts[0], parts[1], "")
			if entries, err := os.ReadDir(repoDir); err == nil && len(entries) == 0 {
				if err := os.Remove(repoDir); err == nil {
					ownerDir := reader.RunDir(parts[0], "", "")
					if entries, err := os.ReadDir(ownerDir); err == nil && len(entries) == 0 {
						_ = os.Remove(ownerDir)
					}
				}
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(),
			"Purged %d run(s), %d issue outcome(s), %d step result(s) from stats DB. Removed %d run directory(ies).\n",
			runsDeleted, outcomesDeleted, stepsDeleted, dirsRemoved)
		return nil
	},
}

// parseTimestampForDir converts a DB timestamp (2006-01-02T15:04:05Z) to the
// filesystem directory format (20060102-150405).
func parseTimestampForDir(dbTimestamp string) (string, error) {
	const dbLayout = "2006-01-02T15:04:05Z"
	const dirLayout = "20060102-150405"
	t, err := time.Parse(dbLayout, dbTimestamp)
	if err != nil {
		return "", fmt.Errorf("parsing timestamp %q: %w", dbTimestamp, err)
	}
	return t.UTC().Format(dirLayout), nil
}

func init() {
	purgeCmd.Flags().String("repo", "", "Filter by repo (owner/repo)")
	purgeCmd.Flags().String("milestone", "", "Filter by milestone")
	purgeCmd.Flags().Bool("dry-run", false, "Show what would be deleted without deleting")
	rootCmd.AddCommand(purgeCmd)
}
