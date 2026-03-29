package tui

import "github.com/charmbracelet/lipgloss"

// Color palette following the dashboard's visual hierarchy:
//   - muted for chrome/labels
//   - green for success/merged
//   - yellow for in-review/ready-to-merge
//   - red for failures
//
// AdaptiveColor picks the right value depending on the terminal's background
// color (light vs dark). The Light value is used on light backgrounds, Dark
// on dark backgrounds.
var (
	colorMuted  = lipgloss.AdaptiveColor{Light: "#626262", Dark: "#626262"}
	colorBright = lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#FFFDF5"}
	colorGreen  = lipgloss.AdaptiveColor{Light: "#067D52", Dark: "#04B575"}
	colorYellow = lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#F9E04B"}
	colorRed    = lipgloss.AdaptiveColor{Light: "#CC3355", Dark: "#FF5F87"}
	colorLogoFg = lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#FFFDF5"}
	colorLogoBg = lipgloss.AdaptiveColor{Light: "#3A3A3A", Dark: "#5C5C5C"}
)

var (
	// headerLogoStyle styles the "godark" logo text on the left of the header.
	headerLogoStyle = lipgloss.NewStyle().
			Foreground(colorLogoFg).
			Background(colorLogoBg).
			Bold(true).
			Padding(0, 1)

	// headerRepoStyle styles the repo name on the right of the header.
	headerRepoStyle = lipgloss.NewStyle().
			Foreground(colorBright).
			Bold(true)

	// headerLabelStyle styles muted label text (e.g., "base:", "auto-merge:").
	headerLabelStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	// headerValueStyle styles highlighted value text (milestone, timestamp, etc.).
	headerValueStyle = lipgloss.NewStyle().
				Foreground(colorBright)

	// headerSepStyle styles the middle-dot separator character.
	headerSepStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// summaryBarStyle styles the summary bar container.
	summaryBarStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// summaryMergedStyle styles the merged count.
	summaryMergedStyle = lipgloss.NewStyle().
				Foreground(colorGreen)

	// summaryInReviewStyle styles the in-review count.
	summaryInReviewStyle = lipgloss.NewStyle().
				Foreground(colorYellow)

	// summaryFailedStyle styles the failed count.
	summaryFailedStyle = lipgloss.NewStyle().
				Foreground(colorRed)

	// summaryNeutralStyle styles counts with no special meaning (queued, cost).
	summaryNeutralStyle = lipgloss.NewStyle().
				Foreground(colorBright)

	// Badge styles — background-colored tags shown after issue titles.
	badgeMergedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#1A1A1A"}).
				Background(colorGreen).
				Bold(true).
				Padding(0, 1)

	badgeReviewStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#1A1A1A"}).
				Background(colorYellow).
				Bold(true).
				Padding(0, 1)

	badgeInProgressStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#1A5276", Dark: "#85C1E9"}).
				Background(lipgloss.AdaptiveColor{Light: "#D4E6F1", Dark: "#1A3650"}).
				Padding(0, 1)

	badgeQueuedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#626262", Dark: "#AAAAAA"}).
				Background(lipgloss.AdaptiveColor{Light: "#D0D0D0", Dark: "#3A3A3A"}).
				Padding(0, 1)

	badgeFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#1A1A1A"}).
				Background(colorRed).
				Bold(true).
				Padding(0, 1)

	badgeJudgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#1A1A1A"}).
			Background(lipgloss.AdaptiveColor{Light: "#D4760A", Dark: "#FF8C00"}).
			Bold(true).
			Padding(0, 1)

	badgeRateLimitedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#1A1A1A"}).
			Background(lipgloss.AdaptiveColor{Light: "#8B4513", Dark: "#CD853F"}).
			Bold(true).
			Padding(0, 1)

	// dividerStyle styles the horizontal rule between table and summary.
	dividerStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Detail panel styles — for error and judge messages shown below the table.
	detailErrStyle = lipgloss.NewStyle().
			Foreground(colorRed)

	detailJudgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#D4760A", Dark: "#FF8C00"})

	detailLabelStyle = lipgloss.NewStyle().
				Foreground(colorMuted)
)
