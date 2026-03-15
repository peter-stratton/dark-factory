package tui

import (
	"strings"

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

	// Run lifecycle.
	done       bool          // true after the orchestrator finishes
	cancelling bool          // true after user requests cancellation
	cancelFn   func()        // cancels the orchestrator context

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
	case tea.KeyMsg:
		return m.handleKey(msg)

	case RunDoneMsg:
		m.done = true
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case IssueStartedMsg:
		m.handleIssueStarted(msg)

	case IssueStageChangedMsg:
		if idx, ok := m.issueIndex[msg.Number]; ok {
			m.issues[idx].stage = msg.Stage
		}

	case IssueCompletedMsg:
		m.handleIssueCompleted(msg)

	case RunStartedMsg:
		m.handleRunStarted(msg)

	case WaveStartedMsg:
		// Wave metadata is informational; no per-row state changes here.

	case RunFinishedMsg:
		m.merged = msg.Implemented
		m.inReview = msg.ReadyToMerge + msg.NeedsHumanReview
		m.failed = msg.Failed
		m.queued = msg.Blocked
	}

	return m, nil
}

// View implements tea.Model. Composes the header, issue table, and summary bar.
func (m Model) View() string {
	header := renderHeader(m)
	table := renderTable(m.issues, m.spinner, m.width)
	summary := renderSummary(m)

	// Horizontal divider between content and summary bar.
	divWidth := m.width
	if divWidth <= 0 {
		divWidth = 40
	}
	divider := dividerStyle.Render(strings.Repeat("─", divWidth/2))

	var hint string
	if m.done {
		hint = "\n\n" + headerLabelStyle.Render("press q to exit")
	} else if m.cancelling {
		hint = "\n\n" + summaryFailedStyle.Render("cancelling... waiting for current issue to finish")
	} else {
		hint = "\n\n" + headerLabelStyle.Render("press ctrl+c to cancel")
	}

	if table == "" {
		return header + "\n\n" + divider + "\n\n" + summary + hint + "\n"
	}
	return header + "\n\n" + table + "\n\n" + divider + "\n\n" + summary + hint + "\n"
}

// New returns a Model pre-populated with run metadata.
//
// cancelFn is called when the user presses ctrl+c during an active run to
// cancel the orchestrator's context. It may be nil (cancellation disabled).
// mergeFeature and mergeRollup may be empty strings when auto-merge is not
// configured. baseBranch may be empty when using the repository default.
func New(repo, milestone, timestamp, baseBranch, mergeFeature, mergeRollup string, cancelFn func()) Model {
	spin := spinner.New()
	spin.Spinner = spinner.Dot

	m := Model{
		repo:       repo,
		milestone:  milestone,
		timestamp:  timestamp,
		baseBranch: baseBranch,
		cancelFn:   cancelFn,
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

// handleKey processes key presses. During an active run, ctrl+c cancels the
// orchestrator. Once done, q/esc/ctrl+c exit the TUI.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.done {
			return m, tea.Quit
		}
		if !m.cancelling && m.cancelFn != nil {
			m.cancelling = true
			m.cancelFn()
		}
	case "q", "esc":
		if m.done {
			return m, tea.Quit
		}
	}
	return m, nil
}

// handleIssueStarted adds a new issue row or skips if already pre-populated.
// Sets the stage to "starting" so the row shows a spinner immediately,
// keeping the visual state in sync with the queued count.
func (m *Model) handleIssueStarted(msg IssueStartedMsg) {
	if m.issueIndex == nil {
		m.issueIndex = make(map[int]int)
	}
	if _, exists := m.issueIndex[msg.Number]; !exists {
		m.issueIndex[msg.Number] = len(m.issues)
		m.issues = append(m.issues, issueRow{
			number: msg.Number,
			title:  msg.Title,
		})
	}
	if idx, ok := m.issueIndex[msg.Number]; ok {
		m.issues[idx].stage = "starting"
	}
	if m.queued > 0 {
		m.queued--
	}
}

// handleIssueCompleted updates the issue row and increments summary counts.
func (m *Model) handleIssueCompleted(msg IssueCompletedMsg) {
	idx, ok := m.issueIndex[msg.Number]
	if !ok {
		return
	}
	m.issues[idx].status = msg.Status
	m.issues[idx].prNumber = msg.PRNumber
	m.issues[idx].retries = msg.Retries
	m.issues[idx].errMsg = msg.ErrMsg
	switch msg.Status {
	case "implemented":
		m.merged++
	case "ready-to-merge", "needs-human-review":
		m.inReview++
	case "failed":
		m.failed++
	}
}

// handleRunStarted populates header metadata and pre-fills the issue table.
func (m *Model) handleRunStarted(msg RunStartedMsg) {
	m.repo = msg.Repo
	m.milestone = msg.Milestone
	m.timestamp = msg.Timestamp
	m.baseBranch = msg.BaseBranch
	if msg.MergeFeature != "" || msg.MergeRollup != "" {
		m.autoMerge = &autoMerge{
			feature: msg.MergeFeature,
			rollup:  msg.MergeRollup,
		}
	}
	if m.issueIndex == nil {
		m.issueIndex = make(map[int]int)
	}
	for _, iss := range msg.Issues {
		if _, exists := m.issueIndex[iss.Number]; !exists {
			m.issueIndex[iss.Number] = len(m.issues)
			m.issues = append(m.issues, issueRow{
				number: iss.Number,
				title:  iss.Title,
			})
		}
	}
	m.queued = len(m.issues)
}
