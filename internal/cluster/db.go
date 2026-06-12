package cluster

import (
	"encoding/json"
	"fmt"
	"time"
)

// CNPGBackup represents a parsed CNPG backup metadata.
type CNPGBackup struct {
	Name      string `json:"name"`
	Phase     string `json:"phase"`
	Cluster   string `json:"cluster"`
	CreatedAt string `json:"createdAt"`
}

type backupsCNPGJSONList struct {
	Items []backupCNPGJSONItem `json:"items"`
}

type backupCNPGJSONItem struct {
	Metadata struct {
		Name              string `json:"name"`
		Namespace         string `json:"namespace"`
		CreationTimestamp string `json:"creationTimestamp"`
	} `json:"metadata"`
	Spec struct {
		Cluster struct {
			Name string `json:"name"`
		} `json:"cluster"`
	} `json:"spec"`
	Status struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type cnpgBackupStatusJSON struct {
	Status struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

// ParseTargetTime validates and parses date strings in RFC3339 or "YYYY-MM-DD HH:MM:SS" format.
func ParseTargetTime(timeStr string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err == nil {
		return t, nil
	}

	const simpleLayout = "2006-01-02 15:04:05"
	t, err = time.ParseInLocation(simpleLayout, timeStr, time.UTC)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid time format. Use RFC3339 (2006-01-02T15:04:05Z) or YYYY-MM-DD HH:MM:SS")
}

// GeneratePITRManifest generates the recovery Cluster YAML/JSON manifest by copying the source cluster's spec.
func GeneratePITRManifest(sourceClusterMap map[string]interface{}, targetName string, targetTime time.Time) (string, error) {
	spec, ok := sourceClusterMap["spec"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid source cluster manifest structure")
	}

	backup, ok := spec["backup"].(map[string]interface{})
	if !ok || backup == nil || backup["barmanObjectStore"] == nil {
		return "", fmt.Errorf("source cluster does not have backup.barmanObjectStore configured")
	}

	barmanStore := backup["barmanObjectStore"].(map[string]interface{})

	// Deep copy the original spec via JSON marshal/unmarshal
	specBytes, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("failed to marshal original spec: %w", err)
	}

	var newSpec map[string]interface{}
	if err := json.Unmarshal(specBytes, &newSpec); err != nil {
		return "", fmt.Errorf("failed to unmarshal spec copy: %w", err)
	}

	// Setup recovery bootstrap section
	newSpec["bootstrap"] = map[string]interface{}{
		"recovery": map[string]interface{}{
			"source": targetName + "-recovery-source",
			"recoveryTarget": map[string]interface{}{
				"targetTime": targetTime.Format(time.RFC3339),
			},
		},
	}

	// Configure external clusters pointing to the same barman store
	newSpec["externalClusters"] = []map[string]interface{}{
		{
			"name":              targetName + "-recovery-source",
			"barmanObjectStore": barmanStore,
		},
	}

	// Construct the final unstructured Cluster map
	recoveryManifest := map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata": map[string]interface{}{
			"name": targetName,
		},
		"spec": newSpec,
	}

	manifestBytes, err := json.Marshal(recoveryManifest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal recovery manifest: %w", err)
	}

	return string(manifestBytes), nil
}

// CreateCNPGBackup triggers a manual CNPG backup in the cluster via SSH.
func (m *Manager) CreateCNPGBackup(masterIP, namespace, clusterName, backupName string) (string, error) {
	if m.DryRun {
		_, _ = fmt.Fprintf(m.Output, "[DRY RUN] Would create CNPG backup %s for cluster %s in namespace %s\n", backupName, clusterName, namespace)
		return backupName, nil
	}

	type cnpgBackupSpec struct {
		Cluster struct {
			Name string `json:"name"`
		} `json:"cluster"`
	}
	type cnpgBackupDef struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec cnpgBackupSpec `json:"spec"`
	}

	def := cnpgBackupDef{
		APIVersion: "postgresql.cnpg.io/v1",
		Kind:       "Backup",
	}
	def.Metadata.Name = backupName
	def.Metadata.Namespace = namespace
	def.Spec.Cluster.Name = clusterName

	jsonBytes, err := json.Marshal(def)
	if err != nil {
		return "", fmt.Errorf("failed to marshal backup definition: %w", err)
	}

	cmd := fmt.Sprintf("sudo k3s kubectl apply -f - << 'EOF'\n%s\nEOF", string(jsonBytes))

	executor, err := m.getExecutor()
	if err != nil {
		return "", err
	}

	_, err = executor.ExecuteCommand(masterIP, m.Port, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to apply CNPG backup resource: %w", err)
	}

	return backupName, nil
}

// GetCNPGBackupStatus queries the phase status of a CNPG backup operation via SSH.
func (m *Manager) GetCNPGBackupStatus(masterIP, namespace, backupName string) (string, error) {
	if m.DryRun {
		return "completed", nil
	}

	cmd := fmt.Sprintf("sudo k3s kubectl get backup.postgresql.cnpg.io -n %s %s -o json", namespace, backupName)
	executor, err := m.getExecutor()
	if err != nil {
		return "", err
	}

	out, err := executor.ExecuteCommand(masterIP, m.Port, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get backup status: %w", err)
	}

	var rawStatus cnpgBackupStatusJSON
	if err := json.Unmarshal([]byte(out), &rawStatus); err != nil {
		return "", fmt.Errorf("failed to parse backup status JSON: %w", err)
	}

	phase := rawStatus.Status.Phase
	if phase == "" {
		phase = "pending"
	}

	return phase, nil
}

// ListCNPGBackups lists CNPG backups from the cluster via SSH.
func (m *Manager) ListCNPGBackups(masterIP, namespace, clusterName string) ([]CNPGBackup, error) {
	if m.DryRun {
		now := time.Now()
		return []CNPGBackup{
			{
				Name:      "manual-backup-1",
				Phase:     "completed",
				Cluster:   clusterName,
				CreatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
			},
			{
				Name:      "manual-backup-2",
				Phase:     "completed",
				Cluster:   clusterName,
				CreatedAt: now.Format(time.RFC3339),
			},
		}, nil
	}

	cmd := fmt.Sprintf("sudo k3s kubectl get backup.postgresql.cnpg.io -n %s -o json", namespace)
	executor, err := m.getExecutor()
	if err != nil {
		return nil, err
	}

	out, err := executor.ExecuteCommand(masterIP, m.Port, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	var rawList backupsCNPGJSONList
	if err := json.Unmarshal([]byte(out), &rawList); err != nil {
		return nil, fmt.Errorf("failed to parse CNPG backups JSON list: %w", err)
	}

	var backups []CNPGBackup
	for _, item := range rawList.Items {
		// Filter by cluster if requested
		if clusterName != "" && item.Spec.Cluster.Name != clusterName {
			continue
		}
		backups = append(backups, CNPGBackup{
			Name:      item.Metadata.Name,
			Phase:     item.Status.Phase,
			Cluster:   item.Spec.Cluster.Name,
			CreatedAt: item.Metadata.CreationTimestamp,
		})
	}

	return backups, nil
}

// GetCNPGCluster retrieves the raw cluster configuration as a map via SSH.
func (m *Manager) GetCNPGCluster(masterIP, namespace, clusterName string) (map[string]interface{}, error) {
	if m.DryRun {
		return map[string]interface{}{
			"apiVersion": "postgresql.cnpg.io/v1",
			"kind":       "Cluster",
			"metadata": map[string]interface{}{
				"name":      clusterName,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"instances": 3,
				"imageName": "ghcr.io/cloudnative-pg/postgresql:18",
				"storage": map[string]interface{}{
					"size": "10Gi",
				},
				"backup": map[string]interface{}{
					"barmanObjectStore": map[string]interface{}{
						"destinationPath": "s3://homelab-clandestino/barman",
					},
				},
			},
		}, nil
	}

	cmd := fmt.Sprintf("sudo k3s kubectl get cluster.postgresql.cnpg.io -n %s %s -o json", namespace, clusterName)
	executor, err := m.getExecutor()
	if err != nil {
		return nil, err
	}

	out, err := executor.ExecuteCommand(masterIP, m.Port, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve cluster: %w", err)
	}

	var clusterMap map[string]interface{}
	if err := json.Unmarshal([]byte(out), &clusterMap); err != nil {
		return nil, fmt.Errorf("failed to parse cluster JSON: %w", err)
	}

	return clusterMap, nil
}

// ApplyCNPGClusterManifest applies a CNPG Cluster manifest via SSH.
func (m *Manager) ApplyCNPGClusterManifest(masterIP, namespace string, manifestYaml []byte) error {
	if m.DryRun {
		_, _ = fmt.Fprintf(m.Output, "[DRY RUN] Would apply CNPG Cluster manifest in namespace %s:\n%s\n", namespace, string(manifestYaml))
		return nil
	}

	cmd := fmt.Sprintf("sudo k3s kubectl apply -n %s -f - << 'EOF'\n%s\nEOF", namespace, string(manifestYaml))
	executor, err := m.getExecutor()
	if err != nil {
		return err
	}

	_, err = executor.ExecuteCommand(masterIP, m.Port, cmd)
	if err != nil {
		return fmt.Errorf("failed to apply CNPG cluster manifest: %w", err)
	}

	return nil
}

// DeleteCNPGCluster deletes a CNPG Cluster and all its associated PVCs via SSH to enable clean restores.
func (m *Manager) DeleteCNPGCluster(masterIP, namespace, clusterName string) error {
	if m.DryRun {
		_, _ = fmt.Fprintf(m.Output, "[DRY RUN] Would delete CNPG Cluster %s and its associated PVCs in namespace %s\n", clusterName, namespace)
		return nil
	}

	executor, err := m.getExecutor()
	if err != nil {
		return err
	}

	// 1. Delete the Cluster CRD and wait for it to be fully removed
	_, _ = fmt.Fprintf(m.Output, "⏳ Deleting CNPG Cluster %s...\n", clusterName)
	deleteClusterCmd := fmt.Sprintf("sudo k3s kubectl delete cluster.postgresql.cnpg.io -n %s %s --wait=true", namespace, clusterName)
	_, err = executor.ExecuteCommand(masterIP, m.Port, deleteClusterCmd)
	if err != nil {
		return fmt.Errorf("failed to delete CNPG Cluster: %w", err)
	}

	// 2. Delete the PVCs labeled with this cluster to prevent dirty bootstrapping
	_, _ = fmt.Fprintf(m.Output, "🧹 Cleaning up PVCs for cluster %s to allow clean restoration...\n", clusterName)
	deletePvcCmd := fmt.Sprintf("sudo k3s kubectl delete pvc -n %s -l cnpg.io/cluster=%s", namespace, clusterName)
	_, err = executor.ExecuteCommand(masterIP, m.Port, deletePvcCmd)
	if err != nil {
		return fmt.Errorf("failed to delete associated PVCs: %w", err)
	}

	return nil
}
