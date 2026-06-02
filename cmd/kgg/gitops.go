package main

import (
	"encoding/json"
	"fmt"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/gitops"
	"github.com/spf13/cobra"
)

var gitopsCmd = &cobra.Command{
	Use:   "gitops",
	Short: "Manage declarative GitOps projects and applications",
	Long: `Manage your ArgoCD projects and applications directly from the CLI.
These changes are saved to your kuargogo.yaml and can be synchronized with 'kgg ops argocd'.`,
}

// --- Project Commands ---

var gitopsProjectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage GitOps projects",
}

var projectDesc string
var gitopsProjectAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new GitOps project",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		err := gitops.NewManager().AddProject(config.GitOpsProject{
			Name:        name,
			Description: projectDesc,
		})
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return
		}
		fmt.Printf("✅ Project '%s' added successfully.\n", name)
	},
}

var gitopsProjectRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a GitOps project and all its applications",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if err := gitops.NewManager().RemoveProject(name); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return
		}
		fmt.Printf("✅ Project '%s' and its apps removed.\n", name)
	},
}

// --- App Commands ---

var gitopsAppCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage GitOps applications within projects",
}

var appNamespace, appBranch string
var gitopsAppAddCmd = &cobra.Command{
	Use:   "add <project> <name> <repo> <path>",
	Short: "Add a new application to a project",
	Args:  cobra.ExactArgs(4),
	Run: func(cmd *cobra.Command, args []string) {
		projectName, appName, repo, path := args[0], args[1], args[2], args[3]
		app := config.GitOpsApp{
			Name:      appName,
			Repo:      repo,
			Path:      path,
			Namespace: appNamespace,
			Branch:    appBranch,
		}
		if err := gitops.NewManager().AddApp(projectName, app); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return
		}
		fmt.Printf("✅ App '%s' added to project '%s'.\n", appName, projectName)
	},
}

var gitopsAppRemoveCmd = &cobra.Command{
	Use:   "remove <project> <name>",
	Short: "Remove an application from a project",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		projectName, appName := args[0], args[1]
		if err := gitops.NewManager().RemoveApp(projectName, appName); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return
		}
		fmt.Printf("✅ App '%s' removed from project '%s'.\n", appName, projectName)
	},
}

// --- Repo/Credentials Commands ---

var gitopsRepoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage private repository credentials",
}

var repoUsername, repoEmail, repoRegistry string
var gitopsRepoAddCmd = &cobra.Command{
	Use:   "add <url> <token> [--username <user>] [--email <email>] [--registry <registry>]",
	Short: "Add a new private repository credential",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		repoURL, token := args[0], args[1]
		cred := config.GitOpsCredential{
			URL:      repoURL,
			Username: repoUsername,
			Password: config.Secret(token),
			Email:    repoEmail,
			Registry: repoRegistry,
		}
		if err := gitops.NewManager().AddCredential(cred); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return
		}
		fmt.Printf("✅ Credential for '%s' added successfully.\n", repoURL)
	},
}

var gitopsRepoRemoveCmd = &cobra.Command{
	Use:   "remove <url>",
	Short: "Remove a private repository credential",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repoURL := args[0]
		if err := gitops.NewManager().RemoveCredential(repoURL); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return
		}
		fmt.Printf("✅ Credential for '%s' removed.\n", repoURL)
	},
}

// --- List Command ---

var gitopsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all GitOps projects and applications",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.GetConfig()
		isJSON, _ := cmd.Flags().GetBool("json")

		if isJSON {
			data, _ := json.MarshalIndent(cfg.GitOps, "", "  ")
			fmt.Println(string(data))
			return
		}

		if len(cfg.GitOps.Projects) == 0 && len(cfg.GitOps.Credentials) == 0 {
			fmt.Println("No GitOps projects or credentials configured.")
			return
		}

		if len(cfg.GitOps.Credentials) > 0 {
			fmt.Println("🔑 Private Repositories:")
			for _, c := range cfg.GitOps.Credentials {
				user := c.Username
				if user == "" {
					user = "<token-only>"
				}
				regInfo := ""
				if c.Registry != "" {
					regInfo = fmt.Sprintf(" [reg: %s]", c.Registry)
				}
				fmt.Printf("   └── 🌐 %s (user: %s)%s\n", c.URL, user, regInfo)
			}
			fmt.Println()
		}

		if len(cfg.GitOps.Projects) > 0 {
			fmt.Println("⛵ GitOps Inventory:")
			for _, p := range cfg.GitOps.Projects {
				fmt.Printf("📂 Project: %s (%s)\n", p.Name, p.Description)
				if len(p.Apps) == 0 {
					fmt.Println("   (no apps)")
					continue
				}
				for _, a := range p.Apps {
					ns := a.Namespace
					if ns == "" {
						ns = "default"
					}
					fmt.Printf("   └── 📦 %-15s [%s] -> %s (ns:%s)\n", a.Name, a.Repo, a.Path, ns)
				}
			}
		} // closing brace for if len(...) > 0
	},
}

func init() {
	rootCmd.AddCommand(gitopsCmd)

	// Project commands
	gitopsCmd.AddCommand(gitopsProjectCmd)
	gitopsProjectAddCmd.Flags().StringVarP(&projectDesc, "desc", "d", "", "Project description")
	gitopsProjectCmd.AddCommand(gitopsProjectAddCmd)
	gitopsProjectCmd.AddCommand(gitopsProjectRemoveCmd)

	// App commands
	gitopsCmd.AddCommand(gitopsAppCmd)
	gitopsAppAddCmd.Flags().StringVarP(&appNamespace, "namespace", "n", "", "Target namespace")
	gitopsAppAddCmd.Flags().StringVarP(&appBranch, "branch", "b", "main", "Git branch/revision")
	gitopsAppCmd.AddCommand(gitopsAppAddCmd)
	gitopsAppCmd.AddCommand(gitopsAppRemoveCmd)

	// Repo commands
	gitopsCmd.AddCommand(gitopsRepoCmd)
	gitopsRepoAddCmd.Flags().StringVarP(&repoUsername, "username", "u", "git", "Username for basic auth (often 'git' for tokens)")
	gitopsRepoAddCmd.Flags().StringVarP(&repoEmail, "email", "e", "", "Email for registry authentication")
	gitopsRepoAddCmd.Flags().StringVarP(&repoRegistry, "registry", "r", "", "Registry hostname (e.g. ghcr.io) for pull secret automation")
	gitopsRepoCmd.AddCommand(gitopsRepoAddCmd)
	gitopsRepoCmd.AddCommand(gitopsRepoRemoveCmd)

	// Sync command
	gitopsCmd.AddCommand(gitopsSyncCmd)

	// List command
	gitopsListCmd.Flags().Bool("json", false, "Output in JSON format")
	gitopsCmd.AddCommand(gitopsListCmd)

	// Pull secrets sync command
	gitopsCmd.AddCommand(gitopsSyncPullSecretsCmd)
}

var gitopsSyncPullSecretsCmd = &cobra.Command{
	Use:   "sync-pull-secrets",
	Short: "Create/update imagePullSecrets in all GitOps namespaces",
	Long: `Reads credentials with a 'registry' field from kuargogo.yaml and creates
(or updates) a Kubernetes docker-registry Secret in every namespace used by
your GitOps applications.

The secret name follows the convention: <registry-slug>-pull-secret
  e.g.  ghcr.io  →  ghcr-io-pull-secret

This command is idempotent — safe to run multiple times or after rotating a PAT.

Examples:
  kgg gitops sync-pull-secrets             # apply to cluster
  kgg gitops sync-pull-secrets --dry-run   # preview without touching the cluster`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.GetConfig()

		svc := gitops.NewPullSecretsService()
		svc.Output = cmd.OutOrStdout()
		svc.DryRun = config.IsDryRun()

		if err := svc.Sync(cfg); err != nil {
			return fmt.Errorf("sync-pull-secrets: %w", err)
		}
		return nil
	},
}

var gitopsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Reconcile ArgoCD state (projects, apps, and repos) with local config",
	Long: `Applies the GitOps state from kuargogo.yaml to the live ArgoCD instance
in the cluster. This will create or update:
- AppProjects
- Applications
- Repository Credentials (as Secrets)

Uses 'kubectl apply' internally for idempotency.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.GetConfig()

		orc := gitops.NewOrchestrator()
		orc.Output = cmd.OutOrStdout()
		orc.DryRun = config.IsDryRun()

		if err := orc.Sync(cfg); err != nil {
			return fmt.Errorf("gitops sync: %w", err)
		}
		return nil
	},
}
