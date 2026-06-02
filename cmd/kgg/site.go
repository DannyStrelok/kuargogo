package main

import (
	"fmt"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/spf13/cobra"
)

var siteCmd = &cobra.Command{
	Use:   "site",
	Short: "Run full cluster deployment (provision → GPU → K3s → NFS)",
	Long: `Executes the site.yml master playbook which orchestrates the complete
cluster setup sequence:
  1. Provision all nodes (OS hardening, packages, firewall)
  2. GPU setup on capable nodes
  3. Initialize K3s on the first control-plane node
  4. Join remaining nodes to the cluster
  5. Configure NFS mounts on workers

Use --tags to run only specific phases (e.g., --tags k3s,init).`,
	Run: func(cmd *cobra.Command, _ []string) {
		tagsFlag, _ := cmd.Flags().GetString("tags")
		var tags []string
		if tagsFlag != "" {
			tags = strings.Split(tagsFlag, ",")
		}

		fmt.Println("🚀 Starting full cluster deployment...")
		fmt.Println()

		result, err := ansible.RunSite(DryRun, tags, nil)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return
		}

		if !result.Success {
			fmt.Printf("❌ Site deploy failed with exit code %d\n", result.ExitCode)
			return
		}

		fmt.Println()
		fmt.Printf("✅ Full cluster deployment completed in %s\n", result.Duration.Round(1e9))
	},
}

func init() {
	siteCmd.Flags().String("tags", "", "Comma-separated Ansible tags to run (e.g., k3s,init)")
	rootCmd.AddCommand(siteCmd)
}
