package actions

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/cluster"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/provision"
)

// ClusterInit initializes K3s on the first master node via Ansible
func ClusterInit(masterNode config.Node, isHA bool, tags []string) tea.Cmd {
	return func() tea.Msg {
		// Create progress channel and start async task
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			_, _ = fmt.Fprintf(writer, "🚀 Initializing K3s on %s (%s)\n", masterNode.Name, masterNode.IP)
			_, _ = fmt.Fprintf(writer, "HA Mode: %v\n\n", isHA)

			cfg := config.GetConfig()
			vip := cfg.K3s.VIP
			result, err := ansible.RunK3sInit(masterNode.Name, isHA, vip, config.IsDryRun(), tags, writer)

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Error: %v", err)
				return
			}

			if !result.Success {
				ch <- fmt.Sprintf("\n❌ Init failed (exit code %d)", result.ExitCode)
				return
			}

			ch <- "\n✅ Master initialized successfully!\n"

			// Extract and persist token
			token := ansible.ExtractClusterToken(result.Stdout)
			if token != "" {
				ch <- fmt.Sprintf("🔑 Cluster Token: %s\n", token)
				_ = config.ModifyConfig(func(c *config.ClusterConfig) {
					c.K3s.Token = config.Secret(token)
				})
				if err := config.SaveConfig(); err != nil {
					ch <- fmt.Sprintf("⚠️  Failed to save token to config: %v\n", err)
				} else {
					ch <- "💾 Token persisted to config.\n"
				}
			}

			ch <- "\n📝 Next: Use 'Cluster Operations → Join Node' to add workers"
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// ClusterJoin joins a node to the cluster via Ansible
func ClusterJoin(node config.Node, masterNode config.Node, tags []string) tea.Cmd {
	return func() tea.Msg {
		// Create progress channel and start async task
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			// Get token from master (SSH)
			cfg := config.GetConfig()
			kp, err := cfg.SSH.ExpandedKeyPath()
			if err != nil {
				ch <- fmt.Sprintf("❌ Error expanding key path: %v", err)
				return
			}

			port := cfg.SSH.Port
			if port == 0 {
				port = 22
			}

			var sb strings.Builder
			masterMgr := cluster.NewManager(masterNode.User, kp, port, config.IsDryRun())
			masterMgr.Output = &sb
			token, err := masterMgr.GetMasterToken(masterNode.IP)
			if err != nil {
				ch <- fmt.Sprintf("❌ Failed to get token from master: %v", err)
				return
			}
			_, _ = writer.Write([]byte(sb.String())) // Forward any output from token retrieval

			// Determine role
			role := "agent"
			if node.Role == "master" || node.Role == "server" {
				role = "server"
			}
			isGPU := node.Labels["gpu"] == "nvidia"

			_, _ = fmt.Fprintf(writer, "🔗 Joining %s (%s) as %s...\n", node.Name, node.IP, role)
			if isGPU {
				_, _ = fmt.Fprintf(writer, "🎮 GPU node detected\n")
			}
			_, _ = fmt.Fprintf(writer, "\n")

			vip := cfg.K3s.VIP
			result, err := ansible.RunK3sJoin(node.Name, masterNode.IP, token, role, vip, config.IsDryRun(), tags, writer)

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Error: %v", err)
				return
			}

			if !result.Success {
				ch <- fmt.Sprintf("\n❌ Join failed (exit code %d)", result.ExitCode)
				return
			}

			ch <- "\n✅ Node joined successfully!"
			if isGPU {
				ch <- "\n🎮 Run 'kgg setup-gpu' on this node if not done yet."
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// ClusterDrain drains a node via Ansible (runs on master)
func ClusterDrain(masterNode config.Node, targetNodeName string, tags []string) tea.Cmd {
	return func() tea.Msg {
		// Create progress channel and start async task
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			_, _ = fmt.Fprintf(writer, "🔄 Draining node %s...\n\n", targetNodeName)

			result, err := ansible.RunK3sDrain(masterNode.Name, targetNodeName, config.IsDryRun(), tags, writer)

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Error: %v", err)
				return
			}

			if !result.Success {
				ch <- fmt.Sprintf("\n❌ Drain failed (exit code %d)", result.ExitCode)
				return
			}

			ch <- "\n✅ Node drained successfully!\n"
			ch <- "📝 Pods have been evicted. Node is now unschedulable."
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// ClusterReset uninstalls K3s from a node via Ansible
func ClusterReset(node config.Node, tags []string) tea.Cmd {
	return func() tea.Msg {
		// Create progress channel and start async task
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			_, _ = fmt.Fprintf(writer, "🗑️  Resetting K3s on %s (%s)...\n\n", node.Name, node.IP)

			result, err := ansible.RunK3sReset(node.Name, config.IsDryRun(), tags, writer)

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Error: %v", err)
				return
			}

			if !result.Success {
				ch <- fmt.Sprintf("\n❌ Reset failed (exit code %d)", result.ExitCode)
				return
			}

			ch <- "\n✅ K3s uninstalled successfully!"
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// SiteDeploy runs the site.yml master playbook for full cluster deployment
func SiteDeploy(tags []string) tea.Cmd {
	return func() tea.Msg {
		// Create progress channel and start async task
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			msg := "🚀 Starting full cluster deployment...\n"
			if config.IsDryRun() {
				msg = "🧪 [DRY RUN] Starting full cluster deployment simulation...\n"
			}
			_, _ = writer.Write([]byte(msg))

			cfg := config.GetConfig()
			if cfg.K3s.VIP != "" {
				_, _ = fmt.Fprintf(writer, "🌐 High Availability enabled (VIP: %s)\n", cfg.K3s.VIP)
			} else {
				_, _ = writer.Write([]byte("ℹ️  Standalone (non-HA) mode detected.\n"))
			}

			_, _ = writer.Write([]byte("🔑 Note: K3s token will be retrieved dynamically from the master node.\n"))
			_, _ = writer.Write([]byte("🧹 Proactively cleaning SSH host keys for cluster nodes...\n\n"))

			// Clean VIP
			if cfg.K3s.VIP != "" {
				_ = provision.RemoveSystemHostKey(cfg.K3s.VIP)
				_ = provision.RemoveHostKey(cfg.K3s.VIP)
			}

			// Clean all nodes
			for _, n := range cfg.Nodes {
				_ = provision.RemoveSystemHostKey(n.IP)
				_ = provision.RemoveHostKey(n.IP)
				if n.Name != n.IP {
					_ = provision.RemoveSystemHostKey(n.Name)
					_ = provision.RemoveHostKey(n.Name)
				}
			}

			result, err := ansible.RunSite(config.IsDryRun(), tags, writer)

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Error: %v", err)
				return
			}

			if !result.Success {
				ch <- fmt.Sprintf("\n❌ Site deploy failed (exit code %d)", result.ExitCode)
				return
			}

			ch <- fmt.Sprintf("\n✅ Full cluster deployment completed in %s", result.Duration.Round(1e9))

			// Extract and persist token
			token := ansible.ExtractClusterToken(result.Stdout)
			if token != "" {
				ch <- fmt.Sprintf("🔑 Cluster Token: %s\n", token)
				_ = config.ModifyConfig(func(c *config.ClusterConfig) {
					c.K3s.Token = config.Secret(token)
				})
				if err := config.SaveConfig(); err != nil {
					ch <- fmt.Sprintf("⚠️  Failed to save token to config: %v\n", err)
				} else {
					ch <- "💾 Token persisted to config.\n"
				}
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// ClusterRemediate runs kgg cluster remediate for a target node via manager.go
func ClusterRemediate(masterNode config.Node, targetNodeName string, tags []string) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			cfg := config.GetConfig()
			keyPath, err := cfg.SSH.ExpandedKeyPath()
			if err != nil {
				ch <- fmt.Sprintf("❌ Error expanding key path: %v", err)
				return
			}

			port := cfg.SSH.Port
			if port == 0 {
				port = 22
			}

			mgr := cluster.NewManager(masterNode.User, keyPath, port, config.IsDryRun())
			mgr.Output = writer

			_, _ = fmt.Fprintf(writer, "🛠️  Starting manual K3s Node Remediation for %s...\n\n", targetNodeName)
			err = mgr.RemediateNode(&masterNode, targetNodeName, tags)
			if err != nil {
				ch <- fmt.Sprintf("\n❌ Remediation failed: %v", err)
				return
			}
			ch <- fmt.Sprintf("\n✅ Remediation completed successfully for %s!\n", targetNodeName)
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

