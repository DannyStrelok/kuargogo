package models

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/DannyStrelok/kuargogo/internal/ui/engine"
)

type FormModel struct {
	BaseModel
	form         *huh.Form
	submitAction func(*huh.Form) tea.Cmd
}

func NewFormModel(form *huh.Form, onSubmit func(*huh.Form) tea.Cmd) *FormModel {
	m := &FormModel{
		form:         form,
		submitAction: onSubmit,
	}
	m.SetTitle("Form Input")
	return m
}

func (m *FormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle escape key to cancel form
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if keyMsg.String() == "esc" {
			return m, engine.Pop()
		}
		if keyMsg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	// Delegate to huh form
	formModel, cmd := m.form.Update(msg)
	if f, ok := formModel.(*huh.Form); ok {
		m.form = f
	}

	if m.form.State == huh.StateCompleted {
		taskCmd := m.submitAction(m.form)

		// Replace this form with OutputModel to avoid leaving empty form in stack
		outModel := NewOutputModel(taskCmd)
		return m, engine.Replace(outModel)
	}

	return m, cmd
}

func (m *FormModel) View() tea.View {
	return tea.NewView("\n" + m.form.View())
}
