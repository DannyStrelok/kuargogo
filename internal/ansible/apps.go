package ansible

import (
	"fmt"
	"io"
	"log"
)

// RunAppDeploy executes the apps-deploy.yml playbook to install a K8s resource.
// manifestFile is the absolute path to the local YAML file to apply.
// limit restricts execution to specific hosts (e.g. the master node name).
func RunAppDeploy(dryRun bool, tags []string, limit string, manifestFile string, output io.Writer) (*Result, error) {
	if _, err := fmt.Fprintln(output, "Running Ansible Playbook: apps-deploy.yml"); err != nil {
		log.Printf("Warning: failed to write status: %v", err)
	}

	return runPlaybook("apps-deploy.yml", limit, dryRun, tags, map[string]string{
		"manifest_file": manifestFile,
	}, output, false)
}

// RunAppBackup executes the apps-backup.yml playbook to trigger a backup.
// limit restricts execution to specific hosts (e.g. the master node name).
func RunAppBackup(dryRun bool, tags []string, limit string, output io.Writer) (*Result, error) {
	if _, err := fmt.Fprintln(output, "Running Ansible Playbook: apps-backup.yml"); err != nil {
		log.Printf("Warning: failed to write status: %v", err)
	}

	return runPlaybook("apps-backup.yml", limit, dryRun, tags, nil, output, false)
}
