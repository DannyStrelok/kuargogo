package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/gitops"
)

// PromoteStage triggers a Kargo promotion for a specific stage under a specified pipeline.
func PromoteStage(pipelineName string, stageName string, freightID string) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		if len(cfg.GitOps.Pipelines) == 0 {
			return ResultMsg{Output: "❌ Kargo is not configured in kuargogo.yaml"}
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
			} else {
				return ResultMsg{Output: fmt.Sprintf("❌ Pipeline %q not found in kuargogo.yaml", pipelineName)}
			}
		}

		kubeconfig, err := cfg.K3s.ExpandedKubeconfigPath()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error resolving kubeconfig: %v", err)}
		}

		ns := p.Project
		if ns == "" {
			ns = p.Namespace
		}
		if ns == "" {
			ns = "kargo"
		}

		svc := gitops.NewKargoService(kubeconfig, config.IsDryRun())
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		out, err := svc.Promote(ctx, ns, stageName, freightID)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error promoting to %s: %v\n%s", stageName, err, out)}
		}
		return ResultMsg{Output: fmt.Sprintf("✅ Promotion to %s started successfully!\n%s", stageName, out)}
	}
}

// GetKargoFreight returns a list of available freight IDs in the namespace of a specific pipeline.
func GetKargoFreight(pipelineName string) tea.Cmd {
	return func() tea.Msg {
		cfg := config.GetConfig()
		if len(cfg.GitOps.Pipelines) == 0 {
			return ResultMsg{Output: "❌ Kargo is not configured in kuargogo.yaml"}
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
			} else {
				return ResultMsg{Output: fmt.Sprintf("❌ Pipeline %q not found in kuargogo.yaml", pipelineName)}
			}
		}

		kubeconfig, err := cfg.K3s.ExpandedKubeconfigPath()
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error: %v", err)}
		}

		ns := p.Project
		if ns == "" {
			ns = p.Namespace
		}
		if ns == "" {
			ns = "kargo"
		}

		svc := gitops.NewKargoService(kubeconfig, config.IsDryRun())
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		freightList, err := svc.GetFreight(ctx, ns)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Error fetching freight: %v", err)}
		}

		if len(freightList) == 0 {
			return ResultMsg{Output: "ℹ️ No freight available yet. Wait for Warehouse to sync."}
		}

		return ResultMsg{Output: fmt.Sprintf("📦 Available Freight in %s:\n%s", ns, strings.Join(freightList, "\n"))}
	}
}

// SyncKargoState reconciles Kargo resources in the cluster.
func SyncKargoState() tea.Cmd {
	return func() tea.Msg {
		ch := make(chan string, 10)
		go func() {
			defer close(ch)
			writer := NewProgressWriter(ch)

			msg := "🚢 Starting Kargo Resources Synchronization...\n"
			if config.IsDryRun() {
				msg = "🧪 [DRY RUN] Starting Kargo simulation...\n"
			}
			_, _ = writer.Write([]byte(msg))

			cfg := config.GetConfig()
			orch := gitops.NewOrchestrator()
			orch.Output = writer

			err := orch.Sync(cfg)
			if err != nil {
				ch <- fmt.Sprintf("\n❌ Sync failed: %v", err)
				return
			}
			ch <- "\n✅ Kargo & GitOps state synchronized."
		}()

		return ActionStartedMsg{ProgressChan: ch}
	}
}
