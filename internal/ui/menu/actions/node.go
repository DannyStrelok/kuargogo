package actions

import (
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/observability"
	"github.com/DannyStrelok/kuargogo/internal/provision"
)

// HealthCheck runs diagnostics on a node
func HealthCheck(n config.Node) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		kp, err := cfg.SSH.ExpandedKeyPath()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Error expanding key path: %v", err)}
		}

		executor, err := provision.NewExecutor(n.User, kp, false)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Error connecting: %v", err)}
		}
		// PREVENT TUI CRASH: Do not write to stdout/stderr while Bubble Tea is running
		executor.Stdout = io.Discard
		executor.Stderr = io.Discard

		// 1. Fetch metrics from Prometheus API
		var promResults []provision.HealthCheckResult
		var masterNode *config.Node
		for _, node := range cfg.Nodes {
			if node.Role == "master" || node.Role == "control-plane" {
				masterNode = &node
				break
			}
		}

		if masterNode != nil {
			var promExecutor *provision.Executor
			if masterNode.IP == n.IP {
				promExecutor = executor
			} else {
				promExecutor, _ = provision.NewExecutor(masterNode.User, kp, false)
			}

			if promExecutor != nil {
				promExecutor.Stdout = io.Discard
				promExecutor.Stderr = io.Discard
				promClient := observability.NewClient(promExecutor, masterNode.IP, cfg.SSH.Port)
				promResults, _ = promClient.GetNodeMetrics(n.Name, n.IP)
			}
		}

		// 2. Fetch legacy checks
		legacyResults, err := executor.RunHealthCheck(n.IP, cfg.SSH.Port)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Error running checks: %v", err)}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Health Report for %s (%s)\n\n", n.Name, n.IP)

		allResults := append(promResults, legacyResults...)
		for _, r := range allResults {
			status := r.Result
			if r.Error != nil {
				status = fmt.Sprintf("Error: %v", r.Error)
			}
			fmt.Fprintf(&sb, "%s %s: %s\n", r.Icon, r.Name, status)
		}
		return ResultMsg{Output: sb.String()}
	}
}

// Provision bootstraps a new node with configurable user creation.
// Runs a pre-flight SSH check before launching Ansible.
func Provision(n config.Node, createUser bool, tags []string) tea.Cmd {
	return func() tea.Msg {
		// Create progress channel and start async task
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			_, _ = fmt.Fprintf(writer, "Provisioning %s (%s) via Ansible...\n\n", n.Name, n.IP)

			// Pre-flight: verify SSH key access works
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

			if err := provision.VerifySSHAccess(n.IP, port, n.User, kp, config.IsDryRun()); err != nil {
				ch <- fmt.Sprintf("❌ %v", err)
				return
			}
			_, _ = writer.Write([]byte("✅ SSH pre-flight check passed\n\n"))

			result, err := ansible.RunProvision(n.Name, createUser, "", config.IsDryRun(), tags, writer)

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Error: %v\n", err)
				return
			}
			if !result.Success {
				ch <- fmt.Sprintf("\n❌ Provisioning failed (exit code: %d)\n", result.ExitCode)
				return
			}

			ch <- "\n✅ Provisioning complete!\n"
			if createUser {
				ch <- "👤 User 'kgg-admin' created.\n"
			}
			ch <- "📝 Note: A reboot may be required."
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// GPUSetup installs NVIDIA drivers on a node via Ansible
func GPUSetup(n config.Node, tags []string) tea.Cmd {
	return func() tea.Msg {
		// Create progress channel and start async task
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			_, _ = fmt.Fprintf(writer, "Setting up GPU on %s (%s) via Ansible...\n\n", n.Name, n.IP)

			result, err := ansible.RunGPUSetup(n.Name, config.IsDryRun(), tags, writer)

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Error: %v\n", err)
				return
			}
			if !result.Success {
				ch <- fmt.Sprintf("\n❌ GPU setup failed (exit code: %d)\n", result.ExitCode)
				return
			}
			ch <- "\n✅ GPU setup complete! A reboot is recommended.\n"
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// NodeAdd adds a new node to the configuration
func NodeAdd(node config.Node) tea.Cmd {
	return func() tea.Msg {
		// Validation
		if node.Name == "" || node.IP == "" || node.Role == "" {
			return ResultMsg{Output: "❌ Error: Name, IP, and Role are required."}
		}

		// Check for duplicates
		cfg := config.GetConfig()
		for _, n := range cfg.Nodes {
			if n.Name == node.Name || n.IP == node.IP {
				return ResultMsg{Output: fmt.Sprintf("❌ Error: Node with name '%s' or IP '%s' already exists.", node.Name, node.IP)}
			}
		}

		// Use default user if not provided
		if node.User == "" {
			node.User = "root"
		}

		newNodes := append(cfg.Nodes, node)
		config.UpdateNodes(newNodes)

		// Save
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error saving config: %v", err)}
		}

		return ResultMsg{Output: fmt.Sprintf("✅ Node added: %s (%s) [%s]", node.Name, node.IP, node.Role)}
	}
}

// NodeEdit updates an existing node's configuration
func NodeEdit(originalName string, node config.Node) tea.Cmd {
	return func() tea.Msg {
		if err := config.UpdateNode(originalName, node); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating node: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error saving config: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Node '%s' updated.", node.Name)}
	}
}

// NodeRemove deletes a node from the configuration
func NodeRemove(identifier string) tea.Cmd {
	return func() tea.Msg {
		if err := config.RemoveNode(identifier); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error removing node: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error saving config: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Node '%s' removed.", identifier)}
	}
}

// BootstrapNode runs the full node preparation flow:
// Network Bootstrap (optional) → SSH Identity Setup → Ansible Provisioning.
func BootstrapNode(nodeName, dhcpIP, staticIP, user, password, keyPath string, sshPort int, createUser, skipProvision bool) tea.Cmd {
	return func() tea.Msg {
		// Create progress channel and start async task
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			err := provision.FullBootstrap(provision.FullBootstrapOptions{
				NodeName:      nodeName,
				DHCP_IP:       dhcpIP,
				StaticIP:      staticIP,
				User:          user,
				Password:      password,
				KeyPath:       keyPath,
				SSHPort:       sshPort,
				CreateUser:    createUser,
				SkipProvision: skipProvision,
				Role: func() string {
					n := config.FindNode(nodeName)
					if n != nil {
						return n.Role
					}
					return ""
				}(),
				DryRun: config.IsDryRun(),
				Output: writer,
			})

			if err != nil {
				ch <- fmt.Sprintf("\n❌ Bootstrap failed: %v", err)
				return
			}
			ch <- "\n✅ Bootstrap complete!"
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// CleanHost removes a node's host key from the system known_hosts file.
func CleanHost(n config.Node) tea.Cmd {
	return CleanSystemHost(n.Name)
}

// CleanSystemHost removes entries for a hostname or IP from the system known_hosts file.
// If the identifier is a node name in the config, it also cleans its IP.
func CleanSystemHost(identifier string) tea.Cmd {
	return func() tea.Msg {
		var sb strings.Builder
		targetIP := identifier

		// Try to find node in configuration to get its IP
		if node := config.FindNode(identifier); node != nil {
			targetIP = node.IP
		}

		fmt.Fprintf(&sb, "🧹 Cleaning host key for %s (%s) from system known_hosts and kgg_known_hosts...\n", identifier, targetIP)

		// Clean by IP
		_ = provision.RemoveSystemHostKey(targetIP)
		_ = provision.RemoveHostKey(targetIP)

		// Also clean by identifier if it's different from IP (e.g. hostname)
		if identifier != targetIP {
			_ = provision.RemoveSystemHostKey(identifier)
			_ = provision.RemoveHostKey(identifier)
		}

		fmt.Fprintf(&sb, "✅ Done. You can now SSH to the host again.")
		return ResultMsg{Output: sb.String()}
	}
}
