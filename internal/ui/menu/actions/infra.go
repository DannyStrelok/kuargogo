package actions

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/deps"
	"github.com/DannyStrelok/kuargogo/internal/infra"
	"github.com/DannyStrelok/kuargogo/internal/provision"
)

// InfraInit provisions the Infrastructure Manager on the Raspberry Pi node via Ansible.
// It captures all output for display in the TUI.
func InfraInit() tea.Cmd {
	return func() tea.Msg {
		// Check dependencies
		if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v\n\nPlease install Ansible first.", err)}
		}

		// Find Infra Node
		var infraNode *config.Node
		cfg := config.GetConfig()
		for i := range cfg.Nodes {
			if cfg.Nodes[i].Role == "infra-manager" {
				infraNode = &cfg.Nodes[i]
				break
			}
		}

		if infraNode == nil {
			return ResultMsg{Output: "❌ Error: No node with role 'infra-manager' found in config.\n\nPlease configure an infrastructure manager node in kuargogo.yaml."}
		}

		// Pass Telegram config and nodes as extra-vars
		extraVars, err := ansible.PreprocessInfraVars(infraNode)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error preparing variables: %v", err)}
		}

		// Create progress channel and start async task
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			// Proactively clear existing host keys to prevent verification failures
			// during initial provisioning of the infrastructure manager.
			_ = provision.RemoveSystemHostKey(infraNode.IP)
			_ = provision.RemoveHostKey(infraNode.IP)
			if infraNode.Name != infraNode.IP {
				_ = provision.RemoveSystemHostKey(infraNode.Name)
				_ = provision.RemoveHostKey(infraNode.Name)
			}

			result, err := ansible.RunInfraInit(false, nil, infraNode.Name, extraVars, writer)

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Error: %v", err)
				return
			}

			if result != nil && !result.Success {
				ch <- fmt.Sprintf("\n❌ Infra provisioning failed (exit code: %d)", result.ExitCode)
				return
			}

			ch <- "\n✅ Infrastructure Manager provisioned successfully!"
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// InfraBotUpdate fast-tracks the update of the Telegram bot and kuargogo binary.
// It uses tags [bot, rkcli] to only run relevant tasks.
func InfraBotUpdate() tea.Cmd {
	return func() tea.Msg {
		if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v\n\nPlease install Ansible first.", err)}
		}

		// Find Infra Node
		cfg := config.GetConfig()
		infraNode := cfg.GetInfraManager()
		if infraNode == nil {
			return ResultMsg{Output: "❌ Error: No node with role 'infra-manager' found in config."}
		}

		extraVars, err := ansible.PreprocessInfraVars(infraNode)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error preparing variables: %v", err)}
		}

		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			result, err := ansible.RunInfraBotUpdate(config.IsDryRun(), extraVars, writer)

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Error: %v", err)
				return
			}

			if result != nil && !result.Success {
				ch <- fmt.Sprintf("\n❌ Bot update failed (exit code: %d)", result.ExitCode)
				return
			}

			ch <- "\n✅ Telegram Bot and KGG binary updated successfully!"
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// InfraHeartbeat performs a cluster-wide health check and returns the diagnostic report.
func InfraHeartbeat(aiEnabled bool) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		infraNode := cfg.GetInfraManager()
		if infraNode == nil {
			return ResultMsg{Output: "❌ Error: No node with role 'infra-manager' found in config."}
		}

		keyPath, err := cfg.SSH.ExpandedKeyPath()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error expanding SSH key path: %v", err)}
		}

		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			ch <- "💓 Starting Diagnostic Health Check...\n"
			mgr := infra.NewManager(infraNode.User, keyPath, cfg.SSH.Port, config.IsDryRun())
			mgr.Output = writer

			report, err := mgr.RunHealthCheck(aiEnabled)
			if err != nil {
				ch <- fmt.Sprintf("\n❌ Health check failed: %v", err)
				return
			}

			// Format the report for the TUI
			var sb strings.Builder
			sb.WriteString("\n--- Heartbeat Report ---\n")
			for _, n := range report.Nodes {
				statusIcon := "✅"
				if n.Status != "ONLINE" {
					statusIcon = "❌"
				}
				fmt.Fprintf(&sb, "%s %-15s [%s] CPU: %s (%s) RAM: %-15s Disk: %s\n", statusIcon, n.NodeName, n.Status, n.CPUUsage, n.CPUTemp, n.RAMUsage, n.DiskUsage)
				if n.Error != nil {
					fmt.Fprintf(&sb, "   ⚠️ Error: %v\n", n.Error)
				}
				for svc, st := range n.Services {
					svcIcon := "🟢"
					if st != "active" {
						svcIcon = "🔴"
					}
					fmt.Fprintf(&sb, "   %s %-10s: %s\n", svcIcon, svc, st)
				}
			}
			sb.WriteString("------------------------\n")
			if report.Summary != "" {
				sb.WriteString("\n🤖 AI Insight:\n")
				sb.WriteString(report.Summary)
				sb.WriteString("\n------------------------\n")
			}

			if len(report.RepairActions) > 0 {
				fmt.Fprintf(&sb, "\n✨ AI has suggested %d repairs.\nUse CLI 'kgg infra heartbeat --ai' to apply them interactively.\n", len(report.RepairActions))
			}

			ch <- sb.String()
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}
