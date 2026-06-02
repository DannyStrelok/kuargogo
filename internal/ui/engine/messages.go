package engine

import (
	tea "charm.land/bubbletea/v2"
)

// ============================================================================
// Engine Navigation Commands
// ============================================================================

type PushMsg struct {
	Model EngineModel
}

func Push(m EngineModel) tea.Cmd {
	return func() tea.Msg {
		return PushMsg{Model: m}
	}
}

type PopMsg struct{}

func Pop() tea.Cmd {
	return func() tea.Msg {
		return PopMsg{}
	}
}

type ReplaceMsg struct {
	Model EngineModel
}

func Replace(m EngineModel) tea.Cmd {
	return func() tea.Msg {
		return ReplaceMsg{Model: m}
	}
}

type PopToRootMsg struct{}

func PopToRoot() tea.Cmd {
	return func() tea.Msg {
		return PopToRootMsg{}
	}
}

// ResumeMsg is sent to a model when it becomes the top model again after a pop.
type ResumeMsg struct{}

// ============================================================================
// Global Error Handling
// ============================================================================

type ErrorMsg struct {
	Err error
}

func ShowError(err error) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Err: err}
	}
}
// ============================================================================
// Configuration Sync
// ============================================================================

// ConfigReloadedMsg is sent when the underlying kuargogo.yaml has been modified
// either externally or via an internal action that called SaveConfig().
type ConfigReloadedMsg struct{}
