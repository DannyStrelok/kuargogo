package actions

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/gitops"
)

// AddGitOpsProject adds a new GitOps project to the configuration
func AddGitOpsProject(project config.GitOpsProject) tea.Cmd {
	return func() tea.Msg {
		if err := gitops.NewManager().AddProject(project); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error adding project: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Project '%s' added successfully!", project.Name)}
	}
}

// UpdateGitOpsProject modifies an existing GitOps project (retaining its apps)
func UpdateGitOpsProject(index int, project config.GitOpsProject) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		if index < 0 || index >= len(cfg.GitOps.Projects) {
			return ResultMsg{Output: "❌ Error: Invalid project index"}
		}
		oldName := cfg.GitOps.Projects[index].Name
		if err := gitops.NewManager().UpdateProject(oldName, project); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating project: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Project '%s' updated successfully!", project.Name)}
	}
}

// RemoveGitOpsProject deletes a project from the configuration
func RemoveGitOpsProject(index int) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		if index < 0 || index >= len(cfg.GitOps.Projects) {
			return ResultMsg{Output: "❌ Error: Invalid project index"}
		}
		name := cfg.GitOps.Projects[index].Name
		if err := gitops.NewManager().RemoveProject(name); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error removing project: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Project '%s' removed successfully!", name)}
	}
}

// AddGitOpsApp adds a new app to a specific GitOps project
func AddGitOpsApp(projectIndex int, app config.GitOpsApp) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		if projectIndex < 0 || projectIndex >= len(cfg.GitOps.Projects) {
			return ResultMsg{Output: "❌ Error: Invalid project index"}
		}
		projectName := cfg.GitOps.Projects[projectIndex].Name
		if err := gitops.NewManager().AddApp(projectName, app); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error adding app: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ App '%s' added successfully!", app.Name)}
	}
}

// UpdateGitOpsApp modifies an existing app within a project
func UpdateGitOpsApp(projectIndex int, appIndex int, app config.GitOpsApp) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		if projectIndex < 0 || projectIndex >= len(cfg.GitOps.Projects) {
			return ResultMsg{Output: "❌ Error: Invalid project index"}
		}
		projectName := cfg.GitOps.Projects[projectIndex].Name
		if appIndex < 0 || appIndex >= len(cfg.GitOps.Projects[projectIndex].Apps) {
			return ResultMsg{Output: "❌ Error: Invalid app index"}
		}
		oldAppName := cfg.GitOps.Projects[projectIndex].Apps[appIndex].Name
		if err := gitops.NewManager().UpdateApp(projectName, oldAppName, app); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error updating app: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ App '%s' updated successfully!", app.Name)}
	}
}

// RemoveGitOpsApp deletes an app from a project
func RemoveGitOpsApp(projectIndex int, appIndex int) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		if projectIndex < 0 || projectIndex >= len(cfg.GitOps.Projects) {
			return ResultMsg{Output: "❌ Error: Invalid project index"}
		}
		projectName := cfg.GitOps.Projects[projectIndex].Name
		if appIndex < 0 || appIndex >= len(cfg.GitOps.Projects[projectIndex].Apps) {
			return ResultMsg{Output: "❌ Error: Invalid app index"}
		}
		appName := cfg.GitOps.Projects[projectIndex].Apps[appIndex].Name
		if err := gitops.NewManager().RemoveApp(projectName, appName); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error removing app: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ App '%s' removed successfully!", appName)}
	}
}

// AddGitOpsCredential adds a new private repository credential
func AddGitOpsCredential(cred config.GitOpsCredential) tea.Cmd {
	return func() tea.Msg {
		if err := gitops.NewManager().AddCredential(cred); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error adding credential: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Credential for '%s' saved successfully!", cred.URL)}
	}
}

// RemoveGitOpsCredential removes a private repository credential
func RemoveGitOpsCredential(index int) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		if index < 0 || index >= len(cfg.GitOps.Credentials) {
			return ResultMsg{Output: "❌ Error: Invalid credential index"}
		}
		url := cfg.GitOps.Credentials[index].URL
		if err := gitops.NewManager().RemoveCredential(url); err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error removing credential: %v", err)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Credential for '%s' removed successfully!", url)}
	}
}

// SyncPullSecrets creates or updates Kubernetes imagePullSecrets in all GitOps
// namespaces for every credential that has a 'registry' field configured.
func SyncPullSecrets() tea.Cmd {
	return func() tea.Msg {
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)
			cfg := config.GetConfig()

			svc := gitops.NewPullSecretsService()
			svc.Output = writer
			svc.DryRun = config.IsDryRun()

			if err := svc.Sync(cfg); err != nil {
				ch <- fmt.Sprintf("\n❌ Pull secret sync failed: %v", err)
			} else {
				ch <- "\n✅ Pull secrets synchronized successfully!"
			}
		}()
		return ActionStartedMsg{ProgressChan: ch}
	}
}

// SyncGitOpsState reconciles the local GitOps configuration with the live ArgoCD instance.
func SyncGitOpsState() tea.Cmd {
	return func() tea.Msg {
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)
			cfg := config.GetConfig()

			orc := gitops.NewOrchestrator()
			orc.Output = writer
			orc.DryRun = config.IsDryRun()

			if err := orc.Sync(cfg); err != nil {
				ch <- fmt.Sprintf("\n❌ GitOps sync failed: %v", err)
			} else {
				ch <- "\n✅ GitOps state synchronized successfully!"
			}
		}()
		return ActionStartedMsg{ProgressChan: ch}
	}
}
