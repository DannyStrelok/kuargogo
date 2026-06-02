package actions

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/ai"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/infra"
)

// AIStatus checks the status of running AI models
func AIStatus() tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig().AI
		client, err := ai.NewClient(cfg, config.IsDryRun())
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v", err)}
		}

		models, err := client.ListRunning()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v", err)}
		}

		if len(models) == 0 {
			return ResultMsg{Output: "No models running."}
		}

		var sb strings.Builder
		sb.WriteString("Running Models:\n")
		for _, mod := range models {
			fmt.Fprintf(&sb, "- %s (VRAM: %.0f MB)\n", mod.Name, float64(mod.SizeVRAM)/1024/1024)
		}
		return ResultMsg{Output: sb.String()}
	}
}

// AIPull downloads a model
func AIPull(modelName string) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig().AI
		client, err := ai.NewClient(cfg, config.IsDryRun())
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v", err)}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "📥 Pulling model '%s'...\n", modelName)

		err = client.Pull(modelName, func(status string, total, completed int64) {
			if completed == total && total > 0 {
				fmt.Fprintf(&sb, "✅ %s: Done\n", status)
			}
		})

		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("%s\n❌ Error: %v", sb.String(), err)}
		}
		return ResultMsg{Output: fmt.Sprintf("%s\n✅ Pull complete.", sb.String())}
	}
}

// AIChat starts an interactive chat session using the CLI subprocess
func AIChat(modelName string) tea.Cmd {
	return func() tea.Msg {
		exe, err := os.Executable()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error getting executable: %v", err)}
		}

		c := exec.Command(exe, "ai", "chat", "--model", modelName)
		return tea.ExecProcess(c, func(err error) tea.Msg {
			if err != nil {
				return ResultMsg{Output: fmt.Sprintf("❌ Chat session failed: %v", err)}
			}
			return ResultMsg{Output: "👋 Chat session ended."}
		})
	}
}

// AIHealth runs the cluster-wide diagnostic scan with AI insights
func AIHealth(aiEnabled bool) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		infraNode := cfg.GetInfraManager()
		if infraNode == nil {
			return ResultMsg{Output: "❌ Error: No node with role 'infra-manager' found."}
		}

		keyPath, err := cfg.SSH.ExpandedKeyPath()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ SSH Error: %v", err)}
		}

		mgr := infra.NewManager(infraNode.User, keyPath, cfg.SSH.Port, config.IsDryRun())
		var buf strings.Builder
		mgr.Output = &buf

		fmt.Fprintln(&buf, "💓 Starting Diagnostic Health Check...")
		report, err := mgr.RunHealthCheck(aiEnabled)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("%s\n❌ Health Check failed: %v", buf.String(), err)}
		}

		var sb strings.Builder
		sb.WriteString(buf.String())
		sb.WriteString("\n--- Diagnostic Health Report ---\n")
		for _, n := range report.Nodes {
			statusIcon := "✅"
			if n.Status != "ONLINE" {
				statusIcon = "❌"
			}
			fmt.Fprintf(&sb, "%s %-15s [%s] CPU: %s (%s) RAM: %-15s Disk: %s\n", statusIcon, n.NodeName, n.Status, n.CPUUsage, n.CPUTemp, n.RAMUsage, n.DiskUsage)
			if n.Error != nil {
				fmt.Fprintf(&sb, "   ⚠️ Error: %v\n", n.Error)
			}
			if len(n.FailingPods) > 0 {
				fmt.Fprintf(&sb, "   ⚠️ Failing Pods: %d\n", len(n.FailingPods))
			}

			for svc, st := range n.Services {
				svcIcon := "🟢"
				if st != "active" {
					svcIcon = "🔴"
				}
				fmt.Fprintf(&sb, "   %s %-10s: %s\n", svcIcon, svc, st)
			}
		}
		sb.WriteString("--------------------------------\n\n")
		if report.Summary != "" {
			sb.WriteString("🤖 AI Analysis (Post-mortem):\n")
			sb.WriteString(report.Summary)
			sb.WriteString("\n--------------------------------\n")
		}
		if len(report.RepairActions) > 0 {
			sb.WriteString("\n✨ AI has suggested repairs. Press 'R' to start the repair workflow.")
		}

		return ResultMsg{
			Output:        sb.String(),
			RepairActions: report.RepairActions,
		}
	}
}

// ExecuteRepairs applies a list of AI-suggested repairs.
// This action is safe to call from both CLI and TUI.
func ExecuteRepairs(selected []string) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		infraNode := cfg.GetInfraManager()
		if infraNode == nil {
			return ResultMsg{Output: "❌ Error: Infra Manager not found."}
		}

		keyPath, err := cfg.SSH.ExpandedKeyPath()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ SSH Error: %v", err)}
		}

		mgr := infra.NewManager(infraNode.User, keyPath, cfg.SSH.Port, config.IsDryRun())
		var buf strings.Builder
		mgr.Output = &buf

		fmt.Fprintf(&buf, "🚀 Starting %d repairs...\n", len(selected))
		for _, action := range selected {
			if err := mgr.ExecuteRepairAction(action); err != nil {
				fmt.Fprintf(&buf, "❌ Error executing '%s': %v\n", action, err)
			} else {
				fmt.Fprintf(&buf, "✅ Completed: %s\n", action)
			}
		}

		return ResultMsg{Output: buf.String()}
	}
}

// GenerateSkill creates the skill.md file for external agents
func GenerateSkill() tea.Cmd {
	return func() tea.Msg {
		path, err := ai.GenerateSkillMD()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error generating skill context: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Successfully generated skill context at:\n%s", path)}
	}
}
