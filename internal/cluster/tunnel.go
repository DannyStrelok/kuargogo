package cluster

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// TunnelManager handles SSH port-forwarding tunnels.
type TunnelManager struct {
	Output io.Writer
}

// NewTunnelManager creates a new TunnelManager.
func NewTunnelManager(output io.Writer) *TunnelManager {
	return &TunnelManager{Output: output}
}

// StartGrafanaTunnel starts an SSH tunnel to the Grafana service on the master node.
// It returns a cancel function to stop the tunnel.
func (tm *TunnelManager) StartGrafanaTunnel(ctx context.Context, localPort int) error {
	cfg := config.GetConfig()

	// Find the first master node — capture user from config to avoid hardcoding
	var masterIP, masterUser string
	for _, n := range cfg.Nodes {
		if n.Role == "master" {
			masterIP = n.IP
			masterUser = n.User
			break
		}
	}

	if masterIP == "" {
		return fmt.Errorf("no master node found in configuration")
	}
	if masterUser == "" {
		masterUser = "kgg-admin" // Sensible default if not set
	}

	keyPath, err := cfg.SSH.ExpandedKeyPath()
	if err != nil {
		return fmt.Errorf("failed to get SSH key path: %w", err)
	}

	// Command: ssh -i key -L localPort:localhost:3000 user@masterIP "kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80"
	// We use localhost:3000 on the remote side because kubectl will be listening there.

	sshCmd := []string{
		"-o", "StrictHostKeyChecking=no",
		"-i", keyPath,
		"-L", fmt.Sprintf("%d:localhost:3000", localPort),
		fmt.Sprintf("%s@%s", masterUser, masterIP),
		"sudo k3s kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80 --address 0.0.0.0",
	}

	_, _ = fmt.Fprintf(tm.Output, "🔌 Establishing secure tunnel to %s...\n", masterIP)

	cmd := exec.CommandContext(ctx, "ssh", sshCmd...)

	// We don't want to wait for the command to finish here, but we want to monitor its start.
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start SSH tunnel: %w", err)
	}

	// Wait a bit to see if it crashes immediately
	time.Sleep(2 * time.Second)
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return fmt.Errorf("tunnel exited prematurely")
	}

	_, _ = fmt.Fprintf(tm.Output, "✅ Tunnel active on http://localhost:%d\n", localPort)
	return nil
}

// OpenBrowser opens the specified URL in the default browser.
// Supports Windows, macOS and Linux transparently.
func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // linux and others
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}
