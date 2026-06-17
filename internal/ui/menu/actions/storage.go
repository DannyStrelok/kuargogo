package actions

import (
	"fmt"

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

		ch := make(chan string, 10)

		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			_, _ = writer.Write([]byte(fmt.Sprintf("🔧 Mounting %s to %s on %s (%s) via Ansible...\n", disk, mountPoint, n.Name, n.IP)))
			_, _ = writer.Write([]byte("⚠️  Warning: This will partition and format the disk (ext4) if not already done.\n\n"))

			result, err := ansible.RunMountStorage(n.Name, disk, mountPoint, config.IsDryRun(), tags, writer)

			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("\n❌ Error: %v\n", err)))
			} else if !result.Success {
				_, _ = writer.Write([]byte(fmt.Sprintf("\n❌ Mount failed (exit code: %d)\n", result.ExitCode)))
			} else {
				_, _ = writer.Write([]byte(fmt.Sprintf("\n✅ Storage mounted successfully on %s!\n", n.Name)))
				_, _ = writer.Write([]byte("⚠️  Warning: Added to /etc/fstab for persistence.\n"))
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// LonghornInit triggers the Ansible playbook to install Longhorn.
func LonghornInit() tea.Cmd {
	return func() tea.Msg {
		ch := make(chan string, 10)

		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)
			_, _ = writer.Write([]byte("🚀 Deploying Longhorn Storage System via Ansible...\n\n"))

			result, err := ansible.RunLonghornInit(config.IsDryRun(), nil, writer)

			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("\n❌ Fatal Error: %v\n", err)))
			} else if !result.Success {
				_, _ = writer.Write([]byte(fmt.Sprintf("\n❌ Playbook failed (exit code: %d)\n", result.ExitCode)))
			} else {
				_, _ = writer.Write([]byte("\n✅ Longhorn deployed successfully!\n(It may take a few minutes for pods to be fully ready.)\n"))
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// LonghornStatus runs a check on the Longhorn pods.
func LonghornStatus() tea.Cmd {
	return func() tea.Msg {
		ch := make(chan string, 10)

		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)
			_, _ = writer.Write([]byte("🩺 Checking Longhorn Status...\n\n"))

			result, err := ansible.RunLonghornStatus("", config.IsDryRun(), writer)

			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("\n❌ Fatal Error: %v\n", err)))
			} else if !result.Success {
				_, _ = writer.Write([]byte(fmt.Sprintf("\n❌ Check failed (exit code: %d)\n", result.ExitCode)))
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}
