package cluster

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/config"
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

// RemediateNode orchestrates draining, resetting, and rejoining a K3s worker node.
func (m *Manager) RemediateNode(masterNode *config.Node, targetNodeName string, tags []string) error {
	// 1. Find Target Node in configuration
	var targetNode *config.Node
	cfg := config.GetConfig()
	for _, n := range cfg.Nodes {
		if n.Name == targetNodeName {
			targetNode = &n
			break
		}
	}
	if targetNode == nil {
		return fmt.Errorf("node %s not found in configuration", targetNodeName)
	}

	// 2. Step 1: Drain target node from K3s cluster
	_, _ = fmt.Fprintf(m.Output, "🔄 [Step 1/4] Draining node %s...\n", targetNodeName)
	drainResult, err := ansible.RunK3sDrain(masterNode.Name, targetNodeName, m.DryRun, tags, m.Output)
	if err != nil {
		return fmt.Errorf("drain failed: %w", err)
	}
	if !drainResult.Success {
		return fmt.Errorf("drain failed with exit status %d", drainResult.ExitCode)
	}
	_, _ = fmt.Fprintf(m.Output, "✅ Node %s drained.\n", targetNodeName)

	// 3. Step 2: Reset K3s on target node
	_, _ = fmt.Fprintf(m.Output, "🗑️  [Step 2/4] Resetting K3s on node %s...\n", targetNodeName)
	resetResult, err := ansible.RunK3sReset(targetNodeName, m.DryRun, tags, m.Output)
	if err != nil {
		return fmt.Errorf("reset failed: %w", err)
	}
	if !resetResult.Success {
		return fmt.Errorf("reset failed with exit status %d", resetResult.ExitCode)
	}
	_, _ = fmt.Fprintf(m.Output, "✅ K3s reset completed on %s.\n", targetNodeName)

	// 4. Step 3: Get Join Token from Master
	_, _ = fmt.Fprintf(m.Output, "🔑 [Step 3/4] Fetching join token from master %s...\n", masterNode.Name)
	token, err := m.GetMasterToken(masterNode.IP)
	if err != nil {
		return fmt.Errorf("failed to fetch join token: %w", err)
	}

	// 5. Step 4: Rejoin target node to K3s cluster
	role := "agent"
	if targetNode.Role == "master" || targetNode.Role == "control-plane" || targetNode.Role == "server" {
		role = "server"
	}
	_, _ = fmt.Fprintf(m.Output, "🔗 [Step 4/4] Rejoining node %s as %s...\n", targetNodeName, role)
	joinResult, err := ansible.RunK3sJoin(targetNodeName, masterNode.IP, token, role, cfg.K3s.VIP, m.DryRun, tags, m.Output)
	if err != nil {
		return fmt.Errorf("join failed: %w", err)
	}
	if !joinResult.Success {
		return fmt.Errorf("join failed with exit status %d", joinResult.ExitCode)
	}

	return nil
}

// UninstallObservability uninstalls the Helm observability stacks and deletes the monitoring namespace.
func (m *Manager) UninstallObservability(masterIP string) error {
	executor, err := m.getExecutor()
	if err != nil {
		return err
	}

	// 1. Delete namespace (this deletes Loki, Tempo, and Promtail, and Helm metadata secrets)
	// We run it with a timeout to avoid hanging forever
	_, _ = fmt.Fprintln(m.Output, "⏳ Deleting 'monitoring' namespace (this may take a few minutes)...")
	_, err = executor.ExecuteCommand(masterIP, m.Port, "sudo k3s kubectl delete namespace monitoring --timeout=5m")
	if err != nil {
		_, _ = fmt.Fprintf(m.Output, "⚠️ Warning during namespace deletion: %v\n", err)
	} else {
		_, _ = fmt.Fprintln(m.Output, "✅ Namespace 'monitoring' deleted.")
	}

	// 2. Clean up cluster-scoped resources left behind by kube-prometheus-stack, loki, tempo
	_, _ = fmt.Fprintln(m.Output, "🧹 Cleaning up cluster-scoped resources (ClusterRoles, Webhooks)...")
	
	// Clean ClusterRoles and ClusterRoleBindings with label
	cleanupCmds := []string{
		"sudo k3s kubectl delete clusterrole,clusterrolebinding,mutatingwebhookconfiguration,validatingwebhookconfiguration -l app.kubernetes.io/instance=kube-prometheus-stack --ignore-not-found",
		"sudo k3s kubectl delete clusterrole,clusterrolebinding -l app.kubernetes.io/instance=loki --ignore-not-found",
		"sudo k3s kubectl delete clusterrole,clusterrolebinding -l app.kubernetes.io/instance=tempo --ignore-not-found",
	}

	for _, cmd := range cleanupCmds {
		_, _ = executor.ExecuteCommand(masterIP, m.Port, cmd)
	}

	_, _ = fmt.Fprintln(m.Output, "✅ Cluster-scoped resources cleaned up.")
	return nil
}

