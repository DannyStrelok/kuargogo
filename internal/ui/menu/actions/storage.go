package actions

import (
	"bytes"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/config"
)

// MountStorage mounts a secondary disk on a node via Ansible
func MountStorage(n config.Node, disk, mountPoint string, tags []string) tea.Cmd {
	return func() tea.Msg {
		// Validation
		if disk == "" || mountPoint == "" {
			return ResultMsg{Output: "❌ Error: Disk and Mount Point are required."}
		}

		var buf bytes.Buffer
		var logBuilder strings.Builder

		fmt.Fprintf(&logBuilder, "🔧 Mounting %s to %s on %s (%s) via Ansible...\n", disk, mountPoint, n.Name, n.IP)
		logBuilder.WriteString("⚠️  Warning: This will partition and format the disk (ext4) if not already done.\n\n")

		// Execute Playbook
		// We pass 'false' for dryRun here as this is an action triggered by the user to actually do work.
		// Ideally, we could expose a dry-run toggle in the UI too.
		result, err := ansible.RunMountStorage(n.Name, disk, mountPoint, config.IsDryRun(), tags, &buf)

		logBuilder.WriteString(buf.String())

		if err != nil {
			fmt.Fprintf(&logBuilder, "\n❌ Error: %v\n", err)
		} else if !result.Success {
			fmt.Fprintf(&logBuilder, "\n❌ Mount failed (exit code: %d)\n", result.ExitCode)
		} else {
			fmt.Fprintf(&logBuilder, "\n✅ Storage mounted successfully on %s!\n", n.Name)
			logBuilder.WriteString("📝 Added to /etc/fstab for persistence.")
		}

		return ResultMsg{Output: logBuilder.String()}
	}
}

// LonghornInit triggers the Ansible playbook to install Longhorn.
func LonghornInit() tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer
		buf.WriteString("🚀 Deploying Longhorn Storage System via Ansible...\n\n")

		result, err := ansible.RunLonghornInit(false, nil, &buf)

		if err != nil {
			fmt.Fprintf(&buf, "\n❌ Fatal Error: %v\n", err)
		} else if !result.Success {
			fmt.Fprintf(&buf, "\n❌ Playbook failed (exit code: %d)\n", result.ExitCode)
		} else {
			buf.WriteString("\n✅ Longhorn deployed successfully!\n(It may take a few minutes for pods to be fully ready.)")
		}

		return ResultMsg{Output: buf.String()}
	}
}

// LonghornStatus runs a check on the Longhorn pods.
func LonghornStatus() tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer
		buf.WriteString("🩺 Checking Longhorn Status...\n\n")

		result, err := ansible.RunLonghornStatus("", config.IsDryRun(), &buf)

		if err != nil {
			fmt.Fprintf(&buf, "\n❌ Fatal Error: %v\n", err)
		} else if !result.Success {
			fmt.Fprintf(&buf, "\n❌ Check failed (exit code: %d)\n", result.ExitCode)
		}

		return ResultMsg{Output: buf.String()}
	}
}
