package help

import (
	"embed"
	"fmt"
	"regexp"
)

//go:embed docs/*.md
var contentFS embed.FS

// Topic represents a single help topic or guide.
type Topic struct {
	ID          string
	Title       string
	Description string
	Filename    string // Filename in the embed FS
}

// Service provides access to help topics and content.
type Service struct {
	topics []Topic
}

// NewService creates a new help service with predefined topics.
func NewService() *Service {
	return &Service{
		topics: []Topic{
			{
				ID:          "00-workflow-roadmap",
				Title:       "🚀 Workflow Roadmap",
				Description: "Master strategy for building and maintaining your homelab",
				Filename:    "docs/00-workflow-roadmap.md",
			},
			{
				ID:          "debian-install",
				Title:       "Debian 13 Installation Guide",
				Description: "Guide for installing Debian 13 on Lenovo m920q and HP 800 G4 Mini",
				Filename:    "docs/debian_install.md",
			},
			{
				ID:          "01-hardware-prep",
				Title:       "Hardware Preparation",
				Description: "Guide for preparing hardware for Debian 13 installation",
				Filename:    "docs/01-hardware-preparation.md",
			},
			{
				ID:          "02-provisioning",
				Title:       "Provisioning",
				Description: "Guide for provisioning nodes",
				Filename:    "docs/02-provisioning.md",
			},
			{
				ID:          "03-cluster",
				Title:       "Cluster and Services",
				Description: "Guide for cluster and services",
				Filename:    "docs/03-cluster-and-services.md",
			},
			{
				ID:          "04-observability",
				Title:       "Observability",
				Description: "Guide for observability configuration",
				Filename:    "docs/04-observability.md",
			},
			{
				ID:          "05-telegram",
				Title:       "🤖 Telegram Bot Setup",
				Description: "Guide for configuring the homelab monitoring and control bot",
				Filename:    "docs/05-telegram-setup.md",
			},
			{
				ID:          "06-gitops",
				Title:       "⛵ GitOps and Secrets",
				Description: "Declarative deployment workflow",
				Filename:    "docs/06-gitops-and-secrets.md",
			},
			{
				ID:          "07-cloudflare",
				Title:       "☁️ Cloudflare Zero Trust",
				Description: "Exposing services securely via Tunnels",
				Filename:    "docs/07-cloudflare-zero-trust.md",
			},
			{
				ID:          "08-automation",
				Title:       "🤖 Automation CI/CD",
				Description: "GitHub Actions integration for your apps",
				Filename:    "docs/08-automation-cicd.md",
			},
			{
				ID:          "09-cloud-sync",
				Title:       "☁️ Cloud Sync & Backup",
				Description: "Safe off-site E2EE configuration storage",
				Filename:    "docs/09-cloud-sync-and-backup.md",
			},
			{
				ID:          "10-new-project-guide",
				Title:       "🚀 New Project Setup Guide",
				Description: "Step-by-step workflow to deploy new microservices with Kargo and ArgoCD",
				Filename:    "docs/10-new-project-guide.md",
			},
			{
				ID:          "11-k9s-cluster-management",
				Title:       "☸️ K9s Cluster Management Guide",
				Description: "Master the command-line TUI to navigate, debug and manage your k3s cluster",
				Filename:    "docs/11-k9s-cluster-management.md",
			},
		},
	}
}

// GetTopics returns all available help topics.
func (s *Service) GetTopics() []Topic {
	return s.topics
}

var (
	importRegex           = regexp.MustCompile(`(?m)^import\s+.*$`)
	terminalShowcaseRegex = regexp.MustCompile(`(?s)<TerminalShowcase\b.*?\/>`)
)

func cleanMDX(content string) string {
	content = importRegex.ReplaceAllString(content, "")
	content = terminalShowcaseRegex.ReplaceAllString(content, "")
	return content
}

// GetContent returns the content of a topic by its ID.
func (s *Service) GetContent(id string) (string, error) {
	var filename string
	for _, t := range s.topics {
		if t.ID == id {
			filename = t.Filename
			break
		}
	}

	if filename == "" {
		return "", fmt.Errorf("topic not found: %s", id)
	}

	content, err := contentFS.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("failed to read content for %s: %w", id, err)
	}

	return cleanMDX(string(content)), nil
}
