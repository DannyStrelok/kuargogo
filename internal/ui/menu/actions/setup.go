package actions

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/DannyStrelok/kuargogo/internal/deps"
	"github.com/DannyStrelok/kuargogo/internal/osutil"
)

// SetupAdminPC installs required dependencies on the host machine
func SetupAdminPC() tea.Cmd {
	return func() tea.Msg {
		if !osutil.IsAdmin() {
			return ResultMsg{
				Output: "❌ Error: This command requires administrative privileges.\n\n" +
					"Please restart the application as an Administrator (Windows: Right click -> Run as Administrator) " +
					"or using sudo (Linux/macOS).",
			}
		}

		// Create progress channel and start async task
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			_, _ = writer.Write([]byte("Starting Admin PC Setup...\n"))
			_, _ = writer.Write([]byte("--------------------------\n"))

			// 1. Install Ansible
			if err := deps.CheckDependency("ansible-playbook"); err == nil {
				_, _ = writer.Write([]byte("✅ Ansible is already installed.\n"))
			} else {
				_, _ = writer.Write([]byte("⏳ Installing Ansible...\n"))
				if err := deps.InstallAnsible(writer); err != nil {
					_, _ = fmt.Fprintf(writer, "❌ Failed to install Ansible: %v\n", err)
				} else {
					_, _ = writer.Write([]byte("✅ Ansible installed successfully!\n"))
				}
			}

			_, _ = writer.Write([]byte("--------------------------\n"))

			// 2. Install K9s
			if err := deps.CheckDependency("k9s"); err == nil {
				_, _ = writer.Write([]byte("✅ K9s is already installed.\n"))
			} else {
				_, _ = writer.Write([]byte("⏳ Installing K9s...\n"))
				if err := deps.InstallK9s(writer); err != nil {
					_, _ = fmt.Fprintf(writer, "❌ Failed to install K9s: %v\n", err)
				} else {
					_, _ = writer.Write([]byte("✅ K9s installed successfully!\n"))
				}
			}

			_, _ = writer.Write([]byte("--------------------------\n"))
			_, _ = writer.Write([]byte("Setup complete! You may need to restart your terminal for PATH changes to take effect.\n"))
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}
