package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/cloudflare"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/deps"
	"github.com/DannyStrelok/kuargogo/internal/notify"
)

var opsNotify bool

var opsCmd = &cobra.Command{
	Use:   "ops",
	Short: "Operations commands powered by Ansible",
	Long: `Run Ansible-powered operations across all cluster nodes.
	
Requires ansible and ansible-playbook to be installed.
Playbooks are located in infrastructure/playbooks/.`,
}

var opsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Run maintenance playbook on all nodes",
	Long: `Executes the update.yml playbook to perform system maintenance:
- apt update && apt upgrade
- Clean up old packages
- Reboot if required

Examples:
  kgg ops update              # Run maintenance on all nodes
  kgg ops update --notify     # Run and send Telegram notification
  kgg ops update --dry-run    # Preview changes without applying`,
	RunE: runOpsUpdate,
}

var opsNfsCmd = &cobra.Command{
	Use:   "nfs",
	Short: "Configure NFS mounts from NAS to cluster nodes",
	Long: `Executes the setup-nfs.yml playbook to configure NFS mounts.

This will:
- Install NFS client packages
- Create mount points
- Configure /etc/fstab entries
- Mount the NFS shares

Examples:
  kgg ops nfs              # Configure NFS mounts
  kgg ops nfs --notify     # Configure and send Telegram notification
  kgg ops nfs --dry-run    # Preview changes`,
	RunE: runOpsNfs,
}

var opsObservabilityCmd = &cobra.Command{
	Use:   "observability",
	Short: "Deploy LGTM Stack (Prometheus, Grafana, Loki)",
	Long: `Executes the ops-observability.yml playbook to deploy the Observability Stack.
This installs kube-prometheus-stack and loki-stack.

Example:
  kgg ops observability --notify`,
	RunE: runOpsObservability,
}

var opsCloudflareCmd = &cobra.Command{
	Use:   "cloudflare-tunnel",
	Short: "Deploy Cloudflare Zero Trust and CertManager",
	Long: `Executes the ops-cloudflare.yml playbook to expose the homelab through Cloudflare Tunnels
and auto-configure Let's Encrypt Wildcard SSL certificates via DNS-01 API challenges.

Example:
  kgg ops cloudflare-tunnel --notify`,
	RunE: runOpsCloudflare,
}

var opsArgoCDCmd = &cobra.Command{
	Use:   "argocd",
	Short: "Deploy ArgoCD GitOps engine",
	Long: `Executes the ops-argocd.yml playbook to install the ArgoCD orchestrator.
This will enable declarative GitOps deployments for your applications.

Example:
  kgg ops argocd --notify`,
	RunE: runOpsArgoCD,
}

var opsKargoCmd = &cobra.Command{
	Use:   "kargo",
	Short: "Deploy Kargo Promotion engine",
	Long: `Executes the ops-kargo.yml playbook to install the Kargo Promotion engine.
This will enable application lifecycle and promotion management.

Example:
  kgg ops kargo --notify`,
	RunE: runOpsKargo,
}

var opsBackupCmd = &cobra.Command{
	Use:   "backup-system",
	Short: "Deploy Velero Disaster Recovery system",
	Long: `Executes the ops-velero.yml playbook to deploy Velero to the cluster.
It reads S3 credentials from your kuargogo.yaml under the "backup" key.

Example:
  kgg ops backup-system --notify`,
	RunE: runOpsBackup,
}

var opsMigrateUserCmd = &cobra.Command{
	Use:   "migrate-user",
	Short: "Migrate homelab nodes from rk-admin to kgg-admin",
	Long: `Executes the migrate-rk-to-kgg.yml playbook to transition cluster nodes
from the legacy rk-admin user to the new kgg-admin user safely.

Examples:
  kgg ops migrate-user                            # Migrate all cluster nodes
  kgg ops migrate-user --node 192.168.1.101      # Migrate a specific host by IP
  kgg ops migrate-user --old-user custom-admin   # Specify a custom source user`,
	RunE: runOpsMigrateUser,
}

func init() {
	rootCmd.AddCommand(opsCmd)
	opsCmd.PersistentFlags().BoolVar(&opsNotify, "notify", false, "Send Telegram notification on completion")
	opsCmd.AddCommand(opsUpdateCmd)
	opsCmd.AddCommand(opsNfsCmd)
	opsCmd.AddCommand(opsObservabilityCmd)

	opsCloudflareCmd.Flags().Bool("provision", false, "Automate Tunnel creation and token retrieval")
	opsCmd.AddCommand(opsCloudflareCmd)

	opsCmd.AddCommand(opsArgoCDCmd)
	opsCmd.AddCommand(opsKargoCmd)
	opsCmd.AddCommand(opsBackupCmd)

	opsMigrateUserCmd.Flags().String("node", "", "Limit migration to a specific node name or IP")
	opsMigrateUserCmd.Flags().String("old-user", "rk-admin", "Current SSH user on target nodes")
	opsMigrateUserCmd.Flags().String("new-user", "kgg-admin", "New admin user to create on target nodes")
	opsMigrateUserCmd.Flags().String("ssh-key", "", "Path to custom SSH private key for connection/migration")
	opsCmd.AddCommand(opsMigrateUserCmd)
}

func sendNotification(result *ansible.Result) {
	if !opsNotify || result == nil {
		return
	}

	notifier := notify.NewTelegramNotifier()
	notifier.DryRun = DryRun

	if !notifier.IsConfigured() {
		fmt.Println("⚠️  Telegram not configured (set bot_token and admin_id in config)")
		return
	}

	if err := notifier.NotifyAnsibleResult(result); err != nil {
		fmt.Printf("⚠️  Failed to send Telegram notification: %v\n", err)
	} else {
		fmt.Println("📱 Telegram notification sent")
	}
}

func runOpsUpdate(cmd *cobra.Command, args []string) error {
	// Check dependencies
	if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
		return err
	}

	playbookDir, err := ansible.FindPlaybookDir()
	if err != nil {
		return err
	}
	runner := ansible.NewRunner(playbookDir)
	runner.DryRun = DryRun

	// Start spinner for visual feedback
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Running update playbook..."
	s.Start()

	// Capture output to buffer (for TUI compatibility)
	var buf bytes.Buffer
	runner.Output = &buf

	result, err := runner.Run("update.yml", "", nil)

	s.Stop()

	// Print captured output
	fmt.Print(buf.String())

	if err != nil {
		sendNotification(result)
		return fmt.Errorf("update failed: %w", err)
	}

	if result.Success {
		fmt.Printf("\n✅ Update completed successfully in %s\n", result.Duration.Round(time.Second))
	} else {
		fmt.Printf("\n❌ Update failed (exit code: %d) in %s\n", result.ExitCode, result.Duration.Round(time.Second))
	}

	sendNotification(result)
	return nil
}

func runOpsNfs(cmd *cobra.Command, args []string) error {
	// Check dependencies
	if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
		return err
	}

	playbookDir, err := ansible.FindPlaybookDir()
	if err != nil {
		return err
	}
	runner := ansible.NewRunner(playbookDir)
	runner.DryRun = DryRun

	// Start spinner
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Configuring NFS mounts..."
	s.Start()

	var buf bytes.Buffer
	runner.Output = &buf

	result, err := runner.Run("setup-nfs.yml", "", nil)

	s.Stop()

	fmt.Print(buf.String())

	if err != nil {
		sendNotification(result)
		return fmt.Errorf("NFS setup failed: %w", err)
	}

	if result.Success {
		fmt.Printf("\n✅ NFS setup completed successfully in %s\n", result.Duration.Round(time.Second))
	} else {
		fmt.Printf("\n❌ NFS setup failed (exit code: %d) in %s\n", result.ExitCode, result.Duration.Round(time.Second))
	}

	sendNotification(result)
	return nil
}

func runOpsBackup(cmd *cobra.Command, args []string) error {
	if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
		return err
	}

	cfg := config.GetConfig()
	if cfg.Backup.S3AccessKey == "" {
		fmt.Println("⚠️  Warning: No S3 credentials found in config (backup.s3_access_key).")
		fmt.Println("Velero will be installed without external cloud credentials.")
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Deploying Velero Disaster Recovery..."
	s.Start()

	var buf bytes.Buffer
	result, err := ansible.RunOpsBackup(DryRun, nil, nil, &buf)

	s.Stop()
	fmt.Print(buf.String())

	if err != nil {
		sendNotification(result)
		return fmt.Errorf("velero deployment failed: %w", err)
	}

	if result.Success {
		fmt.Printf("\n✅ Velero Disaster Recovery deployed successfully in %s\n", result.Duration.Round(time.Second))
	} else {
		fmt.Printf("\n❌ Velero deployment failed (exit code: %d) in %s\n", result.ExitCode, result.Duration.Round(time.Second))
	}

	sendNotification(result)
	return nil
}

func runOpsObservability(cmd *cobra.Command, args []string) error {
	if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
		return err
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Deploying Observability Stack (LGTM)..."
	s.Start()

	var buf bytes.Buffer
	result, err := ansible.RunOpsObservability(DryRun, nil, nil, &buf)

	s.Stop()
	fmt.Print(buf.String())

	if err != nil {
		sendNotification(result)
		return fmt.Errorf("observability deployment failed: %w", err)
	}

	if result.Success {
		fmt.Printf("\n✅ Observability Stack deployed successfully in %s\n", result.Duration.Round(time.Second))
	} else {
		fmt.Printf("\n❌ Observability deployment failed (exit code: %d) in %s\n", result.ExitCode, result.Duration.Round(time.Second))
	}

	sendNotification(result)
	return nil
}

func runOpsCloudflare(cmd *cobra.Command, args []string) error {
	if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
		return err
	}

	cfg := config.GetConfig()
	provision, _ := cmd.Flags().GetBool("provision")

	if provision {
		fmt.Println("🌐 Starting Cloudflare Tunnel automation...")
		if _, err := cloudflare.ProvisionAndSaveTunnel(cmd.Context(), os.Stdout); err != nil {
			return err
		}

		// Reload cfg so extraVars below pick up the persisted tunnel credentials.
		cfg = config.GetConfig()
		fmt.Println("✅ Cloudflare automation complete. Proceeding with deployment...")
	}

	// (Rest of the manual check logic)
	if cfg.Cloudflare.APIToken == "" || cfg.Cloudflare.TunnelToken == "" {
		fmt.Println("⚠️  Warning: Missing Cloudflare credentials in config (cloudflare.api_token or cloudflare.tunnel_token).")
		fmt.Println("Deploying without secrets...")
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Deploying Cloudflare Zero Trust and CertManager..."
	s.Start()

	var buf bytes.Buffer
	result, err := ansible.RunOpsCloudflare(DryRun, nil, nil, &buf)

	s.Stop()
	fmt.Print(buf.String())

	if err != nil {
		sendNotification(result)
		return fmt.Errorf("cloudflare deployment failed: %w", err)
	}

	if result.Success {
		fmt.Printf("\n✅ Cloudflare Tunnels and CertManager deployed successfully in %s\n", result.Duration.Round(time.Second))
	} else {
		fmt.Printf("\n❌ Cloudflare deployment failed (exit code: %d) in %s\n", result.ExitCode, result.Duration.Round(time.Second))
	}

	sendNotification(result)
	return nil
}

func runOpsArgoCD(cmd *cobra.Command, args []string) error {
	if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
		return err
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Deploying ArgoCD GitOps Engine..."
	s.Start()

	var buf bytes.Buffer
	result, err := ansible.RunOpsArgoCD(DryRun, nil, nil, &buf)

	s.Stop()
	fmt.Print(buf.String())

	if err != nil {
		sendNotification(result)
		return fmt.Errorf("argocd deployment failed: %w", err)
	}

	if result.Success {
		fmt.Printf("\n✅ ArgoCD GitOps Engine deployed successfully in %s\n", result.Duration.Round(time.Second))
	} else {
		fmt.Printf("\n❌ ArgoCD deployment failed (exit code: %d) in %s\n", result.ExitCode, result.Duration.Round(time.Second))
	}

	sendNotification(result)
	return nil
}

func runOpsMigrateUser(cmd *cobra.Command, args []string) error {
	if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
		return err
	}

	node, _ := cmd.Flags().GetString("node")
	if node != "" {
		cfg := config.GetConfig()
		found := false
		for _, n := range cfg.Nodes {
			if n.Name == node || n.IP == node {
				node = n.Name
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("node '%s' not found in kuargogo.yaml", node)
		}
	}

	oldUser, _ := cmd.Flags().GetString("old-user")
	newUser, _ := cmd.Flags().GetString("new-user")
	sshKey, _ := cmd.Flags().GetString("ssh-key")

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	suffix := " Migrating users to kgg-admin..."
	if node != "" {
		suffix = fmt.Sprintf(" Migrating user on node %s to kgg-admin...", node)
	}
	s.Suffix = suffix
	s.Start()

	var buf bytes.Buffer
	result, err := ansible.RunMigrateUser(node, oldUser, newUser, sshKey, DryRun, &buf)

	s.Stop()
	fmt.Print(buf.String())

	if err != nil {
		sendNotification(result)
		return fmt.Errorf("user migration failed: %w", err)
	}

	if result.Success {
		fmt.Printf("\n✅ User migration completed successfully in %s\n", result.Duration.Round(time.Second))
	} else {
		fmt.Printf("\n❌ User migration failed (exit code: %d) in %s\n", result.ExitCode, result.Duration.Round(time.Second))
	}

	sendNotification(result)
	return nil
}

func runOpsKargo(cmd *cobra.Command, args []string) error {
	if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
		return err
	}

	// 1. Ensure Kargo Admin Credentials
	cfg := config.GetConfig()
	if cfg.GitOps.KargoEngine == nil {
		_ = config.ModifyConfig(func(c *config.ClusterConfig) {
			c.GitOps.KargoEngine = &config.KargoEngine{}
		})
		cfg.GitOps.KargoEngine = &config.KargoEngine{}
	}

	if cfg.GitOps.KargoEngine.AdminPassword == "" || cfg.GitOps.KargoEngine.AdminPasswordHash == "" {
		fmt.Println("🔑 Generating secure Kargo admin credentials...")

		pass := string(cfg.GitOps.KargoEngine.AdminPassword)
		var err error
		if pass == "" {
			pass, err = config.GenerateRandomString(24)
			if err != nil {
				return fmt.Errorf("failed to generate secure password: %w", err)
			}
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash generation failed: %w", err)
		}

		_ = config.ModifyConfig(func(c *config.ClusterConfig) {
			if c.GitOps.KargoEngine == nil {
				c.GitOps.KargoEngine = &config.KargoEngine{}
			}
			c.GitOps.KargoEngine.AdminPassword = config.Secret(pass)
			c.GitOps.KargoEngine.AdminPasswordHash = string(hash)
		})
		_ = config.SaveConfig()
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Deploying Kargo Promotion Engine..."
	s.Start()

	var buf bytes.Buffer
	result, err := ansible.RunOpsKargo(DryRun, nil, nil, &buf)

	s.Stop()
	fmt.Print(buf.String())

	if err != nil {
		sendNotification(result)
		return fmt.Errorf("kargo deployment failed: %w", err)
	}

	if result.Success {
		finalCfg := config.GetConfig()
		passMsg := ""
		if finalCfg.GitOps.KargoEngine != nil {
			passMsg = fmt.Sprintf("\n🔑 Admin Password: %s", string(finalCfg.GitOps.KargoEngine.AdminPassword))
		}
		fmt.Printf("\n✅ Kargo Promotion Engine deployed successfully in %s%s\n", result.Duration.Round(time.Second), passMsg)
	} else {
		fmt.Printf("\n❌ Kargo deployment failed (exit code: %d) in %s\n", result.ExitCode, result.Duration.Round(time.Second))
	}

	sendNotification(result)
	return nil
}

