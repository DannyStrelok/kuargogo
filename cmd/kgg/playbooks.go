package main

import (
	"fmt"
	"os"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/spf13/cobra"
)

var playbooksCmd = &cobra.Command{
	Use:   "playbooks",
	Short: "Manage Ansible playbooks and roles",
}

var exportPlaybooksCmd = &cobra.Command{
	Use:   "export [names...]",
	Short: "Eject embedded playbooks into ~/.kuargogo/playbooks/",
	Long: `Copy embedded playbooks and roles to your local configuration folder for customization.
If no names are provided, use --all to export everything.`,
	Run: func(cmd *cobra.Command, args []string) {
		all, _ := cmd.Flags().GetBool("all")
		overwrite, _ := cmd.Flags().GetBool("force")

		var selected []string
		if all {
			var err error
			selected, err = ansible.ListAvailablePlaybooks()
			if err != nil {
				fmt.Printf("❌ Error listing playbooks: %v\n", err)
				os.Exit(1)
			}
		} else {
			if len(args) == 0 {
				fmt.Println("❌ Please specify playbook names or use --all")
				_ = cmd.Help()
				os.Exit(1)
			}
			selected = args
		}

		fmt.Printf("📤 Exporting %d items to ~/.kuargogo/playbooks/...\n", len(selected))
		summary, err := ansible.ExportPlaybooks(selected, overwrite)

		fmt.Println(summary)
		if err != nil {
			fmt.Printf("❌ Export failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Export complete!")
	},
}

func init() {
	exportPlaybooksCmd.Flags().Bool("all", false, "Export all available playbooks and roles")
	exportPlaybooksCmd.Flags().BoolP("force", "f", false, "Overwrite existing local files")
	playbooksCmd.AddCommand(exportPlaybooksCmd)
	rootCmd.AddCommand(playbooksCmd)
}
