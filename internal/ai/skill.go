package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// GenerateSkillMD creates a machine-readable markdown file that describes the cluster.
// The file is stored in ~/.kuargogo/skill.md by default.
func GenerateSkillMD() (string, error) {
	cfg := config.GetConfig()
	contextName := config.GetCurrentContext()

	var sb strings.Builder

	_, _ = sb.WriteString("# kuargogo Cluster Skill Context\n\n")
	_, _ = fmt.Fprintf(&sb, "Active Context: `%s`\n", contextName)
	_, _ = sb.WriteString("This file provides context for AI agents interacting with this specific kuargogo managed cluster.\n\n")

	sb.WriteString("## Infrastructure Overview\n\n")
	sb.WriteString("| Node Name | IP Address | Role | Arch | Labels |\n")
	sb.WriteString("|-----------|------------|------|------|--------|\n")
	for _, n := range cfg.Nodes {
		labels := ""
		for k, v := range n.Labels {
			labels += fmt.Sprintf("%s=%s ", k, v)
		}
		_, _ = fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s |\n", n.Name, n.IP, n.Role, n.Arch, strings.TrimSpace(labels))
	}
	_, _ = sb.WriteString("\n")
	_, _ = sb.WriteString("## Network & Control Plane\n")
	_, _ = fmt.Fprintf(&sb, "- **Switch IP**: `%s` (Driver: %s)\n", cfg.Network.SwitchIP, cfg.Network.Driver)
	_, _ = fmt.Fprintf(&sb, "- **K3s VIP**: `%s` (HA: %v)\n", cfg.K3s.VIP, cfg.K3s.HA)
	_, _ = fmt.Fprintf(&sb, "- **SSH Port**: `%d`\n\n", cfg.SSH.Port)

	if len(cfg.GitOps.Projects) > 0 {
		sb.WriteString("## GitOps Projects\n\n")
		for _, p := range cfg.GitOps.Projects {
			_, _ = fmt.Fprintf(&sb, "### Project: %s\n", p.Name)
			if p.Description != "" {
				_, _ = fmt.Fprintf(&sb, "%s\n", p.Description)
			}
			_, _ = sb.WriteString("| App Name | Repository | Path | Namespace |\n")
			_, _ = sb.WriteString("|----------|------------|------|-----------|\n")
			for _, a := range p.Apps {
				_, _ = fmt.Fprintf(&sb, "| %s | %s | %s | %s |\n", a.Name, a.Repo, a.Path, a.Namespace)
			}
			_, _ = sb.WriteString("\n")
		}
	}

	sb.WriteString("## Available Commands (kuargogo)\n")
	sb.WriteString("- `kgg node ls`: List all nodes and status\n")
	sb.WriteString("- `kgg node ssh <node>`: SSH into a node\n")
	sb.WriteString("- `kgg pwr reboot <node>`: Reboot via hardware/switch\n")
	sb.WriteString("- `kgg gitops sync <project>`: Force GitOps synchronization\n")
	sb.WriteString("- `kgg infra heartbeat`: Run system-wide health check\n")
	sb.WriteString("- `kgg ai chat`: Start interactive AI session\n\n")

	sb.WriteString("## Instructions for AI Agents\n")
	sb.WriteString("1. **Verify State**: Always run `kgg node ls` before recommending infrastructure changes.\n")
	sb.WriteString("2. **Hardware First**: Use `kgg pwr` commands for hard reboots if a node is unresponsive via SSH.\n")
	sb.WriteString("3. **Declarative**: All changes should ideally be reflected in `kuargogo.yaml` and applied via `kgg update` or Ansible playbooks.\n")

	content := sb.String()

	// Write to ~/.kuargogo/skill.md
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	dir := filepath.Join(home, ".kuargogo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	filePath := filepath.Join(dir, "skill.md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write skill.md: %w", err)
	}

	return filePath, nil
}
