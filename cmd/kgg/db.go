package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/DannyStrelok/kuargogo/internal/cluster"
	"github.com/DannyStrelok/kuargogo/internal/config"
)

var (
	dbNamespace   string
	dbBackupName  string
	dbRestoreTime string
	dbTargetName  string
	dbForce       bool
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database disaster recovery and backup utilities",
	Long:  `Manage CloudNativePG (CNPG) PostgreSQL database backups and Point-in-Time Recovery (PITR).`,
}

var dbBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Manage database backups",
}

var dbBackupCreateCmd = &cobra.Command{
	Use:   "create <cluster-name>",
	Short: "Trigger a manual CNPG database backup",
	Long:  `Creates a CNPG Backup resource on the cluster to execute an on-demand physical backup to S3/R2.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDBBackupCreate,
}

var dbBackupListCmd = &cobra.Command{
	Use:   "list <cluster-name>",
	Short: "List backups of a CNPG database",
	Long:  `Queries the cluster to list available backups for a specific database cluster.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDBBackupList,
}

var dbRestoreCmd = &cobra.Command{
	Use:   "restore <cluster-name>",
	Short: "Restore database state to a specific point in time (PITR)",
	Long: `Point-in-Time Recovery (PITR) for CloudNativePG database clusters.
Generates and applies a declarative recovery manifest pointing to the external backup object store.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDBRestore,
}

func init() {
	rootCmd.AddCommand(dbCmd)

	dbCmd.PersistentFlags().StringVarP(&dbNamespace, "ns", "n", "clandestino-dev", "Kubernetes namespace of the database cluster")

	dbCmd.AddCommand(dbBackupCmd)
	dbBackupCmd.AddCommand(dbBackupCreateCmd)
	dbBackupCmd.AddCommand(dbBackupListCmd)

	dbBackupCreateCmd.Flags().StringVar(&dbBackupName, "name", "", "Custom backup name (defaults to manual-<timestamp>)")

	dbCmd.AddCommand(dbRestoreCmd)
	dbRestoreCmd.Flags().StringVarP(&dbRestoreTime, "time", "t", "", "Point-in-Time target for recovery (RFC3339 format, e.g., '2026-06-12T18:00:00Z', or simple 'YYYY-MM-DD HH:MM:SS')")
	_ = dbRestoreCmd.MarkFlagRequired("time")
	dbRestoreCmd.Flags().StringVar(&dbTargetName, "target-name", "", "Name of the recovered cluster (defaults to <cluster-name>-pitr)")
	dbRestoreCmd.Flags().BoolVar(&dbForce, "force", false, "In-place restore: deletes active cluster and its PVCs first, then restores using the same name")
}

func getClusterManager() (*cluster.Manager, *config.Node, error) {
	cfg := config.GetConfig()
	var master *config.Node
	for _, n := range cfg.Nodes {
		if n.Role == "master" || n.Role == "control-plane" {
			master = &n
			break
		}
	}
	if master == nil {
		return nil, nil, fmt.Errorf("no master node found in configuration")
	}

	kp, err := cfg.SSH.ExpandedKeyPath()
	if err != nil {
		return nil, nil, err
	}

	mgr := cluster.NewManager(master.User, kp, cfg.SSH.Port, DryRun)
	return mgr, master, nil
}

func runDBBackupCreate(cmd *cobra.Command, args []string) error {
	clusterName := args[0]
	mgr, master, err := getClusterManager()
	if err != nil {
		return err
	}

	backupName := dbBackupName
	if backupName == "" {
		backupName = fmt.Sprintf("manual-%s", time.Now().Format("20060102-150405"))
	}

	fmt.Printf("🚀 Requesting manual backup %q for cluster %q in namespace %q...\n", backupName, clusterName, dbNamespace)
	actualName, err := mgr.CreateCNPGBackup(master.IP, dbNamespace, clusterName, backupName)
	if err != nil {
		return fmt.Errorf("failed to trigger backup: %w", err)
	}

	fmt.Printf("✅ Backup resource %q successfully applied in namespace %q.\n", actualName, dbNamespace)
	fmt.Println("⏳ Monitoring progress...")

	for {
		if DryRun {
			fmt.Println("✨ [DRY RUN] Backup completed successfully.")
			break
		}

		status, err := mgr.GetCNPGBackupStatus(master.IP, dbNamespace, actualName)
		if err != nil {
			fmt.Printf("⚠️  Warning checking status: %v\n", err)
		} else {
			fmt.Printf("Status: %s\n", status)
			if status == "completed" {
				fmt.Println("\n✅ Backup finished successfully!")
				break
			}
			if status == "failed" {
				return fmt.Errorf("backup completed with failure status")
			}
		}

		time.Sleep(3 * time.Second)
	}

	return nil
}

func runDBBackupList(cmd *cobra.Command, args []string) error {
	clusterName := args[0]
	mgr, master, err := getClusterManager()
	if err != nil {
		return err
	}

	backups, err := mgr.ListCNPGBackups(master.IP, dbNamespace, clusterName)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) == 0 {
		fmt.Printf("No backups found for cluster %q in namespace %q.\n", clusterName, dbNamespace)
		return nil
	}

	fmt.Printf("\n💾 AVAILABLE CNPG BACKUPS (Namespace: %s):\n", dbNamespace)
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Printf("%-35s %-15s %-25s\n", "BACKUP NAME", "STATUS", "CREATED AT")
	fmt.Println("--------------------------------------------------------------------------------")
	for _, b := range backups {
		fmt.Printf("%-35s %-15s %-25s\n", b.Name, b.Phase, b.CreatedAt)
	}
	fmt.Println("--------------------------------------------------------------------------------")

	return nil
}

func runDBRestore(cmd *cobra.Command, args []string) error {
	sourceCluster := args[0]
	mgr, master, err := getClusterManager()
	if err != nil {
		return err
	}

	parsedTime, err := cluster.ParseTargetTime(dbRestoreTime)
	if err != nil {
		return fmt.Errorf("failed to parse recovery time: %w", err)
	}

	targetCluster := dbTargetName
	if targetCluster == "" {
		targetCluster = fmt.Sprintf("%s-pitr", sourceCluster)
	}

	if dbForce {
		targetCluster = sourceCluster // In-place recovery forces name back to original

		// Double confirmation input to protect against catastrophic typos
		fmt.Printf("⚠️  DANGER: You are about to perform an IN-PLACE restore of cluster %q.\n", sourceCluster)
		fmt.Printf("This will DELETE the active cluster and all its volumes.\n")
		fmt.Printf("To confirm this action, please type the cluster name (%s): ", sourceCluster)
		
		if !DryRun {
			var input string
			_, err := fmt.Scanln(&input)
			if err != nil || input != sourceCluster {
				return fmt.Errorf("confirmation failed: cluster name did not match")
			}
		} else {
			fmt.Println("[DRY RUN] Bypassing manual confirmation scan.")
		}

		// Pre-restore backup for safety
		preBackupName := fmt.Sprintf("pre-restore-%s", time.Now().Format("20060102-150405"))
		fmt.Printf("💾 Creating safety pre-restore backup %q...\n", preBackupName)
		actualPreName, backupErr := mgr.CreateCNPGBackup(master.IP, dbNamespace, sourceCluster, preBackupName)
		if backupErr != nil {
			fmt.Printf("⚠️ Warning: Failed to trigger pre-restore backup: %v. Proceeding anyway...\n", backupErr)
		} else {
			// Poll safety backup (with a timeout of 2 mins max)
			timeoutChan := time.After(2 * time.Minute)
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()

		WaitLoop:
			for {
				select {
				case <-timeoutChan:
					fmt.Println("⚠️ Warning: Pre-restore backup timed out. Proceeding with restore...")
					break WaitLoop
				case <-ticker.C:
					if DryRun {
						break WaitLoop
					}
					status, checkErr := mgr.GetCNPGBackupStatus(master.IP, dbNamespace, actualPreName)
					if checkErr != nil {
						fmt.Printf("⚠️ Warning checking backup status: %v\n", checkErr)
					} else {
						fmt.Printf("Backup status: %s\n", status)
						if status == "completed" {
							fmt.Println("✅ Safety backup completed successfully!")
							break WaitLoop
						}
						if status == "failed" {
							fmt.Println("⚠️ Warning: Safety backup failed. Proceeding anyway...")
							break WaitLoop
						}
					}
				}
			}
		}

		// Delete cluster & PVCs
		fmt.Printf("🗑️  Deleting active database cluster %q...\n", sourceCluster)
		if err := mgr.DeleteCNPGCluster(master.IP, dbNamespace, sourceCluster); err != nil {
			return fmt.Errorf("failed to delete active cluster: %w", err)
		}
		fmt.Println("✅ Active cluster deleted successfully.")
	}

	fmt.Printf("🔍 Retrieving source cluster %q configuration...\n", sourceCluster)
	sourceMap, err := mgr.GetCNPGCluster(master.IP, dbNamespace, sourceCluster)
	if err != nil {
		return fmt.Errorf("failed to retrieve source cluster configuration: %w", err)
	}

	fmt.Println("⚙️  Generating recovery bootstrap manifest...")
	manifestJSON, err := cluster.GeneratePITRManifest(sourceMap, targetCluster, parsedTime)
	if err != nil {
		return fmt.Errorf("failed to generate recovery manifest: %w", err)
	}

	fmt.Printf("🚀 Applying recovery cluster %q in namespace %q...\n", targetCluster, dbNamespace)
	if err := mgr.ApplyCNPGClusterManifest(master.IP, dbNamespace, []byte(manifestJSON)); err != nil {
		return fmt.Errorf("failed to apply recovery cluster: %w", err)
	}

	fmt.Printf("\n✅ Recovery cluster %q successfully deployed!\n", targetCluster)
	fmt.Println("⏳ CNPG operator will bootstrap it from the external R2/S3 Barman object store.")
	fmt.Println("\n⚠️  [GITOPS WARNING]: Since this cluster is managed via GitOps (ArgoCD/Flux),")
	fmt.Println("    please remember to update the cluster manifest in Git or pause auto-sync")
	fmt.Println("    for this resource to prevent GitOps from overwriting the recovery state.")

	return nil
}
