package cloudflare

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/cloudflare/cloudflare-go"
)

// Manager handles interaction with the Cloudflare API for Zero Trust management.
type Manager struct {
	client *cloudflare.API
	cfg    config.Cloudflare
	Output io.Writer
}

// NewManager creates a new Cloudflare manager using the provided configuration.
func NewManager(cfg config.Cloudflare) (*Manager, error) {
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("cloudflare API Token is required for automation")
	}

	api, err := cloudflare.NewWithAPIToken(string(cfg.APIToken))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Cloudflare client: %w", err)
	}

	return &Manager{
		client: api,
		cfg:    cfg,
		Output: os.Stdout,
	}, nil
}

// GetAccountID retrieves the Account ID associated with the API token.
func (m *Manager) GetAccountID(ctx context.Context) (string, error) {
	if m.cfg.AccountID != "" {
		return m.cfg.AccountID, nil
	}

	// List accounts and pick the first one (homelab context)
	accounts, _, err := m.client.Accounts(ctx, cloudflare.AccountsListParams{})
	if err != nil {
		return "", fmt.Errorf("failed to list Cloudflare accounts: %w", err)
	}

	if len(accounts) == 0 {
		return "", fmt.Errorf("no Cloudflare accounts found for this token")
	}

	// Prefer the account that matches the email if possible, otherwise first
	for _, acc := range accounts {
		if strings.Contains(strings.ToLower(acc.Name), strings.ToLower(m.cfg.Email)) {
			return acc.ID, nil
		}
	}

	return accounts[0].ID, nil
}

// GetZoneID retrieves the Zone ID for a given domain.
func (m *Manager) GetZoneID(ctx context.Context, domainName string) (string, error) {
	if domainName == "" {
		return "", fmt.Errorf("domain name is required to look up Zone ID")
	}

	zoneID, err := m.client.ZoneIDByName(domainName)
	if err != nil {
		return "", fmt.Errorf("failed to find Zone ID for domain %s: %w", domainName, err)
	}

	return zoneID, nil
}

// ProvisionTunnel creates a Cloudflare Tunnel if it doesn't exist.
// It returns the TunnelID, the base64-encoded TunnelToken, and the AccountID.
func (m *Manager) ProvisionTunnel(ctx context.Context, name string) (string, string, string, error) {
	accountID, err := m.GetAccountID(ctx)
	if err != nil {
		return "", "", "", err
	}

	rc := cloudflare.AccountIdentifier(accountID)

	// 1. Check if tunnel already exists
	tunnels, _, err := m.client.ListTunnels(ctx, rc, cloudflare.TunnelListParams{Name: name})
	if err == nil && len(tunnels) > 0 {
		t := tunnels[0]
		_, _ = fmt.Fprintf(m.Output, "✅ Existing tunnel '%s' found (ID: %s)\n", name, t.ID)
		return t.ID, "", accountID, nil
	}

	// 2. Create new tunnel
	_, _ = fmt.Fprintf(m.Output, "🚀 Creating new Cloudflare Tunnel: %s...\n", name)

	// Generate a cryptographically secure random 32-byte secret (Base64).
	// NOTE: GenerateRandomString(32) already uses crypto/rand internally.
	secretRaw, err := config.GenerateRandomString(32)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate tunnel secret: %w", err)
	}
	secret := base64.StdEncoding.EncodeToString([]byte(secretRaw))

	tunnel, err := m.client.CreateTunnel(ctx, rc, cloudflare.TunnelCreateParams{
		Name:      name,
		Secret:    secret,
		ConfigSrc: "cloudflare", // We use Cloudflare Managed config
	})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create tunnel: %w", err)
	}

	// 3. Generate the Tunnel Token (base64 of the credentials JSON)
	type credentials struct {
		AccountID string `json:"a"`
		TunnelID  string `json:"t"`
		Secret    string `json:"s"`
	}
	creds := credentials{
		AccountID: accountID,
		TunnelID:  tunnel.ID,
		Secret:    secret,
	}
	credsJSON, _ := json.Marshal(creds)
	token := base64.StdEncoding.EncodeToString(credsJSON)

	_, _ = fmt.Fprintf(m.Output, "✅ Tunnel created successfully.\n")
	return tunnel.ID, token, accountID, nil
}

// UpsertPublicHostname configures a public hostname for the tunnel.
// If domain is empty, it uses the default one from config.
func (m *Manager) UpsertPublicHostname(ctx context.Context, tunnelID, subdomain, domain, serviceURL string) error {
	accountID, err := m.GetAccountID(ctx)
	if err != nil {
		return err
	}

	targetDomain := domain

	zoneID, err := m.GetZoneID(ctx, targetDomain)
	if err != nil {
		return err
	}

	rc := cloudflare.AccountIdentifier(accountID)
	hostname := targetDomain
	if subdomain != "" && subdomain != "@" {
		hostname = fmt.Sprintf("%s.%s", subdomain, targetDomain)
	}
	_, _ = fmt.Fprintf(m.Output, "🌐 Configuring Public Hostname: %s → %s\n", hostname, serviceURL)

	// Fetch current configuration
	configResult, err := m.client.GetTunnelConfiguration(ctx, rc, tunnelID)
	if err != nil {
		// If no config exists, start with a fresh one
		configResult = cloudflare.TunnelConfigurationResult{}
	}

	// Update Ingress rules
	var validRules []cloudflare.UnvalidatedIngressRule
	var catchAllRule *cloudflare.UnvalidatedIngressRule
	exists := false

	for _, rule := range configResult.Config.Ingress {
		// Isolate the catch-all rule so it doesn't get stuck in the middle
		if rule.Hostname == "" || rule.Service == "http_status:404" {
			catchAllRule = &rule
			continue
		}
		if rule.Hostname == hostname {
			rule.Service = serviceURL
			// Enable NoTLSVerify for internal cluster HTTPS targets
			if strings.HasPrefix(serviceURL, "https://") && strings.Contains(serviceURL, ".svc.cluster.local") {
				rule.OriginRequest = &cloudflare.OriginRequestConfig{
					NoTLSVerify: cloudflare.BoolPtr(true),
				}
			}
			exists = true
		}
		validRules = append(validRules, rule)
	}

	if !exists {
		rule := cloudflare.UnvalidatedIngressRule{
			Hostname: hostname,
			Service:  serviceURL,
		}
		// Enable NoTLSVerify for internal cluster HTTPS targets
		if strings.HasPrefix(serviceURL, "https://") && strings.Contains(serviceURL, ".svc.cluster.local") {
			rule.OriginRequest = &cloudflare.OriginRequestConfig{
				NoTLSVerify: cloudflare.BoolPtr(true),
			}
		}
		validRules = append(validRules, rule)
	}

	// Ensure catch-all is appended at the very end
	if catchAllRule == nil {
		validRules = append(validRules, cloudflare.UnvalidatedIngressRule{
			Service: "http_status:404",
		})
	} else {
		validRules = append(validRules, *catchAllRule)
	}

	_, err = m.client.UpdateTunnelConfiguration(ctx, rc, cloudflare.TunnelConfigurationParams{
		TunnelID: tunnelID,
		Config: cloudflare.TunnelConfiguration{
			Ingress: validRules,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to update tunnel configuration: %w", err)
	}

	_, _ = fmt.Fprintf(m.Output, "📡 Ensuring DNS record (CNAME %s -> %s.cfargotunnel.com)...\n", subdomain, tunnelID)
	target := fmt.Sprintf("%s.cfargotunnel.com", tunnelID)

	records, _, err := m.client.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{Name: hostname})
	if err == nil && len(records) > 0 {
		if records[0].Content == target {
			_, _ = fmt.Fprintf(m.Output, "✅ DNS record already exists and is correct.\n")
			return nil
		}
		// Update existing
		_, err = m.client.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.UpdateDNSRecordParams{
			ID:      records[0].ID,
			Type:    "CNAME",
			Name:    hostname,
			Content: target,
			Proxied: cloudflare.BoolPtr(true),
		})
	} else {
		// Create new
		_, err = m.client.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.CreateDNSRecordParams{
			Type:    "CNAME",
			Name:    hostname,
			Content: target,
			Proxied: cloudflare.BoolPtr(true),
		})
	}

	if err != nil {
		return fmt.Errorf("failed to create DNS record: %w", err)
	}

	_, _ = fmt.Fprintf(m.Output, "✅ DNS record synchronization complete.\n")
	return nil
}

// ListZones returns a list of all active domains (zones) in the Cloudflare account.
func (m *Manager) ListZones(ctx context.Context) ([]string, error) {
	zones, err := m.client.ListZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list zones: %w", err)
	}

	var names []string
	for _, z := range zones {
		// Only include active zones
		if z.Status == "active" {
			names = append(names, z.Name)
		}
	}
	return names, nil
}

// UpsertAccessApplication creates or updates an Access Application for a specific domain.
func (m *Manager) UpsertAccessApplication(ctx context.Context, accountID, name, domain string) (string, error) {
	rc := cloudflare.AccountIdentifier(accountID)

	// 1. Check if application already exists
	apps, _, err := m.client.ListAccessApplications(ctx, rc, cloudflare.ListAccessApplicationsParams{})
	if err == nil {
		for _, app := range apps {
			if app.Domain == domain {
				_, _ = fmt.Fprintf(m.Output, "✅ Existing Access Application found for %s (ID: %s)\n", domain, app.ID)
				return app.ID, nil
			}
		}
	}

	// 2. Create new application
	_, _ = fmt.Fprintf(m.Output, "🚀 Creating Cloudflare Access Application for %s...\n", domain)
	app, err := m.client.CreateAccessApplication(ctx, rc, cloudflare.CreateAccessApplicationParams{
		Name:            name,
		Domain:          domain,
		SessionDuration: "24h",
		Type:            "self_hosted",
	})
	if err != nil {
		if strings.Contains(err.Error(), "10000") {
			return "", fmt.Errorf("authentication error (10000): Your API Token likely lacks Zero Trust permissions. Please ensure your token has 'Account -> Access: Apps and Policies -> Edit' permissions")
		}
		return "", fmt.Errorf("failed to create access application: %w", err)
	}

	return app.ID, nil
}

// UpsertAccessPolicy ensures that only specific emails have access to an application.
func (m *Manager) UpsertAccessPolicy(ctx context.Context, accountID, appID, name string, emails []string) error {
	rc := cloudflare.AccountIdentifier(accountID)

	includeRules := []interface{}{}
	for _, email := range emails {
		includeRules = append(includeRules, map[string]interface{}{
			"email": map[string]interface{}{
				"email": email,
			},
		})
	}

	// 1. Check if policy already exists
	policies, _, err := m.client.ListAccessPolicies(ctx, rc, cloudflare.ListAccessPoliciesParams{ApplicationID: appID})
	if err == nil {
		for _, p := range policies {
			if p.Name == name {
				_, _ = fmt.Fprintf(m.Output, "✅ Access Policy '%s' already exists. Updating...\n", name)

				// Update existing
				_, err = m.client.UpdateAccessPolicy(ctx, rc, cloudflare.UpdateAccessPolicyParams{
					ApplicationID: appID,
					PolicyID:      p.ID,
					Name:          name,
					Decision:      "allow",
					Include:       includeRules,
				})
				return err
			}
		}
	}

	// 2. Create new policy
	_, _ = fmt.Fprintf(m.Output, "🔒 Creating Access Policy for authorized users...\n")

	_, err = m.client.CreateAccessPolicy(ctx, rc, cloudflare.CreateAccessPolicyParams{
		ApplicationID: appID,
		Name:          name,
		Decision:      "allow",
		Include:       includeRules,
	})

	if err != nil {
		if strings.Contains(err.Error(), "10000") {
			return fmt.Errorf("authentication error (10000): Your API Token likely lacks Zero Trust permissions. Please ensure your token has 'Account -> Access: Apps and Policies -> Edit' permissions")
		}
		return fmt.Errorf("failed to create access policy: %w", err)
	}

	return nil
}
