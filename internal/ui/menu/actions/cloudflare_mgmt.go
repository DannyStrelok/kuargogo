package actions

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// AddCloudflareDomain adds a new domain entry to the Cloudflare config.
func AddCloudflareDomain(domain string, accessEnabled bool) tea.Cmd {
	return func() tea.Msg {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			return ResultMsg{Output: "Error: domain cannot be empty"}
		}

		err := config.ModifyConfig(func(c *config.ClusterConfig) {
			// Prevent duplicates
			for _, d := range c.Cloudflare.Domains {
				if d.Domain == domain {
					return
				}
			}
			c.Cloudflare.Domains = append(c.Cloudflare.Domains, config.CloudflareDomain{
				Domain:        domain,
				AccessEnabled: accessEnabled,
			})
		})
		if err != nil {
			return ResultMsg{Output: "Error adding domain: " + err.Error()}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: "Error persisting config: " + err.Error()}
		}
		return ResultMsg{Output: fmt.Sprintf("Domain '%s' added. Add services and Sync to apply changes.", domain)}
	}
}

// RemoveCloudflareDomain deletes a domain and all its services by index.
func RemoveCloudflareDomain(index int) tea.Cmd {
	return func() tea.Msg {
		var domainName string
		err := config.ModifyConfig(func(c *config.ClusterConfig) {
			if index < 0 || index >= len(c.Cloudflare.Domains) {
				return
			}
			domainName = c.Cloudflare.Domains[index].Domain
			c.Cloudflare.Domains = append(c.Cloudflare.Domains[:index], c.Cloudflare.Domains[index+1:]...)
		})
		if err != nil {
			return ResultMsg{Output: "Error removing domain: " + err.Error()}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: "Error persisting config: " + err.Error()}
		}
		return ResultMsg{Output: fmt.Sprintf("âœ… Domain '%s' removed.", domainName)}
	}
}

// AddCloudflareService adds a new service to the specified domain by index.
func AddCloudflareService(domainIndex int, name, subdomain, target string, protected bool) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(c *config.ClusterConfig) {
			if domainIndex < 0 || domainIndex >= len(c.Cloudflare.Domains) {
				return
			}
			c.Cloudflare.Domains[domainIndex].Services = append(
				c.Cloudflare.Domains[domainIndex].Services,
				config.CloudflareService{
					Name:      strings.TrimSpace(name),
					Subdomain: strings.TrimSpace(subdomain),
					Target:    strings.TrimSpace(target),
					Protected: protected,
				},
			)
		})
		if err != nil {
			return ResultMsg{Output: "Error adding service: " + err.Error()}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: "Error persisting config: " + err.Error()}
		}
		return ResultMsg{Output: fmt.Sprintf("âœ… Service '%s' added. Remember to Sync to apply changes.", name)}
	}
}

// UpdateCloudflareService modifies an existing service in the specified domain.
func UpdateCloudflareService(domainIndex, serviceIndex int, name, subdomain, target string, protected bool) tea.Cmd {
	return func() tea.Msg {
		err := config.ModifyConfig(func(c *config.ClusterConfig) {
			if domainIndex < 0 || domainIndex >= len(c.Cloudflare.Domains) {
				return
			}
			svcs := c.Cloudflare.Domains[domainIndex].Services
			if serviceIndex < 0 || serviceIndex >= len(svcs) {
				return
			}
			c.Cloudflare.Domains[domainIndex].Services[serviceIndex] = config.CloudflareService{
				Name:      strings.TrimSpace(name),
				Subdomain: strings.TrimSpace(subdomain),
				Target:    strings.TrimSpace(target),
				Protected: protected,
			}
		})
		if err != nil {
			return ResultMsg{Output: "Error updating service: " + err.Error()}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: "Error persisting config: " + err.Error()}
		}
		return ResultMsg{Output: fmt.Sprintf("âœ… Service '%s' updated. Remember to Sync to apply changes.", name)}
	}
}

// RemoveCloudflareService deletes a service from the specified domain.
func RemoveCloudflareService(domainIndex, serviceIndex int) tea.Cmd {
	return func() tea.Msg {
		var serviceName string
		err := config.ModifyConfig(func(c *config.ClusterConfig) {
			if domainIndex < 0 || domainIndex >= len(c.Cloudflare.Domains) {
				return
			}
			svcs := c.Cloudflare.Domains[domainIndex].Services
			if serviceIndex < 0 || serviceIndex >= len(svcs) {
				return
			}
			serviceName = svcs[serviceIndex].Name
			c.Cloudflare.Domains[domainIndex].Services = append(svcs[:serviceIndex], svcs[serviceIndex+1:]...)
		})
		if err != nil {
			return ResultMsg{Output: "Error removing service: " + err.Error()}
		}
		if err := config.SaveConfig(); err != nil {
			return ResultMsg{Output: "Error persisting config: " + err.Error()}
		}
		return ResultMsg{Output: fmt.Sprintf("âœ… Service '%s' removed. Remember to Sync to apply changes.", serviceName)}
	}
}
