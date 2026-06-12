package actions

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/cluster"
	"github.com/DannyStrelok/kuargogo/internal/config"
)

// VeleroBackupsListMsg is sent when the backups query finishes.
type VeleroBackupsListMsg struct {
	Backups []cluster.VeleroBackup
	Err     error
}

// OpsListVeleroBackups lists backups as a Bubble Tea Cmd.
func OpsListVeleroBackups() tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		var master *config.Node
		for _, n := range cfg.Nodes {
			if n.Role == "master" || n.Role == "control-plane" {
				master = &n
				break
			}
		}
		if master == nil {
			return VeleroBackupsListMsg{Err: fmt.Errorf("no master node found in configuration")}
		}

		keyPath, err := cfg.SSH.ExpandedKeyPath()
		if err != nil {
			return VeleroBackupsListMsg{Err: err}
		}

		mgr := cluster.NewManager(master.User, keyPath, cfg.SSH.Port, config.IsDryRun())
		backups, err := mgr.ListVeleroBackups(master.IP)
		if err != nil {
			return VeleroBackupsListMsg{Err: err}
		}

		return VeleroBackupsListMsg{Backups: backups}
	}
}

// OpsStartVeleroRestore triggers and monitors a Velero restore operation.
func OpsStartVeleroRestore(backupName string, namespaces []string) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan string, 10)

		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			cfg := config.GetConfig()
			var master *config.Node
			for _, n := range cfg.Nodes {
				if n.Role == "master" || n.Role == "control-plane" {
					master = &n
					break
				}
			}
			if master == nil {
				_, _ = writer.Write([]byte("❌ Error: no master node found in configuration\n"))
				return
			}

			keyPath, err := cfg.SSH.ExpandedKeyPath()
			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("❌ Error: %v\n", err)))
				return
			}

			mgr := cluster.NewManager(master.User, keyPath, cfg.SSH.Port, config.IsDryRun())
			mgr.Output = writer

			_, _ = writer.Write([]byte(fmt.Sprintf("🚀 Starting restore from backup %q...\n", backupName)))

			restoreName, err := mgr.StartVeleroRestore(master.IP, backupName, namespaces)
			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("❌ Error starting restore: %v\n", err)))
				return
			}

			_, _ = writer.Write([]byte(fmt.Sprintf("✅ Restore resource created: %s\n", restoreName)))
			_, _ = writer.Write([]byte("⏳ Monitoring restoration progress...\n"))

			for {
				if config.IsDryRun() {
					_, _ = writer.Write([]byte("✨ [DRY RUN] Restore completed successfully.\n"))
					break
				}

				status, err := mgr.GetVeleroRestoreStatus(master.IP, restoreName)
				if err != nil {
					_, _ = writer.Write([]byte(fmt.Sprintf("⚠️  Warning checking status: %v\n", err)))
				} else {
					_, _ = writer.Write([]byte(fmt.Sprintf("Status: %s\n", status)))
					if status == "Completed" {
						_, _ = writer.Write([]byte("\n✅ Restore finished successfully!\n"))
						break
					}
					if status == "Failed" || status == "PartiallyFailed" {
						_, _ = writer.Write([]byte(fmt.Sprintf("\n❌ Restore completed with state: %s\n", status)))
						break
					}
				}

				time.Sleep(3 * time.Second)
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// OpsCreateVeleroBackup triggers and monitors a Velero backup operation.
func OpsCreateVeleroBackup(backupName string, namespaces []string, ttlStr string) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan string, 10)

		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			cfg := config.GetConfig()
			var master *config.Node
			for _, n := range cfg.Nodes {
				if n.Role == "master" || n.Role == "control-plane" {
					master = &n
					break
				}
			}
			if master == nil {
				_, _ = writer.Write([]byte("❌ Error: no master node found in configuration\n"))
				return
			}

			keyPath, err := cfg.SSH.ExpandedKeyPath()
			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("❌ Error: %v\n", err)))
				return
			}

			mgr := cluster.NewManager(master.User, keyPath, cfg.SSH.Port, config.IsDryRun())
			mgr.Output = writer

			_, _ = writer.Write([]byte(fmt.Sprintf("🚀 Starting manual backup %q...\n", backupName)))

			actualName, err := mgr.CreateVeleroBackup(master.IP, backupName, namespaces, ttlStr)
			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("❌ Error starting backup: %v\n", err)))
				return
			}

			_, _ = writer.Write([]byte(fmt.Sprintf("✅ Backup resource created: %s\n", actualName)))
			_, _ = writer.Write([]byte("⏳ Monitoring backup progress...\n"))

			for {
				if config.IsDryRun() {
					_, _ = writer.Write([]byte("✨ [DRY RUN] Backup completed successfully.\n"))
					break
				}

				status, err := mgr.GetVeleroBackupStatus(master.IP, actualName)
				if err != nil {
					_, _ = writer.Write([]byte(fmt.Sprintf("⚠️  Warning checking status: %v\n", err)))
				} else {
					_, _ = writer.Write([]byte(fmt.Sprintf("Status: %s\n", status)))
					if status == "Completed" {
						_, _ = writer.Write([]byte("\n✅ Backup finished successfully!\n"))
						break
					}
					if status == "Failed" || status == "PartiallyFailed" || status == "FailedValidation" {
						_, _ = writer.Write([]byte(fmt.Sprintf("\n❌ Backup completed with state: %s\n", status)))
						break
					}
				}

				time.Sleep(3 * time.Second)
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}
