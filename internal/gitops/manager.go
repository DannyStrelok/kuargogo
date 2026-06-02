package gitops

import (
	"fmt"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// Manager handles strictly business logic for GitOps configuration mutations,
// ensuring separation of concerns between CLI / TUI interfaces and internal state.
type Manager struct{}

// NewManager creates a new GitOps Manager
func NewManager() *Manager {
	return &Manager{}
}

// --- Project Operations ---

// AddProject adds a new GitOps project. Returns an error if the name already exists.
func (m *Manager) AddProject(project config.GitOpsProject) error {
	var validationErr error
	err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
		for _, p := range cfg.GitOps.Projects {
			if p.Name == project.Name {
				validationErr = fmt.Errorf("project '%s' already exists", project.Name)
				return
			}
		}
		cfg.GitOps.Projects = append(cfg.GitOps.Projects, project)
	})
	if err != nil {
		return fmt.Errorf("failed to modify config: %w", err)
	}
	if validationErr != nil {
		return validationErr
	}
	return config.SaveConfig()
}

// UpdateProject updates metadata for an existing GitOps project without deleting its apps.
func (m *Manager) UpdateProject(oldName string, project config.GitOpsProject) error {
	var validationErr error
	err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
		for i, p := range cfg.GitOps.Projects {
			if p.Name == oldName {
				// Preserve existing apps
				project.Apps = p.Apps
				cfg.GitOps.Projects[i] = project
				return
			}
		}
		validationErr = fmt.Errorf("project '%s' not found", oldName)
	})
	if err != nil {
		return fmt.Errorf("failed to modify config: %w", err)
	}
	if validationErr != nil {
		return validationErr
	}
	return config.SaveConfig()
}

// RemoveProject deletes a GitOps project and all its nested apps.
func (m *Manager) RemoveProject(name string) error {
	var found bool
	err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
		for i, p := range cfg.GitOps.Projects {
			if p.Name == name {
				copy(cfg.GitOps.Projects[i:], cfg.GitOps.Projects[i+1:])
				cfg.GitOps.Projects[len(cfg.GitOps.Projects)-1] = config.GitOpsProject{}
				cfg.GitOps.Projects = cfg.GitOps.Projects[:len(cfg.GitOps.Projects)-1]
				found = true
				break
			}
		}
	})
	if err != nil {
		return fmt.Errorf("failed to modify config: %w", err)
	}
	if !found {
		return fmt.Errorf("project '%s' not found", name)
	}
	return config.SaveConfig()
}

// --- App Operations ---

// AddApp adds a new application to an existing project.
func (m *Manager) AddApp(projectName string, app config.GitOpsApp) error {
	var validationErr error
	err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
		for i, p := range cfg.GitOps.Projects {
			if p.Name == projectName {
				for _, a := range p.Apps {
					if a.Name == app.Name {
						validationErr = fmt.Errorf("app '%s' already exists in project '%s'", app.Name, projectName)
						return
					}
				}
				cfg.GitOps.Projects[i].Apps = append(cfg.GitOps.Projects[i].Apps, app)
				return
			}
		}
		validationErr = fmt.Errorf("project '%s' not found", projectName)
	})
	if err != nil {
		return fmt.Errorf("failed to modify config: %w", err)
	}
	if validationErr != nil {
		return validationErr
	}
	return config.SaveConfig()
}

// UpdateApp modifies an existing application inside a project.
func (m *Manager) UpdateApp(projectName string, oldAppName string, app config.GitOpsApp) error {
	var validationErr error
	err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
		for i, p := range cfg.GitOps.Projects {
			if p.Name == projectName {
				for j, a := range p.Apps {
					if a.Name == oldAppName {
						cfg.GitOps.Projects[i].Apps[j] = app
						return
					}
				}
				validationErr = fmt.Errorf("app '%s' not found in project '%s'", oldAppName, projectName)
				return
			}
		}
		validationErr = fmt.Errorf("project '%s' not found", projectName)
	})
	if err != nil {
		return fmt.Errorf("failed to modify config: %w", err)
	}
	if validationErr != nil {
		return validationErr
	}
	return config.SaveConfig()
}

// RemoveApp removes an application from a project.
func (m *Manager) RemoveApp(projectName, appName string) error {
	var validationErr error
	err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
		for i, p := range cfg.GitOps.Projects {
			if p.Name == projectName {
				for j, a := range p.Apps {
					if a.Name == appName {
						copy(cfg.GitOps.Projects[i].Apps[j:], cfg.GitOps.Projects[i].Apps[j+1:])
						cfg.GitOps.Projects[i].Apps[len(cfg.GitOps.Projects[i].Apps)-1] = config.GitOpsApp{}
						cfg.GitOps.Projects[i].Apps = cfg.GitOps.Projects[i].Apps[:len(cfg.GitOps.Projects[i].Apps)-1]
						return
					}
				}
				validationErr = fmt.Errorf("app '%s' not found in project '%s'", appName, projectName)
				return
			}
		}
		validationErr = fmt.Errorf("project '%s' not found", projectName)
	})
	if err != nil {
		return fmt.Errorf("failed to modify config: %w", err)
	}
	if validationErr != nil {
		return validationErr
	}
	return config.SaveConfig()
}

// --- Credentials Operations ---

// AddCredential adds a new repository credential or updates the token if the URL exists.
func (m *Manager) AddCredential(cred config.GitOpsCredential) error {
	err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
		for i, c := range cfg.GitOps.Credentials {
			if c.URL == cred.URL {
				// Update existing credential
				cfg.GitOps.Credentials[i] = cred
				return
			}
		}
		cfg.GitOps.Credentials = append(cfg.GitOps.Credentials, cred)
	})
	if err != nil {
		return fmt.Errorf("failed to modify config: %w", err)
	}
	return config.SaveConfig()
}

// RemoveCredential removes a repository credential.
func (m *Manager) RemoveCredential(url string) error {
	var found bool
	err := config.ModifyConfig(func(cfg *config.ClusterConfig) {
		for i, c := range cfg.GitOps.Credentials {
			if c.URL == url {
				copy(cfg.GitOps.Credentials[i:], cfg.GitOps.Credentials[i+1:])
				cfg.GitOps.Credentials[len(cfg.GitOps.Credentials)-1] = config.GitOpsCredential{}
				cfg.GitOps.Credentials = cfg.GitOps.Credentials[:len(cfg.GitOps.Credentials)-1]
				found = true
				break
			}
		}
	})
	if err != nil {
		return fmt.Errorf("failed to modify config: %w", err)
	}
	if !found {
		return fmt.Errorf("credential for '%s' not found", url)
	}
	return config.SaveConfig()
}

