package actions

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/i18n"
	"github.com/DannyStrelok/kuargogo/internal/ui/engine"
	"github.com/DannyStrelok/kuargogo/internal/updater"
	"github.com/DannyStrelok/kuargogo/internal/version"
)

// ContextSwitchedMsg signals a context was switched and should pop to root
type ContextSwitchedMsg struct {
	ContextName string
}

// ConfigSwitchContext switches the active context and saves the configuration.
// Returns a ContextSwitchedMsg that will trigger PopToRoot in the caller.
func ConfigSwitchContext(contextName string) tea.Cmd {
	return func() tea.Msg {
		// Update Current Context (Thread-Safe)
		if err := config.SwitchContext(contextName); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error switching context: %v", err)}
		}

		// Save to file
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error saving config: %v", err)}
		}

		// Return special message that triggers PopToRoot
		return ContextSwitchedMsg{ContextName: contextName}
	}
}

// HandleContextSwitched returns a batch command that shows a brief message then pops to root
func HandleContextSwitched(contextName string) tea.Cmd {
	return tea.Batch(
		tea.Printf("✅ Switched to context: %s", contextName),
		engine.PopToRoot(),
	)
}

// ConfigDeleteContext removes a context from the AppConfig and saves it
func ConfigDeleteContext(contextName string) tea.Cmd {
	return func() tea.Msg {
		err := config.DeleteContext(contextName)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error deleting context: %v", err)}
		}

		// Save the modified AppConfig
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Context deleted but failed to save config file: %v", err)}
		}

		return ResultMsg{Output: fmt.Sprintf("🗑️ Context '%s' deleted successfully.", contextName)}
	}
}

// ConfigSetK3sVIP updates the K3s HA Virtual IP and saves the configuration.
func ConfigSetK3sVIP(vip string) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.K3s.VIP = vip
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating VIP: %v", err)}
		}

		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ VIP updated but failed to save config file: %v", err)}
		}

		return ResultMsg{Output: fmt.Sprintf("✅ K3s HA Virtual IP set to: %s", vip)}
	}
}

// ConfigUpdateSSH updates SSH settings and saves the configuration.
func ConfigUpdateSSH(ssh config.SSH) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.SSH = ssh
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating SSH config: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		return ResultMsg{Output: "✅ SSH configuration updated successfully."}
	}
}

// ConfigUpdateNetwork updates network switch settings and saves the configuration.
func ConfigUpdateNetwork(net config.Network) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.Network = net
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating Network config: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		return ResultMsg{Output: "✅ Network configuration updated successfully."}
	}
}

// ConfigUpdateMQTT updates MQTT settings and saves the configuration.
func ConfigUpdateMQTT(mqtt config.MQTT) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.MQTT = mqtt
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating MQTT config: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		return ResultMsg{Output: "✅ MQTT configuration updated successfully."}
	}
}

// ConfigUpdateK3s updates general K3s settings and saves the configuration.
func ConfigUpdateK3s(k3s config.K3s) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.K3s = k3s
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating K3s config: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		return ResultMsg{Output: "✅ K3s configuration updated successfully."}
	}
}

// ConfigUpdateTelegram updates Telegram bot settings and saves the configuration.
func ConfigUpdateTelegram(tg config.Telegram) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.Telegram = tg
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating Telegram config: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		return ResultMsg{Output: "✅ Telegram configuration updated successfully."}
	}
}

// ConfigUpdateNFS updates NFS storage settings and saves the configuration.
func ConfigUpdateNFS(nfs config.NFS) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.NFS = nfs
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating NFS config: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		return ResultMsg{Output: "✅ NFS configuration updated successfully."}
	}
}

// ConfigUpdateDiscovery updates mDNS and network discovery settings.
func ConfigUpdateDiscovery(d config.DiscoveryConfig) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.Discovery = d
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating Discovery config: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		return ResultMsg{Output: "✅ Discovery configuration updated successfully."}
	}
}

// ConfigUpdateMaintenance toggles the global maintenance mode.
func ConfigUpdateMaintenance(enabled bool) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.MaintenanceMode = enabled
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating Maintenance Mode: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		status := "Enabled"
		if !enabled {
			status = "Disabled"
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Maintenance Mode %s successfully.", status)}
	}
}

// ConfigUpdateCloudflare updates Cloudflare settings and saves the configuration.
func ConfigUpdateCloudflare(cf config.Cloudflare) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.Cloudflare = cf
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating Cloudflare config: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		return ResultMsg{Output: "✅ Cloudflare configuration updated successfully."}
	}
}

// ConfigUpdateBackup updates Backup settings and saves the configuration.
func ConfigUpdateBackup(bk config.Backup) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.Backup = bk
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating Backup config: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		return ResultMsg{Output: "✅ Backup configuration updated successfully."}
	}
}

// ConfigUpdateHardware updates the hardware support setting and saves the configuration.
func ConfigUpdateHardware(enabled bool) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.HardwareEnabled = enabled
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating Hardware config: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		status := "Disabled"
		if enabled {
			status = "Enabled"
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Hardware support %s successfully.", status)}
	}
}

// ConfigUpdateAI updates AI settings and saves the configuration.
func ConfigUpdateAI(ai config.AIConfig) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.AI = ai
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating AI config: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		return ResultMsg{Output: "✅ AI configuration updated successfully."}
	}
}

// ConfigUpdateLang updates the global UI language and saves the configuration.
func ConfigUpdateLang(lang string) tea.Cmd {
	return func() tea.Msg {
		if err := config.ModifyAppConfig(func(cfg *config.RootConfig) {
			cfg.Lang = lang
		}); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating language: %v", err)}
		}

		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Language updated but failed to save: %v", err)}
		}
		i18n.SetLang(lang)
		return ResultMsg{Output: "✅ Language updated successfully. Some menus may require a restart to fully refresh."}
	}
}

// ConfigUpdateAnsible updates Ansible settings (WSLDistro, VaultPasswordFile) and saves the configuration.
func ConfigUpdateAnsible(ansible config.Ansible) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
			cfg.Ansible = ansible
		})
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating Ansible config: %v", err)}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Config updated but failed to save: %v", err)}
		}
		return ResultMsg{Output: "✅ Ansible configuration updated successfully."}
	}
}

// SelfUpdate checks for and applies updates to kuargogo.
func SelfUpdate() tea.Cmd {
	return func() tea.Msg {
		if version.Current == "dev" {
			return ResultMsg{Output: "⚠️  You are running a development version. Automatic updates are disabled."}
		}

		info, found, err := updater.CheckUpdate(version.Current, "DannyStrelok/kuargogo")
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error detecting update: %v", err)}
		}

		if !found {
			return ResultMsg{Output: "✅ Current version is the latest."}
		}

		// Since PerformUpdate is destructive (replaces binary), we should probably
		// just inform the user in TUI and let them do it or use tea.ExecProcess if we wanted
		// to show the same "y/n" prompt. But TUI actions should ideally be non-interactive
		// or use their own UI.
		// For simplicity and safety, we'll inform them and provide the command.
		return ResultMsg{Output: fmt.Sprintf("🚀 New version available: %s\n\nRelease Notes:\n%s\n\nTo update, run:\n  kgg self-update", info.Version, info.ReleaseNotes)}
	}
}
