package models

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DannyStrelok/kuargogo/internal/ui/menu/actions"
)

func TestNewKargoDashboardModel(t *testing.T) {
	m := NewKargoDashboardModel("auth-pipeline")
	if m.pipelineName != "auth-pipeline" {
		t.Errorf("expected pipelineName 'auth-pipeline', got %s", m.pipelineName)
	}
	if !m.loading {
		t.Error("expected dashboard to start in loading state")
	}
	if m.Title() != "Kargo Observability: auth-pipeline" {
		t.Errorf("expected Title 'Kargo Observability: auth-pipeline', got %s", m.Title())
	}
}

func TestKargoDashboardModel_Update_WindowSize(t *testing.T) {
	m := NewKargoDashboardModel("auth-pipeline")
	msg := tea.WindowSizeMsg{Width: 100, Height: 40}

	newModel, cmd := m.Update(msg)
	updatedM := newModel.(*KargoDashboardModel)

	if !updatedM.IsReady() {
		t.Error("expected model to be marked ready after WindowSizeMsg")
	}
	if updatedM.width != 100 || updatedM.height != 40 {
		t.Errorf("expected dimensions 100x40, got %dx%d", updatedM.width, updatedM.height)
	}
	if cmd != nil {
		t.Error("expected no command returned for WindowSizeMsg")
	}
}

func TestKargoDashboardModel_Update_PipelineObservabilityMsg(t *testing.T) {
	m := NewKargoDashboardModel("auth-pipeline")

	// Test success msg
	successMsg := actions.PipelineObservabilityMsg{
		Snapshot: actions.PipelineObservabilitySnapshot{
			PipelineName: "auth-pipeline",
			Project:      "homelab",
			Namespace:    "clandestino-app-dev",
			Stages: []actions.KargoStageSnapshot{
				{Name: "dev", CurrentFreight: "8ea3b345d", HealthStatus: "Healthy"},
			},
			Freights: []actions.KargoFreightSnapshot{
				{Name: "8ea3b345d", Alias: "vociferous-badger", CreationTime: time.Now()},
			},
		},
	}

	newModel, cmd := m.Update(successMsg)
	updatedM := newModel.(*KargoDashboardModel)

	if updatedM.loading {
		t.Error("expected loading to be false after data fetch")
	}
	if updatedM.err != nil {
		t.Errorf("expected err to be nil, got %v", updatedM.err)
	}
	if len(updatedM.snapshot.Stages) != 1 {
		t.Errorf("expected 1 stage, got %d", len(updatedM.snapshot.Stages))
	}
	if cmd != nil {
		t.Error("expected no command returned after data fetch success")
	}

	// Test error msg
	errVal := errors.New("cluster connection timeout")
	errorMsg := actions.PipelineObservabilityMsg{
		Error: errVal,
	}

	newModel, _ = m.Update(errorMsg)
	updatedM = newModel.(*KargoDashboardModel)

	if updatedM.loading {
		t.Error("expected loading to be false after error msg")
	}
	if updatedM.err != errVal {
		t.Errorf("expected error %v, got %v", errVal, updatedM.err)
	}
}

func TestFindArgoAppForStage(t *testing.T) {
	apps := []actions.ArgoAppSnapshot{
		{Name: "clandestino-app-dev", HealthStatus: "Healthy", SyncStatus: "Synced"},
		{Name: "clandestino-app-prod", HealthStatus: "Degraded", SyncStatus: "OutOfSync"},
	}

	app := findArgoAppForStage(apps, "dev")
	if app == nil || app.Name != "clandestino-app-dev" {
		t.Errorf("failed to match app for stage 'dev'")
	}

	app = findArgoAppForStage(apps, "prod")
	if app == nil || app.Name != "clandestino-app-prod" {
		t.Errorf("failed to match app for stage 'prod'")
	}

	app = findArgoAppForStage(apps, "test")
	if app != nil {
		t.Errorf("expected no app match for 'test'")
	}
}
