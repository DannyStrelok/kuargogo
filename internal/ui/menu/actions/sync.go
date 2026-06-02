package actions

import (
	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// SyncPush triggers a cloud backup.
func SyncPush() tea.Cmd {
	return func() tea.Msg {
		if err := config.SyncPush(); err != nil {
			return ResultMsg{Output: "Backup failed: " + err.Error()}
		}
		return ResultMsg{Output: "âœ… Configuration backed up to cloud successfully."}
	}
}

// SyncPull triggers a cloud restore.
func SyncPull(masterPass string) tea.Cmd {
	return func() tea.Msg {
		if err := config.SyncPull(masterPass); err != nil {
			return ResultMsg{Output: "Restore failed: " + err.Error()}
		}
		return ResultMsg{Output: "âœ… Configuration restored from cloud. Restart CLI if needed."}
	}
}

// SyncLogout clears session.
func SyncLogout() tea.Cmd {
	return func() tea.Msg {
		provider, err := config.GetSyncProvider()
		if err != nil {
			return ResultMsg{Output: "Error: " + err.Error()}
		}
		if err := provider.Logout(); err != nil {
			return ResultMsg{Output: "Logout failed: " + err.Error()}
		}
		_ = config.ClearMasterKey()
		return ResultMsg{Output: "âœ… Logged out."}
	}
}
