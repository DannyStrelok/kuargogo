package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/deps"
	"github.com/DannyStrelok/kuargogo/internal/infra"
	"github.com/spf13/cobra"
)

var infraCmd = &cobra.Command{
	Use:   "infra [init|bot|heartbeat]",
	Short: "Manage Infrastructure Manager (Raspberry Pi)",
	Long: `Provision the Raspberry Pi infrastructure manager, update the Telegram bot, or perform health checks.

Actions:
  init         Full provision (installs deps, hardening, services, kgg binary)
  bot          Fast update: Deploys updated kgg_telegram.py and kgg binary only
  health       Perform cluster-wide health check (alias: heartbeat)
  heartbeat    Alias for health

Examples:
  kgg infra init                    # Full provision
  kgg infra heartbeat --ai          # Health check with AI analysis`,
	Args: cobra.ExactArgs(1),
	RunE: runInfra,
}

func init() {
	infraCmd.Flags().String("tags", "", "Comma-separated Ansible tags to run (init only)")
	infraCmd.Flags().Bool("ai", false, "Enable AI analysis of heartbeat logs")
	infraCmd.Flags().Bool("json", false, "Output health report in JSON format")
	rootCmd.AddCommand(infraCmd)
}

func runInfra(cmd *cobra.Command, args []string) error {
	action := args[0]

	// Find Infra Node
	var infraNode *config.Node
	cfg := config.GetConfig()
	for _, n := range cfg.Nodes {
		if n.Role == "infra-manager" {
			infraNode = &n
			break
		}
	}

	if infraNode == nil {
		return fmt.Errorf("no node with role 'infra-manager' found in config")
	}

	switch action {
	case "init":
		return runInfraInit(cmd, infraNode)
	case "bot":
		return runInfraBotUpdate(infraNode)
	case "health", "heartbeat":
		return runInfraHealth(cmd, infraNode)
	default:
		return fmt.Errorf("unknown action %q, use: init | bot | heartbeat", action)
	}
}

func runInfraHealth(cmd *cobra.Command, node *config.Node) error {
	cfg := config.GetConfig()
	aiFlag, _ := cmd.Flags().GetBool("ai")

	keyPath, err := cfg.SSH.ExpandedKeyPath()
	if err != nil {
		return fmt.Errorf("error expanding SSH key path: %w", err)
	}

	mgr := infra.NewManager(node.User, keyPath, cfg.SSH.Port, config.IsDryRun())
	jsonFlag, _ := cmd.Flags().GetBool("json")

	if jsonFlag {
		mgr.Output = io.Discard
	} else {
		mgr.Output = os.Stdout
		fmt.Println("💓 Starting Diagnostic Health Check...")
	}
	report, err := mgr.RunHealthCheck(aiFlag)
	if err != nil {
		return err
	}

	if jsonFlag {
		jsonData, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(jsonData))
		return nil
	}

	fmt.Println("\n--- Heartbeat Report ---")
	for _, n := range report.Nodes {
		statusIcon := "✅"
		if n.Status != "ONLINE" {
			statusIcon = "❌"
		}
		fmt.Printf("%s %-15s [%s] CPU: %s (%s) RAM: %-15s Disk: %s\n", statusIcon, n.NodeName, n.Status, n.CPUUsage, n.CPUTemp, n.RAMUsage, n.DiskUsage)
		if n.Error != nil {
			fmt.Printf("   ⚠️ Error: %v\n", n.Error)
		}
		for svc, st := range n.Services {
			svcIcon := "🟢"
			if st != "active" {
				svcIcon = "🔴"
			}
			fmt.Printf("   %s %-10s: %s\n", svcIcon, svc, st)
		}
	}
	fmt.Println("------------------------")
	if report.Summary != "" {
		fmt.Println("\n🤖 AI Insight:")
		fmt.Println(report.Summary)
		fmt.Println("------------------------")
	}

	if len(report.RepairActions) > 0 {
		fmt.Printf("\n✨ AI has suggested %d repairs:\n", len(report.RepairActions))
		for i, action := range report.RepairActions {
			fmt.Printf(" [%d] %s\n", i+1, action)
		}

		fmt.Print("\nApply these repairs? [y/N]: ")
		var response string
		_, _ = fmt.Scanln(&response)

		if strings.ToLower(response) == "y" {
			for _, action := range report.RepairActions {
				if err := mgr.ExecuteRepairAction(action); err != nil {
					fmt.Printf("❌ Failed to execute %s: %v\n", action, err)
				}
			}
			fmt.Println("✅ All selected repairs completed.")
		} else {
			fmt.Println("⚠️ Repairs skipped.")
		}
	}

	return nil
}

func runInfraBotUpdate(node *config.Node) error {
	if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
		return err
	}

	extraVars, err := ansible.PreprocessInfraVars(node)
	if err != nil {
		return err
	}

	fmt.Printf("🚀 Fast-tracking Telegram Bot & KGG binary update on %s (%s)...\n", node.Name, node.IP)

	result, err := ansible.RunInfraBotUpdate(DryRun, extraVars, os.Stdout)
	if err != nil && result == nil {
		return fmt.Errorf("bot update failed: %w", err)
	}

	if result != nil && result.Success {
		fmt.Println("\n✅ Telegram Bot and KGG binary updated successfully!")
	}

	return nil
}
func runInfraInit(cmd *cobra.Command, node *config.Node) error {
	if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
		return err
	}

	tagsFlag, _ := cmd.Flags().GetString("tags")
	var tags []string
	if tagsFlag != "" {
		tags = strings.Split(tagsFlag, ",")
	}

	extraVars, err := ansible.PreprocessInfraVars(node)
	if err != nil {
		return err
	}

	fmt.Printf("Provisioning Infrastructure Manager on %s (%s) via Ansible...\n", node.Name, node.IP)

	result, err := ansible.RunInfraInit(DryRun, tags, node.Name, extraVars, os.Stdout)
	if err != nil && result == nil {
		return fmt.Errorf("infra init failed: %w", err)
	}

	if result != nil && result.Success {
		fmt.Println("\n✅ Infrastructure Manager provisioned successfully!")
	}

	return nil
}
