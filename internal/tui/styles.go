package tui

import "github.com/charmbracelet/lipgloss"

// Color palette following the dashboard's visual hierarchy:
//   - muted for chrome/labels
//   - green for success/merged
//   - yellow for in-review/ready-to-merge
//   - red for failures
const (
	colorMuted   = lipgloss.Color("#626262")
	colorBright  = lipgloss.Color("#FFFDF5")
	colorGreen   = lipgloss.Color("#04B575")
	colorYellow  = lipgloss.Color("#F9E04B")
	colorRed     = lipgloss.Color("#FF5F87")
	colorLogoFg  = lipgloss.Color("#FFFDF5")
	colorLogoBg  = lipgloss.Color("#5C5C5C")
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
				Foreground(lipgloss.Color("#1A1A1A")).
				Background(colorGreen).
				Bold(true).
				Padding(0, 1)

	badgeReviewStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1A1A1A")).
				Background(colorYellow).
				Bold(true).
				Padding(0, 1)

	badgeQueuedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AAAAAA")).
				Background(lipgloss.Color("#3A3A3A")).
				Padding(0, 1)

	badgeFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1A1A1A")).
				Background(colorRed).
				Bold(true).
				Padding(0, 1)

	// dividerStyle styles the horizontal rule between table and summary.
	dividerStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)
