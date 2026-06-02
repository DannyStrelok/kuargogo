package main

import (
	"fmt"
	"os"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/ui/wizard"
	"github.com/spf13/cobra"
)

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Run the setup wizard to generate configuration",
	Long:  `Runs an interactive wizard to configure kuargogo for your homelab.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if config already exists (unless --force)
		if !forceInit {
			configPath := config.GetConfigPath()
			if _, err := os.Stat(configPath); err == nil {
				fmt.Printf("Configuration already exists at %s.\n", configPath)
				fmt.Println("Use --force to run the wizard anyway (will add/update context).")
				return nil
			}
		}

		// Run the wizard
		if err := wizard.Run(os.Stdout); err != nil {
			return fmt.Errorf("wizard failed: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Run wizard even if config exists (adds/updates context)")
}
