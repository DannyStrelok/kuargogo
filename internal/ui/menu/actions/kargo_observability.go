package actions

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/gitops"
)

// Type aliases to preserve compatibility with existing TUI views/models
type KargoStageSnapshot = gitops.KargoStageSnapshot
type KargoFreightSnapshot = gitops.KargoFreightSnapshot
type ArgoAppSnapshot = gitops.ArgoAppSnapshot
type PipelineObservabilitySnapshot = gitops.PipelineObservabilitySnapshot

// PipelineObservabilityMsg is the Bubble Tea message returned upon data fetch
type PipelineObservabilityMsg struct {
	Snapshot PipelineObservabilitySnapshot
	Error    error
}

// GetKargoObservability queries live Kubernetes state and returns it as a Bubble Tea Cmd.
func GetKargoObservability(pipelineName string) tea.Cmd {
	return func() tea.Msg {
		snapshot, err := QueryKargoObservability(pipelineName)
		if err != nil {
			return PipelineObservabilityMsg{Error: err}
		}
		return PipelineObservabilityMsg{Snapshot: snapshot}
	}
}

// QueryKargoObservability queries live Kubernetes state and returns the snapshot.
func QueryKargoObservability(pipelineName string) (PipelineObservabilitySnapshot, error) {
	cfg := config.GetConfig()
	kubeconfig, err := cfg.K3s.ExpandedKubeconfigPath()
	if err != nil {
		return PipelineObservabilitySnapshot{}, fmt.Errorf("failed to expand kubeconfig path: %w", err)
	}

	svc := gitops.NewKargoService(kubeconfig, config.IsDryRun())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return svc.QueryObservability(ctx, pipelineName)
}
