package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// autoMerge holds the feature and rollup branch names used for auto-merge.
// A nil pointer means auto-merge is not configured.
type autoMerge struct {
	feature string
	rollup  string
}

// Model is the root Bubble Tea model for the dark-factory TUI.
//
// It holds run metadata displayed in the header chrome, aggregate issue counts
// displayed in the summary bar, an issue table with per-row state, and
// terminal dimensions for layout calculations.
type Model struct {
	// Run metadata — populated via Update messages.
	repo       string
	milestone  string
	timestamp  string
	baseBranch string
	autoMerge  *autoMerge

	// Aggregate counts — updated as issues progress.
	merged    int
	inReview  int
	queued    int
	failed    int
	totalCost float64

	// Issue table state.
	issues     []issueRow
	issueIndex map[int]int // issue number → index in issues slice

	// Spinner for in-progress rows.
	spinner spinner.Model

	// Terminal dimensions.
	width  int
	height int
}

// Compile-time assertion that Model implements tea.Model.
var _ tea.Model = Model{}

// Init implements tea.Model. Returns the spinner tick command so animation
// starts immediately.
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update implements tea.Model. Handles window-size messages, spinner ticks,
// and all progress message types.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case IssueStartedMsg:
		if m.issueIndex == nil {
			m.issueIndex = make(map[int]int)
		}
		m.issueIndex[msg.Number] = len(m.issues)
		m.issues = append(m.issues, issueRow{
			number: msg.Number,
			title:  msg.Title,
		})

	case IssueStageChangedMsg:
		if idx, ok := m.issueIndex[msg.Number]; ok {
			m.issues[idx].stage = msg.Stage
		}

	case IssueCompletedMsg:
		if idx, ok := m.issueIndex[msg.Number]; ok {
			m.issues[idx].status = msg.Status
			m.issues[idx].prNumber = msg.PRNumber
			m.issues[idx].retries = msg.Retries
			m.issues[idx].errMsg = msg.ErrMsg
		}

	case WaveStartedMsg:
		// Wave metadata is informational; no per-row state changes here.

	case RunFinishedMsg:
		// Final counts could update summary bar fields if needed; currently
		// the summary bar is maintained via individual issue events.
	}

	return m, nil
}

// View implements tea.Model. Composes the header, issue table, and summary bar.
func (m Model) View() string {
	header := renderHeader(m)
	table := renderTable(m.issues, m.spinner, m.width)
	summary := renderSummary(m)

	if table == "" {
		return header + "\n\n" + summary + "\n"
	}
	return header + "\n\n" + table + "\n\n" + summary + "\n"
}

// New returns a Model pre-populated with run metadata.
//
// mergeFeature and mergeRollup may be empty strings when auto-merge is not
// configured. baseBranch may be empty when using the repository default.
func New(repo, milestone, timestamp, baseBranch, mergeFeature, mergeRollup string) Model {
	spin := spinner.New()
	spin.Spinner = spinner.Dot

	m := Model{
		repo:       repo,
		milestone:  milestone,
		timestamp:  timestamp,
		baseBranch: baseBranch,
		spinner:    spin,
	}
	if mergeFeature != "" || mergeRollup != "" {
		m.autoMerge = &autoMerge{
			feature: mergeFeature,
			rollup:  mergeRollup,
		}
	}
	return m
}
