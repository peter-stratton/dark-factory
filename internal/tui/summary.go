package tui

import (
	"fmt"
	"strings"
)

// renderSummary returns the summary bar string for a given model.
//
// Format: [N/M workers ·] N merged · N in review · N queued · N failed · $X.XX total cost
// The worker segment is omitted when totalWorkers ≤ 1 (serial mode or not yet wired).
//
// Each count is styled according to its meaning (green/yellow/red/neutral).
// The cost uses two decimal places (e.g. $0.00, $1.23).
func renderSummary(m Model) string {
	styledSep := summaryBarStyle.Render(sep)

	var parts []string
	if m.totalWorkers > 1 {
		parts = append(parts,
			summaryNeutralStyle.Render(fmt.Sprintf("%d/%d", m.activeWorkers, m.totalWorkers))+
				summaryBarStyle.Render(" workers"))
	}
	parts = append(parts,
		summaryMergedStyle.Render(fmt.Sprintf("%d", m.merged))+
			summaryBarStyle.Render(" merged"),
		summaryInReviewStyle.Render(fmt.Sprintf("%d", m.inReview))+
			summaryBarStyle.Render(" in review"),
		summaryNeutralStyle.Render(fmt.Sprintf("%d", m.queued))+
			summaryBarStyle.Render(" queued"),
		summaryFailedStyle.Render(fmt.Sprintf("%d", m.failed))+
			summaryBarStyle.Render(" failed"),
		summaryNeutralStyle.Render(fmt.Sprintf("$%.2f", m.totalCost))+
			summaryBarStyle.Render(" total cost"),
	)

	return strings.Join(parts, styledSep)
}
