package actions

import (
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/observability"
	"github.com/DannyStrelok/kuargogo/internal/provision"
)

// ClusterDashboard fetches CPU, Memory and Disk metrics for all K3s nodes
// via the Prometheus API client tunneled through SSH to a master node.
func ClusterDashboard() tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		kp, err := cfg.SSH.ExpandedKeyPath()
		if err != nil {
			return DashboardMsg{Nodes: []DashboardNodeSnapshot{
				{Name: "error", Error: "SSH key path: " + err.Error()},
			}}
		}

		// Find a master node to tunnel through
		var masterNode *config.Node
		for _, n := range cfg.Nodes {
			if n.Role == "master" || n.Role == "control-plane" {
				masterNode = &n
				break
			}
		}
		if masterNode == nil {
			return DashboardMsg{Nodes: []DashboardNodeSnapshot{
				{Name: "error", Error: "No master node found in config"},
			}}
		}

		executor, err := provision.NewExecutor(masterNode.User, kp, false)
		if err != nil {
			return DashboardMsg{Nodes: []DashboardNodeSnapshot{
				{Name: "error", Error: "SSH connection: " + err.Error()},
			}}
		}
		// Suppress TUI stdout corruption
		executor.Stdout = io.Discard
		executor.Stderr = io.Discard

		promClient := observability.NewClient(executor, masterNode.IP, cfg.SSH.Port)

		var snapshots []DashboardNodeSnapshot

		for _, node := range cfg.Nodes {
			if node.Role == "infra-manager" {
				continue // Skip Raspberry Pi
			}

			snap := DashboardNodeSnapshot{Name: node.Name}

			results, err := promClient.GetNodeMetrics(node.Name, node.IP)
			if err != nil {
				snap.Error = err.Error()
				snapshots = append(snapshots, snap)
				continue
			}

			// Map results
			for _, r := range results {
				switch r.Name {
				case "CPU Usage":
					if r.Error == nil {
						var val float64
						_, _ = parseFloat(r.Result, &val)
						snap.CPU = val
					}
				case "Memory Usage":
					if r.Error == nil {
						var val float64
						_, _ = parseFloat(r.Result, &val)
						snap.Memory = val
					}
				case "Disk Usage (/)":
					if r.Error == nil {
						var val float64
						_, _ = parseFloat(r.Result, &val)
						snap.Disk = val
					}
				}
			}

			snapshots = append(snapshots, snap)
		}

		return DashboardMsg{Nodes: snapshots}
	}
}

// parseFloat extracts a float from a string like "42.3%"
func parseFloat(s string, val *float64) (int, error) {
	return fmt.Sscanf(s, "%f", val)
}
