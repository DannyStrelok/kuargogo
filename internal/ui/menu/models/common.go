package models

import (
	tea "charm.land/bubbletea/v2"
	"github.com/DannyStrelok/kuargogo/internal/ui/engine"
)

// ============================================================================
// Types
// ============================================================================

// MenuNode represents a node in the menu tree.
type MenuNode struct {
	Title           string
	Description     string
	Children        []MenuNode        // Static children
	DynamicChildren func() []MenuNode // Generate children dynamically
	Action          func() tea.Cmd    // Leaf action (returns the task to run)
	IsBack          bool              // Special "Back" item
}

// BaseModel provides common state for models that need to wait for window sizing.
// Embed this in your model. It provides default Implementations for EngineModel.
type BaseModel struct {
	Ready     bool
	theme     engine.Theme
	pageTitle string
	hideCrumb bool
}

func (b *BaseModel) ApplyTheme(t engine.Theme) {
	b.theme = t
}

func (b BaseModel) MouseMode() tea.MouseMode {
	return tea.MouseModeNone
}

// Title satisfies EngineModel to inform the engine of the current window/breadcrumb title.
func (b BaseModel) Title() string {
	if b.pageTitle != "" {
		return b.pageTitle
	}
	return "kuargogo"
}

// SetTitle updates the dynamic page title.
func (b *BaseModel) SetTitle(title string) {
	b.pageTitle = title
}

// ShowBreadcrumbs satisfies EngineModel. True by default.
func (b BaseModel) ShowBreadcrumbs() bool {
	return !b.hideCrumb
}

// HideBreadcrumbs explicitly disables the generic breadcrumb header.
func (b *BaseModel) HideBreadcrumbs() {
	b.hideCrumb = true
}

// MarkReady marks the model as ready (called after receiving WindowSizeMsg)
func (b *BaseModel) MarkReady() {
	b.Ready = true
}

// LoadingView returns a standard V2 declarative view for loading state.
func (b BaseModel) LoadingView() tea.View {
	return tea.NewView("Loading...")
}

// IsReady returns whether the model has been initialized with window size
func (b *BaseModel) IsReady() bool {
	return b.Ready
}

// AdjustedHeight calculates content height with a margin subtracted
func AdjustedHeight(windowHeight, margin int) int {
	h := windowHeight - margin
	if h < 3 {
		return 3
	}
	return h
}
