package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// renderWatchView composes the full watch TUI screen.
func renderWatchView(m WatchModel) string {
	header := renderWatchHeader(m)
	prTable := renderWatchPRTable(m)
	activityLog := renderWatchActivityLog(m)
	footer := renderWatchFooter(m)

	divWidth := m.width
	if divWidth <= 0 {
		divWidth = 40
	}
	divider := dividerStyle.Render(strings.Repeat("─", divWidth/2))

	var sections []string
	sections = append(sections, header)
	sections = append(sections, divider)
	sections = append(sections, prTable)
	if activityLog != "" {
		sections = append(sections, divider)
		sections = append(sections, activityLog)
	}
	sections = append(sections, divider)
	sections = append(sections, footer)

	return strings.Join(sections, "\n\n") + "\n"
}

// renderWatchHeader renders the two-line header: logo + repo name on line 1,
// "watch mode" label on line 2.
func renderWatchHeader(m WatchModel) string {
	logo := headerLogoStyle.Render("godark watch")
	repo := headerRepoStyle.Render(m.repo)

	var line1 string
	if m.width <= 0 {
		line1 = logo + "  " + repo
	} else {
		logoWidth := lipgloss.Width(logo)
		repoWidth := lipgloss.Width(repo)
		gap := m.width - logoWidth - repoWidth
		if gap < 1 {
			gap = 1
		}
		line1 = logo + strings.Repeat(" ", gap) + repo
	}

	line2 := headerLabelStyle.Render("watching for PRs awaiting human review")
	return line1 + "\n" + line2
}

// renderWatchPRTable renders the PR table section. Shows "no PRs awaiting
// review" when the list is empty. When the terminal height is known, at most
// half the available lines are used for PR rows so the activity log and footer
// always remain visible.
func renderWatchPRTable(m WatchModel) string {
	if len(m.prs) == 0 {
		return headerLabelStyle.Render("no PRs awaiting review")
	}

	prs := m.prs
	if m.height > 0 {
		maxRows := m.height / 2
		if maxRows < 1 {
			maxRows = 1
		}
		if len(prs) > maxRows {
			prs = prs[:maxRows]
		}
	}

	var rows []string
	for _, pr := range prs {
		rows = append(rows, renderWatchPRRow(pr, m.width))
	}
	return strings.Join(rows, "\n")
}

// renderWatchPRRow renders a single PR row: number, title, and label badges.
func renderWatchPRRow(pr WatchPRRow, width int) string {
	num := rowNumberStyle.Render(fmt.Sprintf("#%d", pr.Number))

	labelBadges := renderLabelBadges(pr.Labels)
	badgesWidth := lipgloss.Width(labelBadges)

	// Reserve space: "#NNN " (6) + " " gap + badges.
	// When width is unknown (0), skip truncation so the full title is shown.
	var title string
	if width <= 0 {
		title = rowTitleStyle.Render(pr.Title)
	} else {
		titleWidth := width - 8 - badgesWidth - 2
		if titleWidth < 10 {
			titleWidth = 10
		}
		title = rowTitleStyle.Render(truncate(pr.Title, titleWidth))
	}

	left := num + " " + title
	leftWidth := lipgloss.Width(left)

	if labelBadges == "" {
		return left
	}

	// Right-align badges.
	if width <= 0 {
		return left + "  " + labelBadges
	}
	rightCol := width - badgesWidth
	if rightCol < leftWidth+2 {
		rightCol = leftWidth + 2
	}
	gap := rightCol - leftWidth
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + labelBadges
}

// renderLabelBadges renders all labels as styled inline badges.
func renderLabelBadges(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	var parts []string
	for _, l := range labels {
		parts = append(parts, renderLabelBadge(l))
	}
	return strings.Join(parts, " ")
}

// renderLabelBadge renders a single label badge with appropriate color.
func renderLabelBadge(label string) string {
	switch {
	case strings.Contains(label, "approved") || strings.Contains(label, "merged"):
		return badgeMergedStyle.Render(label)
	case strings.Contains(label, "awaiting"):
		return badgeReviewStyle.Render(label)
	case strings.Contains(label, "failed") || strings.Contains(label, "error"):
		return badgeFailedStyle.Render(label)
	case strings.Contains(label, "fixing"):
		return badgeInProgressStyle.Render(label)
	default:
		return badgeQueuedStyle.Render(label)
	}
}

// renderWatchActivityLog renders the activity log section.
func renderWatchActivityLog(m WatchModel) string {
	if len(m.activity) == 0 {
		return ""
	}

	header := headerLabelStyle.Render("recent activity")
	var lines []string
	lines = append(lines, header)
	for _, entry := range m.activity {
		lines = append(lines, "  "+headerValueStyle.Render(entry))
	}
	return strings.Join(lines, "\n")
}

// renderWatchFooter renders the status footer line.
func renderWatchFooter(m WatchModel) string {
	styledSep := headerSepStyle.Render(sep)

	var parts []string

	if m.done {
		parts = append(parts, summaryMergedStyle.Render("stopped"))
		hint := headerLabelStyle.Render("press q to exit")
		return strings.Join(parts, styledSep) + styledSep + hint
	}

	if m.cancelling {
		parts = append(parts, summaryFailedStyle.Render("cancelling..."))
		return strings.Join(parts, styledSep)
	}

	parts = append(parts, headerValueStyle.Render("watching"))

	if !m.lastPoll.IsZero() {
		ago := formatAgo(time.Since(m.lastPoll))
		parts = append(parts, headerLabelStyle.Render("last poll: ")+headerValueStyle.Render(ago))
	}

	hint := headerLabelStyle.Render("press ctrl+c to stop")
	return strings.Join(parts, styledSep) + styledSep + hint
}

// formatAgo returns a human-readable "N ago" string for a duration.
func formatAgo(d time.Duration) string {
	switch {
	case d < time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}
