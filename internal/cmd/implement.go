package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/phs/dark-factory/internal/agent"
	"github.com/phs/dark-factory/internal/config"
	"github.com/phs/dark-factory/internal/detect"
	"github.com/phs/dark-factory/internal/github"
	"github.com/phs/dark-factory/internal/lock"
	"github.com/phs/dark-factory/internal/logging"
	"github.com/phs/dark-factory/internal/notify"
	"github.com/phs/dark-factory/internal/orchestrator"
	"github.com/phs/dark-factory/internal/punchlist"
	"github.com/phs/dark-factory/internal/pypi"
	"github.com/phs/dark-factory/internal/rundata"
	"github.com/phs/dark-factory/internal/sandbox"
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

		flags := config.CLIFlags{Config: configPath}

		if cmd.Flags().Changed("repo") {
			v, _ := cmd.Flags().GetString("repo")
			flags.Repo = &v
		}
		if cmd.Flags().Changed("max-retries") {
			v, _ := cmd.Flags().GetInt("max-retries")
			flags.MaxRetries = &v
		}
		if cmd.Flags().Changed("no-sandbox") {
			v, _ := cmd.Flags().GetBool("no-sandbox")
			flags.NoSandbox = &v
		}
		if cmd.Flags().Changed("auto-merge-feature") {
			v, _ := cmd.Flags().GetString("auto-merge-feature")
			flags.AutoMergeFeature = &v
		}
		if cmd.Flags().Changed("auto-merge-rollup") {
			v, _ := cmd.Flags().GetString("auto-merge-rollup")
			flags.AutoMergeRollup = &v
		}
		if cmd.Flags().Changed("base-branch") {
			v, _ := cmd.Flags().GetString("base-branch")
			flags.BaseBranch = &v
		}
		if cmd.Flags().Changed("default-branch") {
			v, _ := cmd.Flags().GetString("default-branch")
			flags.DefaultBranch = &v
		}

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

		// Create RunDataWriter first to get the run directory for the log file.
		var hook agent.RunDataHook
		writer, writerErr := rundata.New(cfg.Repo, "", issueNums, cfg.BaseBranch, rundata.AutoMerge{Feature: cfg.AutoMerge.Feature, Rollup: cfg.AutoMerge.Rollup})
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

		logger, err := logging.NewLogger(logDir)
		if err != nil {
			return fmt.Errorf("creating logger: %w", err)
		}

		if writerErr != nil {
			logger.Warn("failed to create run data writer, run data will not be recorded", "error", writerErr)
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

		// Stats across all issues.
		var implemented, readyToMerge, needsHumanReview, failed int

		// Punchlist entries accumulated across all issues.
		var punchlistEntries []punchlist.Entry

		punchlistPath, _ := cmd.Flags().GetString("punchlist")

		for _, issueNumber := range issueNums {
			if ctx.Err() != nil {
				logger.Warn("context cancelled, stopping", "error", ctx.Err())
				break
			}

			issue, err := github.FetchIssue(cfg.Repo, issueNumber)
			if err != nil {
				logger.Warn("failed to fetch issue, skipping", "issue_number", issueNumber, "error", err)
				failed++
				fmt.Printf("  #%d — failed to fetch: %s\n", issueNumber, err)
				continue
			}

			outcome := agent.ProcessIssue(ctx, issue, cfg, prompts, authEnv, logger, hook)

			// Write dialogue if we have a PR and a writer.
			if writer != nil && outcome.PRNumber > 0 {
				bodies, fetchErr := fetchPRCommentBodiesFn(cfg.Repo, outcome.PRNumber)
				if fetchErr != nil {
					logger.Warn("failed to fetch PR comment bodies for dialogue",
						"issue_number", issueNumber, "error", fetchErr)
				} else {
					dialogueEntries := orchestrator.BuildDialogueEntries(bodies)
					if len(dialogueEntries) > 0 {
						if err := writer.WriteDialogue(issueNumber, dialogueEntries); err != nil {
							logger.Warn("failed to write dialogue",
								"issue_number", issueNumber, "error", err)
						}
					}
				}
			}

			switch outcome.Status {
			case "implemented":
				implemented++
				fmt.Printf("  #%d %s — implemented (PR #%d, %d retries)\n", issue.Number, issue.Title, outcome.PRNumber, outcome.Retries)
				if err := orchestrator.PullAfterMerge(cfg.EffectiveBaseBranch(), logger); err != nil {
					logger.Warn("could not sync local repo after merge", "error", err)
				}
			case "ready-to-merge":
				readyToMerge++
				fmt.Printf("  #%d %s — ready-to-merge (PR #%d, %d retries)\n", issue.Number, issue.Title, outcome.PRNumber, outcome.Retries)
			case "needs-human-review":
				needsHumanReview++
				fmt.Printf("  #%d %s — needs human review (PR #%d)\n", issue.Number, issue.Title, outcome.PRNumber)
			default:
				failed++
				errMsg := ""
				if outcome.Err != nil {
					errMsg = outcome.Err.Error()
				}
				fmt.Printf("  #%d %s — failed: %s\n", issue.Number, issue.Title, errMsg)
			}

			logger.Info("issue outcome",
				"issue_number", outcome.IssueNumber,
				"status", outcome.Status,
				"pr_number", outcome.PRNumber,
				"retries", outcome.Retries,
				"error", outcome.Err,
			)

			// Fire implementation_complete notification for each processed issue.
			notifyMsg := fmt.Sprintf("issue #%d: status=%s", issueNumber, outcome.Status)
			if outcome.PRNumber > 0 {
				notifyMsg += fmt.Sprintf(", PR #%d", outcome.PRNumber)
			}
			notify.Fire(ctx, notifiers, notify.Event{
				Type:    "implementation_complete",
				Repo:    cfg.Repo,
				Message: notifyMsg,
			}, logger)

			if outcome.PRNumber > 0 {
				files, err := punchlist.FetchChangedFiles(cfg.Repo, outcome.PRNumber)
				if err != nil {
					logger.Warn("failed to fetch changed files for punchlist",
						"pr_number", outcome.PRNumber, "error", err)
				}
				spec, err := punchlist.ReadScenarioSpec(cfg.ScenarioDir, issueNumber)
				if err != nil {
					logger.Warn("failed to read scenario spec for punchlist",
						"issue_number", issueNumber, "error", err)
				}
				punchlistEntries = append(punchlistEntries, punchlist.Entry{
					IssueNumber:  issue.Number,
					IssueTitle:   issue.Title,
					IssueBody:    issue.Body,
					PRNumber:     outcome.PRNumber,
					Repo:         cfg.Repo,
					ScenarioSpec: spec,
					ChangedFiles: files,
				})
			}
		}

		// Print totals.
		fmt.Println()
		fmt.Printf("Results: %d implemented, %d ready-to-merge, %d needs-human-review, %d failed\n",
			implemented, readyToMerge, needsHumanReview, failed)

		// Finalize run data.
		if writer != nil {
			if err := writer.FinalizeRun(rundata.RunSummary{
				Total:       implemented + readyToMerge + needsHumanReview + failed,
				Implemented: implemented,
				Failed:      failed,
			}); err != nil {
				logger.Warn("failed to finalize run data", "error", err)
			}
		}

		// Generate punchlist from all accumulated entries.
		if len(punchlistEntries) > 0 {
			agent.EnrichPunchlistEntries(ctx, punchlistEntries, prompts, cfg, authEnv, logger)

			if writer != nil {
				for _, e := range punchlistEntries {
					plData := rundata.PunchlistData{
						VerificationSteps: e.ExtractVerificationSteps(),
						ScenarioCases:     e.ExtractScenarioCases(),
						AcceptanceTests:   e.AcceptanceTests,
						ChangedFiles:      e.ChangedFiles,
					}
					if err := writer.WritePunchlist(e.IssueNumber, plData); err != nil {
						logger.Warn("failed to write punchlist data",
							"issue_number", e.IssueNumber, "error", err)
					}
				}
			}

			text := punchlist.Generate(punchlistEntries)
			fmt.Println()
			if err := punchlist.Write(text, punchlistPath); err != nil {
				logger.Warn("failed to write punchlist", "error", err)
			}
		}

		return nil
	},
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
