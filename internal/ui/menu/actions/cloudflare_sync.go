package actions

import (
	"context"
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/cloudflare"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/deps"
)

// SyncCloudflare synchronizes ALL domains to Cloudflare (Tunnel + Zero Trust).
func SyncCloudflare() tea.Cmd {
	return func() tea.Msg {
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			if err := RunCloudflareSync(writer); err != nil {
				ch <- fmt.Sprintf("\n❌ Sync failed: %v", err)
			} else {
				ch <- "\n✨ Cloudflare synchronization completed successfully!"
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// SyncCloudflareDomain synchronizes a single domain by its index in cfg.Cloudflare.Domains.
func SyncCloudflareDomain(domainIndex int) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			cfg := config.GetConfig()
			if domainIndex < 0 || domainIndex >= len(cfg.Cloudflare.Domains) {
				ch <- "❌ Error: domain index out of range"
				return
			}

			d := cfg.Cloudflare.Domains[domainIndex]
			_, _ = fmt.Fprintf(writer, "🌐 Synchronizing domain: %s\n", d.Domain)

			if err := runCloudflareValidation(cfg); err != nil {
				ch <- fmt.Sprintf("❌ %v", err)
				return
			}

			mgr, err := cloudflare.NewManager(cfg.Cloudflare)
			if err != nil {
				ch <- fmt.Sprintf("❌ Error: %v", err)
				return
			}
			mgr.Output = writer

			if err := syncDomain(context.Background(), mgr, cfg, domainIndex, writer); err != nil {
				ch <- fmt.Sprintf("\n❌ Sync failed: %v", err)
			} else {
				ch <- fmt.Sprintf("\n✨ Domain %s synchronized successfully!", d.Domain)
			}
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}

// RunCloudflareSync is the internal orchestration logic for synchronizing ALL domains.
func RunCloudflareSync(writer io.Writer) error {
	cfg := config.GetConfig()

	if err := runCloudflareValidation(cfg); err != nil {
		return err
	}

	if err := deps.CheckAll("ansible", "ansible-playbook"); err != nil {
		return fmt.Errorf("ansible is required for certificate management: %w", err)
	}

	_, _ = fmt.Fprintf(writer, "🌐 Starting Cloudflare Multi-Domain Synchronization...\n")
	_, _ = fmt.Fprintf(writer, "   Tunnel: %s\n", cfg.Cloudflare.TunnelID)
	_, _ = fmt.Fprintf(writer, "   Domains: %d\n\n", len(cfg.Cloudflare.Domains))

	// 1. Ensure Wildcard Certificates for all unique domains
	ensuredDomains := make(map[string]bool)
	for _, d := range cfg.Cloudflare.Domains {
		if ensuredDomains[d.Domain] {
			continue
		}
		_, _ = fmt.Fprintf(writer, "📜 Ensuring Wildcard Certificate for %s...\n", d.Domain)
		certResult, err := ansible.EnsureWildcardCertificate(config.IsDryRun(), d.Domain, writer)
		if err != nil || !certResult.Success {
			_, _ = fmt.Fprintf(writer, "⚠️  Warning: Failed to ensure wildcard certificate for %s: %v\n", d.Domain, err)
		}
		ensuredDomains[d.Domain] = true
	}

	// 2. Initialize Cloudflare Manager (shared across all domains - same account/tunnel)
	mgr, err := cloudflare.NewManager(cfg.Cloudflare)
	if err != nil {
		return fmt.Errorf("failed to initialize Cloudflare manager: %w", err)
	}
	mgr.Output = writer

	// 3. Sync each domain
	for i := range cfg.Cloudflare.Domains {
		if err := syncDomain(context.Background(), mgr, cfg, i, writer); err != nil {
			_, _ = fmt.Fprintf(writer, "⚠️  Domain %s sync failed: %v\n", cfg.Cloudflare.Domains[i].Domain, err)
		}
	}

	return nil
}

// syncDomain handles the full synchronization of a single CloudflareDomain.
func syncDomain(ctx context.Context, mgr *cloudflare.Manager, cfg config.ClusterConfig, domainIndex int, writer io.Writer) error {
	d := cfg.Cloudflare.Domains[domainIndex]
	_, _ = fmt.Fprintf(writer, "\n━━━ Domain: %s ━━━\n", d.Domain)

	if len(d.Services) == 0 {
		_, _ = fmt.Fprintf(writer, "ℹ️  No services defined for this domain.\n")
		return nil
	}

	for _, svc := range d.Services {
		hostname := d.Domain
		if svc.Subdomain != "" && svc.Subdomain != "@" {
			hostname = fmt.Sprintf("%s.%s", svc.Subdomain, d.Domain)
		}

		_, _ = fmt.Fprintf(writer, "\n📦 [%s] → %s\n", svc.Name, hostname)

		// A. Configure Tunnel Ingress
		if err := mgr.UpsertPublicHostname(ctx, cfg.Cloudflare.TunnelID, svc.Subdomain, d.Domain, svc.Target); err != nil {
			_, _ = fmt.Fprintf(writer, "❌ Failed to expose %s: %v\n", svc.Name, err)
			continue
		}
		_, _ = fmt.Fprintf(writer, "✅ Tunnel Ingress: %s → %s\n", hostname, svc.Target)

		// B. Configure Zero Trust Access (if domain has Access enabled and service is protected)
		if d.AccessEnabled && svc.Protected {
			_, _ = fmt.Fprintf(writer, "🛡️  Securing with Cloudflare Zero Trust Access...\n")

			appID, err := mgr.UpsertAccessApplication(ctx, cfg.Cloudflare.AccountID, svc.Name, hostname)
			if err != nil {
				_, _ = fmt.Fprintf(writer, "⚠️  Failed to create Access Application for %s: %v\n", svc.Name, err)
				continue
			}

			emails := cfg.Cloudflare.AccessEmails
			if len(emails) == 0 && cfg.Cloudflare.Email != "" {
				emails = []string{cfg.Cloudflare.Email}
			}

			if err := mgr.UpsertAccessPolicy(ctx, cfg.Cloudflare.AccountID, appID, "Authorized Users", emails); err != nil {
				_, _ = fmt.Fprintf(writer, "⚠️  Failed to create Access Policy for %s: %v\n", svc.Name, err)
			} else {
				_, _ = fmt.Fprintf(writer, "✅ Zero Trust PROTECTED: %s\n", hostname)
			}
		} else {
			_, _ = fmt.Fprintf(writer, "🔓 Service is PUBLIC (No Zero Trust Access).\n")
		}
	}

	return nil
}

// runCloudflareValidation checks that required credentials are present.
func runCloudflareValidation(cfg config.ClusterConfig) error {
	if cfg.Cloudflare.APIToken == "" || cfg.Cloudflare.TunnelID == "" {
		return fmt.Errorf("cloudflare is not fully configured: api_token and tunnel_id are required")
	}
	if len(cfg.Cloudflare.Domains) == 0 {
		return fmt.Errorf("no domains configured — add at least one domain under 'Dominios & Servicios'")
	}
	return nil
}
