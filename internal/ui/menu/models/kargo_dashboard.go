package models

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/DannyStrelok/kuargogo/internal/ui/engine"
	"github.com/DannyStrelok/kuargogo/internal/ui/menu/actions"
)

// KargoDashboardModel displays the live pipeline and available warehouse freight
type KargoDashboardModel struct {
	BaseModel
	pipelineName         string
	spinner              spinner.Model
	snapshot             actions.PipelineObservabilitySnapshot
	loading              bool
	err                  error
	selectedFreightIndex int
	width                int
	height               int
}

// NewKargoDashboardModel constructs a new dashboard for a Kargo pipeline
func NewKargoDashboardModel(pipelineName string) *KargoDashboardModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := &KargoDashboardModel{
		pipelineName: pipelineName,
		spinner:      sp,
		loading:      true,
	}
	m.SetTitle(fmt.Sprintf("Kargo Observability: %s", pipelineName))
	return m
}

// Init initializes the dashboard, loading Kargo observability data
func (m *KargoDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		actions.GetKargoObservability(m.pipelineName),
	)
}

// Update handles UI interactions and keyboard events
func (m *KargoDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.loading = true
			m.err = nil
			return m, tea.Batch(m.spinner.Tick, actions.GetKargoObservability(m.pipelineName))
		case "up", "k":
			if len(m.snapshot.Freights) > 0 {
				m.selectedFreightIndex--
				if m.selectedFreightIndex < 0 {
					m.selectedFreightIndex = len(m.snapshot.Freights) - 1
				}
			}
			return m, nil
		case "down", "j":
			if len(m.snapshot.Freights) > 0 {
				m.selectedFreightIndex++
				if m.selectedFreightIndex >= len(m.snapshot.Freights) {
					m.selectedFreightIndex = 0
				}
			}
			return m, nil
		case "enter":
			if len(m.snapshot.Freights) > 0 && m.selectedFreightIndex >= 0 && m.selectedFreightIndex < len(m.snapshot.Freights) {
				freight := m.snapshot.Freights[m.selectedFreightIndex]
				return m, m.promptPromotion(freight.Name)
			}
			return m, nil
		}

	case actions.PipelineObservabilityMsg:
		m.loading = false
		if msg.Error != nil {
			m.err = msg.Error
			return m, nil
		}
		m.snapshot = msg.Snapshot
		if m.selectedFreightIndex >= len(m.snapshot.Freights) {
			m.selectedFreightIndex = 0
		}
		return m, nil
	}

	if m.loading {
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// promptPromotion triggers a sub-selection using FormModel for target stage selection
func (m *KargoDashboardModel) promptPromotion(freightID string) tea.Cmd {
	if len(m.snapshot.Stages) == 0 {
		return nil
	}

	var options []huh.Option[string]
	for _, stage := range m.snapshot.Stages {
		options = append(options, huh.NewOption(strings.ToUpper(stage.Name), stage.Name))
	}

	var stageName string
	f := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select Target Stage").
				Description(fmt.Sprintf("Promote Freight %s", freightID)).
				Options(options...).
				Value(&stageName),
		),
	)

	pipelineName := m.pipelineName
	return engine.Push(NewFormModel(f, func(form *huh.Form) tea.Cmd {
		return engine.Push(NewOutputModel(actions.PromoteStage(pipelineName, stageName, freightID)))
	}))
}

// View prints the beautiful dashboard UI using lipgloss
func (m *KargoDashboardModel) View() tea.View {
	if !m.IsReady() {
		return m.LoadingView()
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.AccentColor())

	dimStyle := lipgloss.NewStyle().
		Foreground(m.theme.MutedColor())

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.SecondaryColor())

	if m.err != nil {
		var s string
		s += titleStyle.Render("🚢 Kargo Observability Dashboard") + "\n\n"
		s += lipgloss.NewStyle().Foreground(m.theme.ErrorColor()).Render(fmt.Sprintf("❌ Error querying cluster state:\n%v", m.err)) + "\n\n"
		s += dimStyle.Render("Press 'r' to retry · 'esc' to go back")
		return tea.NewView(s)
	}

	if m.loading {
		m.spinner.Style = lipgloss.NewStyle().Foreground(m.theme.AccentColor())
		return tea.NewView(fmt.Sprintf("\n  %s Querying Kargo & ArgoCD resources...", m.spinner.View()))
	}

	// 1. Render Horizontal / Vertical stages block
	var stageBlocks []string
	for i, stage := range m.snapshot.Stages {
		var sb strings.Builder

		stageTitle := lipgloss.NewStyle().
			Bold(true).
			Foreground(m.theme.PrimaryColor()).
			Render(fmt.Sprintf("Stage: %s", strings.ToUpper(stage.Name)))
		sb.WriteString(stageTitle + "\n")

		freightVal := stage.CurrentFreight
		if freightVal == "" {
			freightVal = "None"
		}

		alias := ""
		for _, f := range m.snapshot.Freights {
			if f.Name == freightVal {
				alias = f.Alias
				break
			}
		}
		if alias != "" {
			freightVal = fmt.Sprintf("%s (%s)", truncate(freightVal, 8), alias)
		} else {
			freightVal = truncate(freightVal, 16)
		}

		fmt.Fprintf(&sb, "📦 Freight: %s\n", freightVal)

		healthIcon := "⚪"
		healthColor := m.theme.MutedColor()
		if strings.EqualFold(stage.HealthStatus, "Healthy") {
			healthIcon = "🟢"
			healthColor = m.theme.SuccessColor()
		} else if strings.EqualFold(stage.HealthStatus, "Unhealthy") || strings.EqualFold(stage.HealthStatus, "Degraded") {
			healthIcon = "🔴"
			healthColor = m.theme.ErrorColor()
		} else if stage.HealthStatus != "Unknown" && stage.HealthStatus != "" {
			healthIcon = "🟡"
			healthColor = m.theme.WarningColor()
		}
		healthText := lipgloss.NewStyle().Foreground(healthColor).Render(fmt.Sprintf("%s %s", healthIcon, stage.HealthStatus))
		fmt.Fprintf(&sb, "❤️  Health:  %s\n", healthText)

		argoApp := findArgoAppForStage(m.snapshot.ArgoApps, stage.Name)
		if argoApp != nil {
			argoHealthIcon := "⚪"
			argoHealthColor := m.theme.MutedColor()
			if strings.EqualFold(argoApp.HealthStatus, "Healthy") {
				argoHealthIcon = "🟢"
				argoHealthColor = m.theme.SuccessColor()
			} else if strings.EqualFold(argoApp.HealthStatus, "Degraded") || strings.EqualFold(argoApp.HealthStatus, "Missing") {
				argoHealthIcon = "🔴"
				argoHealthColor = m.theme.ErrorColor()
			} else if argoApp.HealthStatus != "" {
				argoHealthIcon = "🟡"
				argoHealthColor = m.theme.WarningColor()
			}
			argoHealthText := lipgloss.NewStyle().Foreground(argoHealthColor).Render(fmt.Sprintf("%s %s", argoHealthIcon, argoApp.HealthStatus))

			syncIcon := "⚪"
			syncColor := m.theme.MutedColor()
			if strings.EqualFold(argoApp.SyncStatus, "Synced") {
				syncIcon = "🟢"
				syncColor = m.theme.SuccessColor()
			} else if strings.EqualFold(argoApp.SyncStatus, "OutOfSync") {
				syncIcon = "🟡"
				syncColor = m.theme.WarningColor()
			}
			syncText := lipgloss.NewStyle().Foreground(syncColor).Render(fmt.Sprintf("%s %s", syncIcon, argoApp.SyncStatus))

			fmt.Fprintf(&sb, "🚢 ArgoCD:  %s\n", argoHealthText)
			fmt.Fprintf(&sb, "   Sync:    %s", syncText)
		} else {
			sb.WriteString("🚢 ArgoCD:  ⚪ Missing")
		}

		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(healthColor).
			Padding(0, 2).
			Width(30)

		stageBlocks = append(stageBlocks, boxStyle.Render(sb.String()))

		if i < len(m.snapshot.Stages)-1 {
			arrowStyle := lipgloss.NewStyle().
				Foreground(m.theme.SecondaryColor()).
				Padding(2, 1)
			stageBlocks = append(stageBlocks, arrowStyle.Render("──>"))
		}
	}

	var pipelineView string
	if len(stageBlocks) > 0 {
		if m.width > 0 && m.width < len(m.snapshot.Stages)*36 {
			var verticalBlocks []string
			for _, block := range stageBlocks {
				if block == "──>" {
					verticalBlocks = append(verticalBlocks, lipgloss.NewStyle().Foreground(m.theme.SecondaryColor()).Padding(0, 4).Render("│\n▼"))
				} else {
					verticalBlocks = append(verticalBlocks, block)
				}
			}
			pipelineView = lipgloss.JoinVertical(lipgloss.Left, verticalBlocks...)
		} else {
			pipelineView = lipgloss.JoinHorizontal(lipgloss.Center, stageBlocks...)
		}
	}

	// 2. Render Freight list
	freightHeader := headerStyle.Render("📦 Available Freight (Artifacts in Warehouse)")
	var fl strings.Builder
	fl.WriteString(freightHeader + "\n")
	fl.WriteString(dimStyle.Render("Use Up/Down (k/j) to navigate · Enter to Promote Selected Freight") + "\n\n")

	if len(m.snapshot.Freights) == 0 {
		fl.WriteString("  No freight available in warehouse. Reconcile or wait for warehouse to sync.\n")
	} else {
		for i, f := range m.snapshot.Freights {
			selected := i == m.selectedFreightIndex

			prefix := "  "
			itemStyle := lipgloss.NewStyle().Foreground(m.theme.TextColor())
			if selected {
				prefix = lipgloss.NewStyle().Foreground(m.theme.AccentColor()).Bold(true).Render("> ")
				itemStyle = lipgloss.NewStyle().
					Foreground(m.theme.AccentColor()).
					Bold(true).
					Background(m.theme.SurfaceColor())
			}

			imageInfo := ""
			if f.ImageRepo != "" {
				imageInfo = fmt.Sprintf(" [%s:%s]", truncate(f.ImageRepo, 30), truncate(f.ImageTag, 10))
			}

			activeStages := ""
			if len(f.ActiveInStages) > 0 {
				var stages []string
				for _, stg := range f.ActiveInStages {
					stages = append(stages, strings.ToUpper(stg))
				}
				activeStages = lipgloss.NewStyle().
					Foreground(m.theme.SuccessColor()).
					Render(fmt.Sprintf(" (Active: %s)", strings.Join(stages, ", ")))
			}

			aliasStr := ""
			if f.Alias != "" {
				aliasStr = fmt.Sprintf("%-18s", fmt.Sprintf("[%s]", f.Alias))
			} else {
				aliasStr = fmt.Sprintf("%-18s", "")
			}

			timeStr := f.CreationTime.Local().Format("2006-01-02 15:04")

			line := fmt.Sprintf("%s%s  %s  %s%s",
				aliasStr,
				truncate(f.Name, 12),
				timeStr,
				imageInfo,
				activeStages,
			)

			fl.WriteString(prefix + itemStyle.Render(line) + "\n")
		}
	}

	// 3. Assemble full View
	var s strings.Builder
	s.WriteString(titleStyle.Render("🚢 KARGO PIPELINE OBSERVABILITY") + "\n")
	fmt.Fprintf(&s, "%s %s  ·  %s %s  ·  %s %s\n",
		headerStyle.Render("Pipeline:"), m.snapshot.PipelineName,
		headerStyle.Render("Project:"), m.snapshot.Project,
		headerStyle.Render("Namespace:"), m.snapshot.Namespace)
	fmt.Fprintf(&s, "%s %s\n\n", headerStyle.Render("Warehouse:"), m.snapshot.WarehouseName)

	s.WriteString(headerStyle.Render("🛣️  Stages Pipeline Flow") + "\n")
	s.WriteString(pipelineView + "\n\n")

	s.WriteString(fl.String())

	s.WriteString("\n" + dimStyle.Render("Press 'r' to refresh · 'esc' to go back · 'q' to quit"))

	return tea.NewView(s.String())
}

func findArgoAppForStage(apps []actions.ArgoAppSnapshot, stageName string) *actions.ArgoAppSnapshot {
	for _, app := range apps {
		if strings.Contains(strings.ToLower(app.Name), strings.ToLower(stageName)) {
			return &app
		}
	}
	return nil
}
