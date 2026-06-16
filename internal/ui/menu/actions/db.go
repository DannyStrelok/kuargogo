package actions

import (
	"context"
	"fmt"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/cluster"
	"github.com/DannyStrelok/kuargogo/internal/config"
)

// OpsCreateCNPGBackup triggers and monitors a CNPG manual backup operation.
func OpsCreateCNPGBackup(clusterName, backupName, namespace string) tea.Cmd {
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

			_, _ = writer.Write([]byte(fmt.Sprintf("🚀 Triggering manual backup %q for cluster %q in namespace %q...\n", backupName, clusterName, namespace)))

			actualName, err := mgr.CreateCNPGBackup(master.IP, namespace, clusterName, backupName)
			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("❌ Error triggering backup: %v\n", err)))
				return
			}

			_, _ = writer.Write([]byte(fmt.Sprintf("✅ Backup CRD %q applied successfully. Monitoring progress...\n", actualName)))

			for {
				if config.IsDryRun() {
					_, _ = writer.Write([]byte("✨ [DRY RUN] CNPG backup completed successfully.\n"))
					break
				}

				status, err := mgr.GetCNPGBackupStatus(master.IP, namespace, actualName)
				if err != nil {
					_, _ = writer.Write([]byte(fmt.Sprintf("⚠️  Warning checking status: %v\n", err)))
				} else {
					_, _ = writer.Write([]byte(fmt.Sprintf("Status: %s\n", status)))
					if status == "completed" {
						_, _ = writer.Write([]byte("\n✅ Backup completed successfully in Cloudflare R2!\n"))
						break
					}
					if status == "failed" {
						_, _ = writer.Write([]byte("\n❌ Backup failed inside the CNPG operator.\n"))
						break
					}
				}

				time.Sleep(3 * time.Second)
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// OpsRestoreCNPGCluster triggers a PITR restore. If force is specified, it cleans up the active cluster and its PVCs first.
func OpsRestoreCNPGCluster(sourceCluster, targetCluster, namespace, timeStr string, force bool) tea.Cmd {
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

			// 1. Parse target time first
			parsedTime, err := cluster.ParseTargetTime(timeStr)
			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("❌ Error parsing target time: %v\n", err)))
				return
			}

			_, _ = writer.Write([]byte(fmt.Sprintf("🕒 Target Recovery Time: %s (UTC)\n", parsedTime.Format(time.RFC3339))))

			// 2. If force is active, perform safety backups and delete active cluster first
			if force {
				_, _ = writer.Write([]byte("\n⚠️ Force option enabled. Performing pre-restore tasks...\n"))

				// Trigger last-minute backup
				preBackupName := fmt.Sprintf("pre-restore-%s", time.Now().Format("20060102-150405"))
				_, _ = writer.Write([]byte(fmt.Sprintf("💾 Creating safety pre-restore backup: %s...\n", preBackupName)))

				actualPreName, backupErr := mgr.CreateCNPGBackup(master.IP, namespace, sourceCluster, preBackupName)
				if backupErr != nil {
					_, _ = writer.Write([]byte(fmt.Sprintf("⚠️ Warning: Failed to trigger pre-restore backup: %v. Proceeding anyway...\n", backupErr)))
				} else {
					// Wait for safety backup to finish (up to 2 minutes max to not hang the UI indefinitely, or normal completion)
					timeoutChan := time.After(2 * time.Minute)
					ticker := time.NewTicker(3 * time.Second)
					defer ticker.Stop()

				WaitLoop:
					for {
						select {
						case <-timeoutChan:
							_, _ = writer.Write([]byte("⚠️ Warning: Pre-restore backup timed out. Proceeding with restore...\n"))
							break WaitLoop
						case <-ticker.C:
							if config.IsDryRun() {
								break WaitLoop
							}
							status, checkErr := mgr.GetCNPGBackupStatus(master.IP, namespace, actualPreName)
							if checkErr != nil {
								_, _ = writer.Write([]byte(fmt.Sprintf("⚠️ Warning checking backup: %v\n", checkErr)))
							} else {
								_, _ = writer.Write([]byte(fmt.Sprintf("Safety backup status: %s\n", status)))
								if status == "completed" {
									_, _ = writer.Write([]byte("✅ Safety backup completed successfully!\n"))
									break WaitLoop
								}
								if status == "failed" {
									_, _ = writer.Write([]byte("⚠️ Warning: Safety backup failed inside CNPG. Proceeding with restore anyway...\n"))
									break WaitLoop
								}
							}
						}
					}
				}

				// Deleting active cluster
				_, _ = writer.Write([]byte(fmt.Sprintf("🗑️ Deleting active cluster %q to allow in-place restore...\n", sourceCluster)))
				err = mgr.DeleteCNPGCluster(master.IP, namespace, sourceCluster)
				if err != nil {
					_, _ = writer.Write([]byte(fmt.Sprintf("❌ Error deleting active cluster/PVCs: %v\n", err)))
					return
				}
				_, _ = writer.Write([]byte("✅ Active cluster and PVCs removed successfully.\n"))
			}

			// 3. Get original cluster info to inherit spec (e.g. S3 Barman details, instances, parameters)
			_, _ = writer.Write([]byte(fmt.Sprintf("🔍 Retrieving source cluster %q configuration...\n", sourceCluster)))
			sourceClusterMap, err := mgr.GetCNPGCluster(master.IP, namespace, sourceCluster)
			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("❌ Error fetching source cluster info: %v\n", err)))
				return
			}

			// 4. Generate PITR bootstrap manifest
			_, _ = writer.Write([]byte("⚙️ Generating Point-in-Time Recovery (PITR) manifest...\n"))
			manifestJSON, err := cluster.GeneratePITRManifest(sourceClusterMap, targetCluster, parsedTime)
			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("❌ Error generating manifest: %v\n", err)))
				return
			}

			// 5. Apply manifest
			_, _ = writer.Write([]byte(fmt.Sprintf("🚀 Applying restored cluster %q in namespace %q...\n", targetCluster, namespace)))
			err = mgr.ApplyCNPGClusterManifest(master.IP, namespace, []byte(manifestJSON))
			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("❌ Error applying manifest: %v\n", err)))
				return
			}

			_, _ = writer.Write([]byte(fmt.Sprintf("\n✅ Recovery cluster %q successfully deployed!\n", targetCluster)))
			_, _ = writer.Write([]byte("⏳ The CNPG operator will now provision the nodes and bootstrap data from Cloudflare R2 Barman store.\n"))

			// 6. GitOps warning
			_, _ = writer.Write([]byte("\n⚠️  [GITOPS WARNING]: Since this cluster is managed via GitOps (ArgoCD/Flux),\n"))
			_, _ = writer.Write([]byte("    please remember to update the cluster manifest in Git or pause auto-sync\n"))
			_, _ = writer.Write([]byte("    for this resource to prevent GitOps from overwriting the recovery state.\n"))
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// OpsListCNPGBackups retrieves the list of CNPG backups and writes them formatted to the progress channel.
func OpsListCNPGBackups(clusterName, namespace string) tea.Cmd {
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

			_, _ = writer.Write([]byte(fmt.Sprintf("🔍 Listing CNPG backups for cluster %q in namespace %q...\n", clusterName, namespace)))

			if config.IsDryRun() {
				_, _ = writer.Write([]byte("\n💾 [DRY RUN] Backups list:\n"))
				_, _ = writer.Write([]byte("--------------------------------------------------------------------------------\n"))
				_, _ = writer.Write([]byte("BACKUP NAME                         STATUS          CREATED AT               \n"))
				_, _ = writer.Write([]byte("--------------------------------------------------------------------------------\n"))
				_, _ = writer.Write([]byte("clandestino-db-daily-backup-20260613 completed       2026-06-16T18:05:23Z     \n"))
				_, _ = writer.Write([]byte("--------------------------------------------------------------------------------\n"))
				return
			}

			backups, err := mgr.ListCNPGBackups(master.IP, namespace, clusterName)
			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("❌ Error listing backups: %v\n", err)))
				return
			}

			if len(backups) == 0 {
				_, _ = writer.Write([]byte("\n📭 No backups found for this cluster.\n"))
				return
			}

			// Sort backups descending by creation timestamp (newest first)
			sort.Slice(backups, func(i, j int) bool {
				t1, err1 := time.Parse(time.RFC3339, backups[i].CreatedAt)
				t2, err2 := time.Parse(time.RFC3339, backups[j].CreatedAt)
				if err1 == nil && err2 == nil {
					return t1.After(t2)
				}
				return backups[i].CreatedAt > backups[j].CreatedAt
			})

			_, _ = writer.Write([]byte("\n💾 AVAILABLE CNPG BACKUPS:\n"))
			_, _ = writer.Write([]byte("--------------------------------------------------------------------------------\n"))
			_, _ = writer.Write([]byte(fmt.Sprintf("%-50s %-12s %-20s\n", "BACKUP NAME", "STATUS", "CREATED AT")))
			_, _ = writer.Write([]byte("--------------------------------------------------------------------------------\n"))
			for _, b := range backups {
				_, _ = writer.Write([]byte(fmt.Sprintf("%-50s %-12s %-20s\n", b.Name, b.Phase, b.CreatedAt)))
			}
			_, _ = writer.Write([]byte("--------------------------------------------------------------------------------\n"))
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// OpsCNPGTunnel starts an SSH port-forwarding tunnel to the CNPG database and prints credentials.
func OpsCNPGTunnel(clusterName, namespace string, localPort int) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan string, 10)

		ctx, cancel := context.WithCancel(context.Background())
		RegisterTunnel(cancel)

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

			tm := cluster.NewTunnelManager(writer)

			_, _ = writer.Write([]byte("🚀 Initializing Local Access to CNPG Database...\n"))

			// Query credentials to show to the user
			if !config.IsDryRun() {
				keyPath, err := cfg.SSH.ExpandedKeyPath()
				if err == nil {
					mgr := cluster.NewManager(master.User, keyPath, cfg.SSH.Port, false)
					password, err := mgr.GetCNPGAppUserPassword(master.IP, namespace, clusterName)
					if err == nil {
						_, _ = writer.Write([]byte("\n🔐 DATABASE CREDENTIALS (App User):\n"))
						_, _ = writer.Write([]byte(fmt.Sprintf("   Host:      localhost\n")))
						_, _ = writer.Write([]byte(fmt.Sprintf("   Port:      %d\n", localPort)))
						_, _ = writer.Write([]byte(fmt.Sprintf("   User:      app\n")))
						_, _ = writer.Write([]byte(fmt.Sprintf("   Password:  %s\n\n", password)))
					}
				}
			} else {
				_, _ = writer.Write([]byte("\n🔐 DATABASE CREDENTIALS (App User - [DRY RUN]):\n"))
				_, _ = writer.Write([]byte(fmt.Sprintf("   Host:      localhost\n")))
				_, _ = writer.Write([]byte(fmt.Sprintf("   Port:      %d\n", localPort)))
				_, _ = writer.Write([]byte(fmt.Sprintf("   User:      app\n")))
				_, _ = writer.Write([]byte(fmt.Sprintf("   Password:  dummy-dryrun-password\n\n")))
			}

			err := tm.StartCNPGTunnel(ctx, localPort, namespace, clusterName)
			if err != nil {
				_, _ = writer.Write([]byte(fmt.Sprintf("\n❌ Failed starting tunnel: %v", err)))
				return
			}

			_, _ = writer.Write([]byte("\n🔓 Database Access ACTIVE"))
			_, _ = writer.Write([]byte("\n----------------------------------------"))
			_, _ = writer.Write([]byte("\nPress 'esc' or 'q' to close the tunnel and return."))

			<-ctx.Done()
			_, _ = writer.Write([]byte("\n🔌 Tunnel closed.\n"))
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}


