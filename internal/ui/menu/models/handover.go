package models

import (
	"fmt"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/ui/engine"
	"github.com/DannyStrelok/kuargogo/internal/ui/menu/actions"
)

// HandoverModel handles a two-step process:
// 1. Perform a background task (like syncing kubeconfig)
// 2. Execute a terminal handover (like launching k9s)
type HandoverModel struct {
	BaseModel
	status  string
	syncErr error
	done    bool
}

type syncDoneMsg struct{ err error }

func NewK9sHandoverModel() *HandoverModel {
	m := &HandoverModel{
		status: "🔄 Sincronizando configuración con el cluster...",
	}
	m.SetTitle("LIVE K9S DASHBOARD")
	return m
}

func (m *HandoverModel) Init() tea.Cmd {
	return func() tea.Msg {
		err := actions.RemoteSyncKubeconfig()
		return syncDoneMsg{err: err}
	}
}

func (m *HandoverModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case syncDoneMsg:
		m.done = true
		if msg.err != nil {
			m.syncErr = msg.err
			m.status = fmt.Sprintf("⚠️  Error de sincronización: %v\n\nAbriendo k9s con la configuración local actual...", msg.err)
		} else {
			m.status = "✅ Sincronización completa. Iniciando k9s..."
		}

		// Proceed to handover
		cfg := config.GetConfig()
		kubeconfig, _ := cfg.K3s.ExpandedKubeconfigPath()

		cmd := exec.Command("k9s")
		if kubeconfig != "" {
			cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
		}

		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return actions.ResultMsg{Output: fmt.Sprintf("❌ Error al lanzar k9s: %v", err)}
			}
			// Pop itself when k9s exits to return to menu
			return engine.Pop()()
		})

	case tea.KeyPressMsg:
		if msg.String() == "q" || msg.String() == "esc" {
			return m, engine.Pop()
		}
	}
	return m, nil
}

func (m *HandoverModel) View() tea.View {
	return tea.NewView(m.status + "\n\nPresiona 'q' para cancelar.")
}
