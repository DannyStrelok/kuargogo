package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/deps"
	"github.com/DannyStrelok/kuargogo/internal/doctor"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run health diagnostics across all nodes",
	Long: `Collects system metrics from all cluster nodes and displays a diagnostic table.

Metrics collected:
- CPU Temperature
- Disk Usage
- Memory Usage  
- Load Average
- Uptime

Requires ansible to be installed.

Examples:
  kgg doctor              # Run diagnostics on all nodes
  kgg doctor --dry-run    # Show what would be collected
  kgg doctor --tags cpu   # Only collect CPU metrics`,
	RunE: runDoctor,
}

var doctorConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Validate kuargogo.yaml configuration and network connectivity",
	Long: `Runs logical checks and tests SSH port reachability for all defined nodes.
Does not require Ansible.`,
	Run: func(cmd *cobra.Command, args []string) {
		doctor.ValidateConfig(os.Stdout)
	},
}

func init() {
	doctorCmd.Flags().String("tags", "", "Comma-separated Ansible tags to run")
	doctorCmd.AddCommand(doctorConfigCmd)
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	// Check dependencies
	if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
		return err
	}

	tagsFlag, _ := cmd.Flags().GetString("tags")
	var tags []string
	if tagsFlag != "" {
		tags = strings.Split(tagsFlag, ",")
	}

	// Start spinner
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Collecting metrics from all nodes..."
	s.Start()

	var buf bytes.Buffer

	result, err := ansible.RunDoctor(DryRun, tags, &buf)

	s.Stop()

	if err != nil && result == nil {
		return fmt.Errorf("doctor failed: %w", err)
	}

	// Parse metrics from Ansible output
	metrics := ansible.ParseDoctorMetrics(buf.String())

	if len(metrics) == 0 {
		fmt.Println("No metrics collected. Check that nodes are reachable.")
		return nil
	}

	// Display table
	fmt.Println("\n🩺 Cluster Health Report")
	fmt.Println()

	table := tablewriter.NewTable(os.Stdout)
	table.Header([]string{"Node", "IP", "CPU °C", "Disk", "RAM", "Load", "Uptime"})

	for _, m := range metrics {
		err := table.Append([]string{
			m.Host,
			m.IP,
			m.CPUTemp + "°",
			m.Disk,
			m.Mem,
			m.Load,
			m.Uptime,
		})
		if err != nil {
			return fmt.Errorf("failed to append table row: %w", err)
		}
	}

	err = table.Render()
	if err != nil {
		return fmt.Errorf("failed to render table: %w", err)
	}

	fmt.Printf("\n✅ Diagnostics completed in %s\n", result.Duration.Round(time.Second))

	return nil
}
