package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/DannyStrelok/kuargogo/internal/ui/menu/actions"
)

var cloudflareCmd = &cobra.Command{
	Use:   "cloudflare",
	Short: "Manage Cloudflare Zero Trust and Tunnels",
}

var cloudflareSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize all defined services to Cloudflare (Tunnel + Zero Trust)",
	Long: `Reads the 'services' list from your kuargogo.yaml and ensures:
- A Wildcard Certificate exists in the cluster
- A public hostname is configured in the Tunnel for each service
- A Zero Trust Access Application and Policy is created for 'protected' services`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return actions.RunCloudflareSync(os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(cloudflareCmd)
	cloudflareCmd.AddCommand(cloudflareSyncCmd)
}
