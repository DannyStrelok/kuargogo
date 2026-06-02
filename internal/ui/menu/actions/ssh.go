package actions

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/provision"
)

// SSHKeyGen generates a new cluster SSH key pair
func SSHKeyGen() tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		path, err := cfg.SSH.ExpandedKeyPath()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Error expanding key path: %v", err)}
		}

		// Check if key already exists
		if _, err := os.Stat(path); err == nil {
			return ResultMsg{Output: fmt.Sprintf("⚠️ Key already exists at %s\n\nTo overwrite, use CLI:\n  kgg ssh-keygen --output %s", path, path)}
		}

		if err := provision.GenerateClusterKey(path); err != nil {
			return ResultMsg{Output: fmt.Sprintf("Error generating key: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Key generated at %s\nPublic key: %s.pub", path, path)}
	}
}

// SSHCopy installs the public key to a remote node via password auth.
// If the cluster key doesn't exist, it is auto-generated first.
func SSHCopy(nodeIP, user, password string) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		basePath, err := cfg.SSH.ExpandedKeyPath()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("Error expanding key path: %v", err)}
		}

		// Auto-generate cluster key if missing
		generated, err := provision.EnsureClusterKey(basePath)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ %v", err)}
		}
		var output string
		if generated {
			output = fmt.Sprintf("🔑 Auto-generated cluster key at %s\n\n", basePath)
		}

		keyPath := basePath + ".pub"

		// Port fallback for robustness
		port := cfg.SSH.Port
		if port == 0 {
			port = 22
		}

		err = provision.InstallKey(nodeIP, port, user, password, keyPath)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("%s❌ Installation failed: %v", output, err)}
		}
		return ResultMsg{Output: fmt.Sprintf("%s✅ Success! Key installed to %s@%s", output, user, nodeIP)}
	}
}

// SSHConsole starts an interactive SSH session with the target node.
// This suspends the BubbleTea TUI and launches the system ssh client.
func SSHConsole(node config.Node) tea.Cmd {
	keyPath, err := config.ResolveKeyPath("")
	if err != nil {
		return func() tea.Msg { return ResultMsg{Output: fmt.Sprintf("Error resolving SSH key: %v", err)} }
	}

	port := config.GetConfig().SSH.Port
	if port == 0 {
		port = 22
	}

	// Build the SSH command
	c := exec.Command("ssh",
		"-i", keyPath,
		"-p", strconv.Itoa(port),
		"-o", "StrictHostKeyChecking=accept-new",
		fmt.Sprintf("%s@%s", node.User, node.IP),
	)

	// tea.ExecProcess suspends the UI and runs the external process
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ SSH session failed: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ SSH session with %s closed.", node.Name)}
	})
}

// BulkCleanHosts removes entries for multiple nodes and the K3s VIP from the system known_hosts file.
func BulkCleanHosts(nodes []config.Node) tea.Cmd {
	return func() tea.Msg {
		var sb strings.Builder
		cfg := config.GetConfig()
		fmt.Fprintf(&sb, "🧹 Performing Bulk Host Key Cleanup...\n")
		fmt.Fprintf(&sb, "===============================\n")

		// Clean the VIP if defined
		if cfg.K3s.VIP != "" {
			_ = provision.RemoveSystemHostKey(cfg.K3s.VIP)
			_ = provision.RemoveHostKey(cfg.K3s.VIP)
			fmt.Fprintf(&sb, "✅ VIP (%s): Cleaned from known_hosts and kgg_known_hosts\n", cfg.K3s.VIP)
		}

		for _, n := range nodes {
			// Clean by IP
			_ = provision.RemoveSystemHostKey(n.IP)
			_ = provision.RemoveHostKey(n.IP)

			// Also clean by name if different
			if n.Name != n.IP {
				_ = provision.RemoveSystemHostKey(n.Name)
				_ = provision.RemoveHostKey(n.Name)
			}
			fmt.Fprintf(&sb, "✅ %-15s: Cleaned entries\n", n.Name)
		}

		fmt.Fprintf(&sb, "\n✨ Done. You can now reconnect to these nodes and the VIP.")
		return ResultMsg{Output: sb.String()}
	}
}
