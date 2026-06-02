package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// KargoStageSnapshot represents a stage status snapshot
type KargoStageSnapshot struct {
	Name           string
	CurrentFreight string
	HealthStatus   string
	ReadyStatus    string
	Message        string
}

// KargoFreightSnapshot represents a freight resource status snapshot
type KargoFreightSnapshot struct {
	Name           string
	Alias          string
	CreationTime   time.Time
	ImageRepo      string
	ImageTag       string
	ActiveInStages []string
}

// ArgoAppSnapshot represents an ArgoCD application status snapshot
type ArgoAppSnapshot struct {
	Name         string
	HealthStatus string
	SyncStatus   string
}

// PipelineObservabilitySnapshot groups the full live status of a pipeline
type PipelineObservabilitySnapshot struct {
	PipelineName  string
	Project       string
	Namespace     string
	WarehouseName string
	Stages        []KargoStageSnapshot
	Freights      []KargoFreightSnapshot
	ArgoApps      []ArgoAppSnapshot
}

// JSON parsing structs for kubectl output

type k8sFreightList struct {
	Items []struct {
		Metadata struct {
			Name              string    `json:"name"`
			CreationTimestamp time.Time `json:"creationTimestamp"`
		} `json:"metadata"`
		Alias  string `json:"alias"`
		Images []struct {
			RepoURL string `json:"repoURL"`
			Tag     string `json:"tag"`
		} `json:"images"`
		Status struct {
			CurrentlyIn map[string]interface{} `json:"currentlyIn"`
		} `json:"status"`
	} `json:"items"`
}

type k8sStage struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status struct {
		FreightSummary string `json:"freightSummary"`
		Health         struct {
			Status string `json:"status"`
		} `json:"health"`
		Conditions []struct {
			Type    string `json:"type"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"conditions"`
	} `json:"status"`
}

type k8sStageList struct {
	Items []k8sStage `json:"items"`
}

type k8sArgoAppList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Status struct {
			Health struct {
				Status string `json:"status"`
			} `json:"health"`
			Sync struct {
				Status string `json:"status"`
			} `json:"sync"`
		} `json:"status"`
	} `json:"items"`
}

// KargoService handles promotion and observability operations for Kargo in the cluster.
type KargoService struct {
	Kubeconfig string
	DryRun     bool
}

// NewKargoService creates a new KargoService instance.
func NewKargoService(kubeconfig string, dryRun bool) *KargoService {
	return &KargoService{
		Kubeconfig: kubeconfig,
		DryRun:     dryRun,
	}
}

// Promote triggers a Kargo stage promotion by creating a Promotion CRD.
func (s *KargoService) Promote(ctx context.Context, namespace, stageName, freightID string) (string, error) {
	promotionYAML := fmt.Sprintf(`
apiVersion: kargo.akuity.io/v1alpha1
kind: Promotion
metadata:
  generateName: promote-%s-
  namespace: %s
spec:
  stage: %s
  freight: %s
`, stageName, namespace, stageName, freightID)

	if s.DryRun {
		return fmt.Sprintf("🧪 [DRY RUN] Promotion manifest generated:\n%s", promotionYAML), nil
	}

	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "kubectl", "--kubeconfig", s.Kubeconfig, "create", "-f", "-")
	cmd.Stdin = strings.NewReader(promotionYAML)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("kubectl create promotion: %w\n%s", err, string(out))
	}

	return string(out), nil
}

// GetFreight lists available freight IDs in the specified namespace.
func (s *KargoService) GetFreight(ctx context.Context, namespace string) ([]string, error) {
	if s.DryRun {
		return []string{"freight-dryrun-1", "freight-dryrun-2"}, nil
	}

	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "kubectl", "--kubeconfig", s.Kubeconfig, "get", "freight", "-n", namespace, "-o", "custom-columns=NAME:.metadata.name", "--no-headers")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl get freight: %w\n%s", err, string(out))
	}

	rawLines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var freight []string
	for _, line := range rawLines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			freight = append(freight, trimmed)
		}
	}
	return freight, nil
}

// QueryObservability retrieves the live status of the pipelines and their stages.
func (s *KargoService) QueryObservability(ctx context.Context, pipelineName string) (PipelineObservabilitySnapshot, error) {
	cfg := config.GetConfig()
	if len(cfg.GitOps.Pipelines) == 0 {
		return PipelineObservabilitySnapshot{}, fmt.Errorf("kargo is not configured in kuargogo.yaml")
	}

	var p config.KargoPipeline
	found := false
	for _, pipe := range cfg.GitOps.Pipelines {
		if pipe.Name == pipelineName {
			p = pipe
			found = true
			break
		}
	}
	if !found {
		if pipelineName == "" {
			p = cfg.GitOps.Pipelines[0]
			pipelineName = p.Name
		} else {
			return PipelineObservabilitySnapshot{}, fmt.Errorf("pipeline %q not found in kuargogo.yaml", pipelineName)
		}
	}

	ns := p.Project
	if ns == "" {
		ns = p.Namespace
	}
	if ns == "" {
		ns = "kargo"
	}

	snapshot := PipelineObservabilitySnapshot{
		PipelineName:  pipelineName,
		Project:       p.Project,
		Namespace:     ns,
		WarehouseName: p.Warehouse.Name,
	}

	if s.DryRun {
		// Mock dry run observability data
		snapshot.Stages = []KargoStageSnapshot{
			{Name: "dev", CurrentFreight: "dev-freight-mock", HealthStatus: "Healthy", ReadyStatus: "True", Message: "Dry run mode"},
		}
		snapshot.Freights = []KargoFreightSnapshot{
			{Name: "dev-freight-mock", Alias: "mock-alias", CreationTime: time.Now()},
		}
		return snapshot, nil
	}

	// 1. Fetch Freights
	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "kubectl", "--kubeconfig", s.Kubeconfig, "get", "freight", "-n", ns, "-o", "json")
	freightBytes, err := cmd.Output()
	if err != nil {
		return PipelineObservabilitySnapshot{}, fmt.Errorf("failed to fetch Kargo Freight: %w", err)
	}

	var fList k8sFreightList
	if err := json.Unmarshal(freightBytes, &fList); err != nil {
		return PipelineObservabilitySnapshot{}, fmt.Errorf("failed to parse Kargo Freight JSON: %w", err)
	}

	for _, item := range fList.Items {
		fSnap := KargoFreightSnapshot{
			Name:         item.Metadata.Name,
			Alias:        item.Alias,
			CreationTime: item.Metadata.CreationTimestamp,
		}
		if len(item.Images) > 0 {
			fSnap.ImageRepo = item.Images[0].RepoURL
			fSnap.ImageTag = item.Images[0].Tag
		}
		for stageName := range item.Status.CurrentlyIn {
			fSnap.ActiveInStages = append(fSnap.ActiveInStages, stageName)
		}
		snapshot.Freights = append(snapshot.Freights, fSnap)
	}

	// 2. Fetch Stages
	stageCtx, stageCancel := context.WithTimeout(ctx, 15*time.Second)
	defer stageCancel()

	cmd = exec.CommandContext(stageCtx, "kubectl", "--kubeconfig", s.Kubeconfig, "get", "stage", "-n", ns, "-o", "json")
	stageBytes, err := cmd.Output()
	if err != nil {
		return PipelineObservabilitySnapshot{}, fmt.Errorf("failed to fetch Kargo Stages: %w", err)
	}

	var sList k8sStageList
	if err := json.Unmarshal(stageBytes, &sList); err != nil {
		return PipelineObservabilitySnapshot{}, fmt.Errorf("failed to parse Kargo Stage JSON: %w", err)
	}

	stageMap := make(map[string]k8sStage)
	for _, item := range sList.Items {
		stageMap[item.Metadata.Name] = item
	}

	// 3. Fetch ArgoCD Apps (to overlay health status)
	argoCtx, argoCancel := context.WithTimeout(ctx, 15*time.Second)
	defer argoCancel()

	cmd = exec.CommandContext(argoCtx, "kubectl", "--kubeconfig", s.Kubeconfig, "get", "application", "-n", "argocd", "-o", "json")
	argoBytes, err := cmd.Output()
	var argoList k8sArgoAppList
	argoFetched := false
	if err == nil {
		if json.Unmarshal(argoBytes, &argoList) == nil {
			argoFetched = true
		}
	}

	// Process configured pipeline stages in order
	for _, stgConf := range p.Stages {
		stgSnap := KargoStageSnapshot{
			Name:           stgConf.Name,
			CurrentFreight: "Unknown",
			HealthStatus:   "Unknown",
			ReadyStatus:    "Unknown",
			Message:        "Not found in cluster",
		}

		if liveStg, ok := stageMap[stgConf.Name]; ok {
			stgSnap.CurrentFreight = liveStg.Status.FreightSummary
			if liveStg.Status.Health.Status != "" {
				stgSnap.HealthStatus = liveStg.Status.Health.Status
			} else {
				stgSnap.HealthStatus = "Unknown"
			}

			for _, cond := range liveStg.Status.Conditions {
				if cond.Type == "Ready" {
					stgSnap.ReadyStatus = cond.Status
					stgSnap.Message = cond.Message
					break
				}
			}
		}

		snapshot.Stages = append(snapshot.Stages, stgSnap)

		// Try to find corresponding ArgoCD app status
		if argoFetched {
			appItem := findArgoAppForStage(argoList.Items, p.Project, stgConf.Name)
			if appItem != nil {
				appSnap := ArgoAppSnapshot{
					Name:         appItem.Metadata.Name,
					HealthStatus: appItem.Status.Health.Status,
					SyncStatus:   appItem.Status.Sync.Status,
				}
				snapshot.ArgoApps = append(snapshot.ArgoApps, appSnap)
			}
		}
	}

	return snapshot, nil
}

// Helper function to match Kargo project/stage to ArgoCD applications
func findArgoAppForStage(items []struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status struct {
		Health struct {
			Status string `json:"status"`
		} `json:"health"`
		Sync struct {
			Status string `json:"status"`
		} `json:"sync"`
	} `json:"status"`
}, project string, stage string) *struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status struct {
		Health struct {
			Status string `json:"status"`
		} `json:"health"`
		Sync struct {
			Status string `json:"status"`
		} `json:"sync"`
	} `json:"status"`
} {
	// 1. Try exact match project-stage
	target := fmt.Sprintf("%s-%s", project, stage)
	for i := range items {
		if strings.EqualFold(items[i].Metadata.Name, target) {
			return &items[i]
		}
	}

	// 2. Try match project & stage substring
	for i := range items {
		name := strings.ToLower(items[i].Metadata.Name)
		if strings.Contains(name, strings.ToLower(project)) && strings.Contains(name, strings.ToLower(stage)) {
			return &items[i]
		}
	}

	// 3. Try match stage substring
	for i := range items {
		name := strings.ToLower(items[i].Metadata.Name)
		if strings.Contains(name, strings.ToLower(stage)) {
			return &items[i]
		}
	}

	return nil
}
