package cluster

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// VeleroBackup represents a parsed Velero backup metadata.
type VeleroBackup struct {
	Name                string `json:"name"`
	Phase               string `json:"phase"`
	StartTimestamp      string `json:"startTimestamp"`
	CompletionTimestamp string `json:"completionTimestamp"`
	TTL                 string `json:"ttl"`
}

type backupsJSONList struct {
	Items []backupJSONItem `json:"items"`
}

type backupJSONItem struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status struct {
		Phase               string `json:"phase"`
		StartTimestamp      string `json:"startTimestamp"`
		CompletionTimestamp string `json:"completionTimestamp"`
	} `json:"status"`
	Spec struct {
		TTL string `json:"ttl"`
	} `json:"spec"`
}

type restoreJSONStatus struct {
	Status struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

// ListVeleroBackups lists all backups from the cluster via SSH.
func (m *Manager) ListVeleroBackups(masterIP string) ([]VeleroBackup, error) {
	if m.DryRun {
		now := time.Now()
		return []VeleroBackup{
			{
				Name:                "clandestino-db-daily-2026-06-10",
				Phase:               "Completed",
				StartTimestamp:      now.Add(-24 * time.Hour).Format(time.RFC3339),
				CompletionTimestamp: now.Add(-24 * time.Hour).Add(15 * time.Second).Format(time.RFC3339),
				TTL:                 "240h0m0s",
			},
			{
				Name:                "clandestino-db-daily-2026-06-11",
				Phase:               "Completed",
				StartTimestamp:      now.Format(time.RFC3339),
				CompletionTimestamp: now.Add(12 * time.Second).Format(time.RFC3339),
				TTL:                 "240h0m0s",
			},
		}, nil
	}

	cmd := "sudo k3s kubectl get backups.velero.io -n velero -o json"
	executor, err := m.getExecutor()
	if err != nil {
		return nil, err
	}

	out, err := executor.ExecuteCommand(masterIP, m.Port, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command on master: %w", err)
	}

	var rawList backupsJSONList
	if err := json.Unmarshal([]byte(out), &rawList); err != nil {
		return nil, fmt.Errorf("failed to parse backups JSON output: %w", err)
	}

	var backups []VeleroBackup
	for _, item := range rawList.Items {
		backups = append(backups, VeleroBackup{
			Name:                item.Metadata.Name,
			Phase:               item.Status.Phase,
			StartTimestamp:      item.Status.StartTimestamp,
			CompletionTimestamp: item.Status.CompletionTimestamp,
			TTL:                 item.Spec.TTL,
		})
	}

	return backups, nil
}

// StartVeleroRestore triggers a Velero restore operation in the cluster via SSH.
// It returns the generated name of the Restore resource.
func (m *Manager) StartVeleroRestore(masterIP string, backupName string, namespaces []string) (string, error) {
	randSuffix := generateRandomSuffix(5)
	restoreName := fmt.Sprintf("restore-%s-%s", backupName, randSuffix)
	if len(restoreName) > 253 {
		restoreName = restoreName[:253]
	}

	if m.DryRun {
		_, _ = fmt.Fprintf(m.Output, "[DRY RUN] Would execute restore creation for backup %s as %s\n", backupName, restoreName)
		return restoreName, nil
	}

	type restoreSpec struct {
		BackupName         string   `json:"backupName"`
		IncludedNamespaces []string `json:"includedNamespaces,omitempty"`
	}
	type restoreDef struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec restoreSpec `json:"spec"`
	}

	def := restoreDef{
		APIVersion: "velero.io/v1",
		Kind:       "Restore",
	}
	def.Metadata.Name = restoreName
	def.Metadata.Namespace = "velero"
	def.Spec.BackupName = backupName
	if len(namespaces) > 0 {
		def.Spec.IncludedNamespaces = namespaces
	}

	jsonBytes, err := json.Marshal(def)
	if err != nil {
		return "", fmt.Errorf("failed to marshal restore definition: %w", err)
	}

	cmd := fmt.Sprintf("sudo k3s kubectl apply -f - << 'EOF'\n%s\nEOF", string(jsonBytes))

	executor, err := m.getExecutor()
	if err != nil {
		return "", err
	}

	_, err = executor.ExecuteCommand(masterIP, m.Port, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to apply restore manifest: %w", err)
	}

	return restoreName, nil
}

// GetVeleroRestoreStatus queries the phase status of a restore operation via SSH.
func (m *Manager) GetVeleroRestoreStatus(masterIP string, restoreName string) (string, error) {
	if m.DryRun {
		return "Completed", nil
	}

	cmd := fmt.Sprintf("sudo k3s kubectl get restore.velero.io -n velero %s -o json", restoreName)
	executor, err := m.getExecutor()
	if err != nil {
		return "", err
	}

	out, err := executor.ExecuteCommand(masterIP, m.Port, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get restore status: %w", err)
	}

	var rawStatus restoreJSONStatus
	if err := json.Unmarshal([]byte(out), &rawStatus); err != nil {
		return "", fmt.Errorf("failed to parse restore status JSON: %w", err)
	}

	phase := rawStatus.Status.Phase
	if phase == "" {
		phase = "New"
	}

	return phase, nil
}

func generateRandomSuffix(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
