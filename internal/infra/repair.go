package infra

import (
	"fmt"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/provision"
)

// ExecuteRepairAction translates a suggested 'kgg' command into internal Go logic.
// This is more efficient and safer than shell execution.
func (m *Manager) ExecuteRepairAction(action string) error {
	// Simple command parser
	parts := strings.Fields(action)
	if len(parts) < 2 || parts[0] != "kgg" {
		return fmt.Errorf("invalid repair command: %s", action)
	}

	cmd := parts[1]
	args := parts[2:]

	_, _ = fmt.Fprintf(m.Output, "🔨 Executing Repair: %s...\n", action)

	if m.DryRun {
		_, _ = fmt.Fprintf(m.Output, "[DRY RUN] Would execute internal logic for: %s\n", action)
		return nil
	}

	switch cmd {
	case "pwr":
		return m.handlePwrRepair(args)
	case "infra":
		return m.handleInfraRepair(args)
	case "node":
		return m.handleNodeRepair(args)
	case "cluster":
		return m.handleClusterRepair(args)
	default:
		return fmt.Errorf("unsupported repair command category: %s", cmd)
	}
}

func (m *Manager) handlePwrRepair(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("insufficient arguments for pwr: %v", args)
	}
	actionStr := args[0] // e.g. "reboot", "on", "off"
	targetName := args[1]

	cfg := config.GetConfig()
	var targetNode *config.Node
	for _, n := range cfg.Nodes {
		if n.Name == targetName {
			targetNode = &n
			break
		}
	}

	if targetNode == nil {
		return fmt.Errorf("node not found: %s", targetName)
	}

	action := provision.PowerAction(actionStr)
	executor, err := provision.NewExecutor(m.User, m.KeyPath, m.DryRun)
	if err != nil {
		return err
	}
	executor.Stdout = m.Output

	_, err = executor.RemotePowerControl(targetNode.IP, m.Port, action)
	return err
}

func (m *Manager) handleInfraRepair(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing infra action")
	}
	subCmd := args[0]
	if subCmd == "init" {
		_, _ = fmt.Fprintln(m.Output, "Note: This is a complex operation that usually requires Ansible. Redirecting to site initialization...")
		// In a real scenario, we might want to trigger the Ansible runner here
		return fmt.Errorf("automatic 'infra init' not fully implemented yet for AI remediation (run manually)")
	}
	return fmt.Errorf("unsupported infra repair sub-command: %s", subCmd)
}

func (m *Manager) handleNodeRepair(args []string) error {
	// kgg node bootstrap <node>
	if len(args) < 1 {
		return fmt.Errorf("missing node action")
	}
	// ... implementation ...
	return fmt.Errorf("node repairs not yet implemented for automated flow")
}

func (m *Manager) handleClusterRepair(args []string) error {
	// kgg cluster join ...
	return fmt.Errorf("cluster repairs not yet implemented for automated flow")
}
