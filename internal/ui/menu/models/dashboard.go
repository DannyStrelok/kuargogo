package models

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/DannyStrelok/kuargogo/internal/ui"
	"github.com/DannyStrelok/kuargogo/internal/ui/engine"
	"github.com/DannyStrelok/kuargogo/internal/ui/menu/actions"
)

// DashboardModel renders a cluster-wide sparkline dashboard
type DashboardModel struct {
	BaseModel
	spinner spinner.Model
	nodes   []actions.DashboardNodeSnapshot
	loading bool
	width   int
	height  int
	taskCmd tea.Cmd
}

// NewDashboardModel creates a dashboard that will run the given fetch command
func NewDashboardModel(fetchCmd tea.Cmd) *DashboardModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := &DashboardModel{
		spinner: sp,
		loading: true,
		taskCmd: fetchCmd,
		width:   80,
	}
	m.SetTitle("Cluster Dashboard")
	return m
}

func (m *DashboardModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.taskCmd)
}

func (m *DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.MarkReady()
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "q":
			return m, engine.Pop()
		case "ctrl+c":
			return m, tea.Quit
		case "r":
			// Refresh
			m.loading = true
			m.nodes = nil
			return m, tea.Batch(m.spinner.Tick, m.taskCmd)
		}

	case engine.ConfigReloadedMsg:
		// Refresh
		m.loading = true
		m.nodes = nil
		return m, tea.Batch(m.spinner.Tick, m.taskCmd)

	case actions.DashboardMsg:
		m.loading = false
		m.nodes = msg.Nodes
		return m, nil
	}

	if m.loading {
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *DashboardModel) View() tea.View {
	if !m.IsReady() {
		return m.LoadingView()
	}

	if m.loading {
		m.spinner.Style = lipgloss.NewStyle().Foreground(m.theme.AccentColor())
		return tea.NewView(fmt.Sprintf("\n  %s Fetching cluster metrics...", m.spinner.View()))
	}

	if len(m.nodes) == 0 {
		return tea.NewView("No node metrics available.\n\nPress 'esc' to go back.")
	}

	// Styles from Theme
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.AccentColor())

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.SecondaryColor())

	nodeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.PrimaryColor())

	errorStyle := lipgloss.NewStyle().
		Foreground(m.theme.ErrorColor())

	dimStyle := lipgloss.NewStyle().
		Foreground(m.theme.MutedColor())

	// Bar width adapts to terminal
	barWidth := 12
	if m.width > 100 {
		barWidth = 16
	} else if m.width < 60 {
		barWidth = 8
	}

	// Build view
	var s string
	s += titleStyle.Render("📊 Cluster Dashboard") + "\n\n"

	// Header
	s += fmt.Sprintf("%-18s │ %-*s %6s │ %-*s %6s │ %-*s %6s\n",
		headerStyle.Render("Node"),
		barWidth, headerStyle.Render("CPU"),
		"",
		barWidth, headerStyle.Render("Memory"),
		"",
		barWidth, headerStyle.Render("Disk"),
		"",
	)
	s += dimStyle.Render("───────────────────┼") +
		dimStyle.Render(fmt.Sprintf("%s┼", repeatChar("─", barWidth+8))) +
		dimStyle.Render(fmt.Sprintf("%s┼", repeatChar("─", barWidth+8))) +
		dimStyle.Render(repeatChar("─", barWidth+8)) + "\n"

	for _, n := range m.nodes {
		name := nodeStyle.Render(fmt.Sprintf("%-18s", truncate(n.Name, 18)))

		if n.Error != "" {
			s += fmt.Sprintf("%s │ %s\n", name, errorStyle.Render("⚠ "+n.Error))
			continue
		}

		cpuBar := ui.RenderBar(n.CPU, barWidth)
		cpuPct := ui.FormatPercent(n.CPU)
		memBar := ui.RenderBar(n.Memory, barWidth)
		memPct := ui.FormatPercent(n.Memory)
		diskBar := ui.RenderBar(n.Disk, barWidth)
		diskPct := ui.FormatPercent(n.Disk)

		s += fmt.Sprintf("%s │ %s %s │ %s %s │ %s %s\n",
			name, cpuBar, cpuPct, memBar, memPct, diskBar, diskPct)
	}

	s += "\n" + dimStyle.Render("Press 'r' to refresh · 'esc' to go back")

	return tea.NewView(s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func repeatChar(ch string, n int) string {
	s := ""
	for i := 0; i < n; i++ {
		s += ch
	}
	return s
}
