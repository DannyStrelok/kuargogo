package main

import (
	"fmt"

	"github.com/DannyStrelok/kuargogo/internal/updater"
	"github.com/DannyStrelok/kuargogo/internal/version"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(versionCmd)
}

var updateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Update kuargogo to the latest version",
	Run:   runUpdate,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the current version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("kuargogo version %s\n", version.Current)
	},
}

func runUpdate(cmd *cobra.Command, args []string) {
	if version.Current == "dev" {
		fmt.Println("You are running a development version. Automatic updates are disabled.")
		return
	}

	info, found, err := updater.CheckUpdate(version.Current, "DannyStrelok/kuargogo")
	if err != nil {
		fmt.Printf("Error detecting update: %v\n", err)
		return
	}

	if !found {
		fmt.Println("Current version is the latest.")
		return
	}

	fmt.Printf("New version available: %s\n", info.Version)
	fmt.Printf("Release Notes:\n%s\n", info.ReleaseNotes)
	fmt.Print("\nDo you want to update? (y/n): ")

	var input string
	if _, err := fmt.Scanln(&input); err != nil && err.Error() != "unexpected newline" {
		fmt.Printf("Warning: input error: %v\n", err)
	}
	if input != "y" && input != "Y" {
		fmt.Println("Update cancelled.")
		return
	}

	fmt.Println("Downloading and updating...")
	if err := updater.PerformUpdate(info.AssetURL); err != nil {
		fmt.Printf("Update failed: %v\n", err)
		return
	}

	fmt.Printf("Successfully updated to %s\n", info.Version)
}
