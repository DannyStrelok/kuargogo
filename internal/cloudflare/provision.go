package cloudflare

import (
	"context"
	"fmt"
	"io"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// TunnelResult holds the result of a tunnel provisioning operation.
type TunnelResult struct {
	TunnelID  string
	Token     string
	AccountID string
}

// ProvisionAndSaveTunnel automates the full Cloudflare Tunnel provisioning flow:
// 1. Derives the tunnel name from the active config context (e.g. "kgg-default-tunnel").
// 2. Calls ProvisionTunnel on the Cloudflare API.
// 3. Persists the returned credentials back into kuargogo.yaml.
//
// The output writer receives progress messages. It is safe to pass io.Discard.
func ProvisionAndSaveTunnel(ctx context.Context, output io.Writer) (*TunnelResult, error) {
	cfg := config.GetConfig()

	if cfg.Cloudflare.APIToken == "" {
		return nil, fmt.Errorf("cloudflare APIToken is required for automated provisioning")
	}

	mgr, err := NewManager(cfg.Cloudflare)
	if err != nil {
		return nil, err
	}
	mgr.Output = output

	// Build a deterministic tunnel name from the active context.
	tunnelName := fmt.Sprintf("kgg-%s-tunnel", config.GetCurrentContext())
	_, _ = fmt.Fprintf(output, "🔎 Tunnel name: %s\n", tunnelName)

	tunnelID, token, accountID, err := mgr.ProvisionTunnel(ctx, tunnelName)
	if err != nil {
		return nil, fmt.Errorf("failed to provision tunnel: %w", err)
	}

	// Persist the credentials into the active config context.
	if err := config.ModifyConfig(func(c *config.ClusterConfig) {
		c.Cloudflare.AccountID = accountID
		c.Cloudflare.TunnelID = tunnelID
		if token != "" {
			c.Cloudflare.TunnelToken = config.Secret(token)
		}
	}); err != nil {
		return nil, fmt.Errorf("failed to update config with tunnel credentials: %w", err)
	}

	if err := config.SaveConfig(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	_, _ = fmt.Fprintf(output, "✅ Tunnel provisioned and credentials saved.\n")

	return &TunnelResult{
		TunnelID:  tunnelID,
		Token:     token,
		AccountID: accountID,
	}, nil
}
