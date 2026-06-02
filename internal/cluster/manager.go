package cluster

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/provision"
)

// Manager handles cluster operations
type Manager struct {
	User     string
	KeyPath  string
	Port     int
	DryRun   bool
	Output   io.Writer // Destination for progress messages (defaults to os.Stdout)
	executor *provision.Executor
}

// NewManager creates a new cluster manager
func NewManager(user, keyPath string, port int, dryRun bool) *Manager {
	return &Manager{
		User:    user,
		KeyPath: keyPath,
		Port:    port,
		DryRun:  dryRun,
		Output:  os.Stdout, // Default to stdout for backward compatibility
	}
}

// getExecutor lazily takes an executor
func (m *Manager) getExecutor() (*provision.Executor, error) {
	if m.executor != nil {
		return m.executor, nil
	}
	exec, err := provision.NewExecutor(m.User, m.KeyPath, m.DryRun)
	if err != nil {
		return nil, err
	}
	exec.Stdout = m.Output
	exec.Stderr = m.Output
	m.executor = exec
	return exec, nil
}

// GetMasterToken retrieves the join token from the master
func (m *Manager) GetMasterToken(ip string) (string, error) {
	if m.DryRun {
		return "K10abcdef1234567890::server:dry-run-token", nil
	}

	cmd := "sudo cat /var/lib/rancher/k3s/server/node-token"
	executor, err := m.getExecutor()
	if err != nil {
		return "", err
	}

	out, err := executor.ExecuteCommand(ip, m.Port, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	return strings.TrimSpace(out), nil
}

// GetLiveNodes returns a list of node names currently registered in K3s
func (m *Manager) GetLiveNodes(masterIP string) ([]string, error) {
	if m.DryRun {
		return []string{"node/lenovo2"}, nil
	}

	// Use a short timeout for live checks to keep TUI responsive
	cmd := "sudo k3s kubectl get nodes -o name --request-timeout=5s"
	executor, err := m.getExecutor()
	if err != nil {
		return nil, err
	}

	out, err := executor.ExecuteCommand(masterIP, m.Port, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get live nodes: %w", err)
	}

	rawNodes := strings.Split(strings.TrimSpace(out), "\n")
	var nodes []string
	for _, n := range rawNodes {
		name := strings.TrimSpace(strings.TrimPrefix(n, "node/"))
		if name != "" {
			nodes = append(nodes, name)
		}
	}

	return nodes, nil
}

