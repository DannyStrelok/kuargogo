package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/DannyStrelok/kuargogo/internal/deps"
	"github.com/DannyStrelok/kuargogo/internal/osutil"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install required dependencies on this PC",
	Long:  `Automatically installs required external tools (Ansible, K9s) on this admin PC based on the Operating System.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Enforce administrative privileges for the setup command
		if err := osutil.EnsureAdmin(); err != nil {
			if errors.Is(err, osutil.ErrElevationTriggered) {
				fmt.Println("This command requires administrative privileges. Attempting to elevate...")
				os.Exit(0)
			}
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Starting Admin PC Setup...")
		fmt.Println("--------------------------")

		// 1. Install Ansible
		if err := deps.CheckDependency("ansible-playbook"); err == nil {
			fmt.Println("✅ Ansible is already installed.")
		} else {
			fmt.Println("⏳ Installing Ansible...")
			if err := deps.InstallAnsible(os.Stdout); err != nil {
				fmt.Printf("❌ Failed to install Ansible: %v\n", err)
			} else {
				fmt.Println("✅ Ansible installed successfully!")
			}
		}

		fmt.Println("--------------------------")

		// 2. Install K9s
		if err := deps.CheckDependency("k9s"); err == nil {
			fmt.Println("✅ K9s is already installed.")
		} else {
			fmt.Println("⏳ Installing K9s...")
			if err := deps.InstallK9s(os.Stdout); err != nil {
				fmt.Printf("❌ Failed to install K9s: %v\n", err)
			} else {
				fmt.Println("✅ K9s installed successfully!")
			}
		}

		fmt.Println("--------------------------")
		fmt.Println("Setup complete! You may need to restart your terminal for PATH changes to take effect.")
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
