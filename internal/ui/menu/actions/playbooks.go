package actions

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/DannyStrelok/kuargogo/internal/ansible"
)

// ExportPlaybooks starts the process of copying selected embedded playbooks
// to the user's home directory (~/.kuargogo/playbooks).
func ExportPlaybooks(selected []string, overwrite bool) tea.Cmd {
	return func() tea.Msg {
		summary, err := ansible.ExportPlaybooks(selected, overwrite)
		if err != nil {
			return ResultMsg{Output: fmt.Sprintf("❌ Export failed: %v\n\n%s", err, summary)}
		}
		return ResultMsg{Output: fmt.Sprintf("📂 Export Complete!\n\n%s", summary)}
	}
}
