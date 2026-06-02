package actions

import (
	tea "charm.land/bubbletea/v2"
)

// ProgressWriter implements io.Writer and sends every write as a ProgressMsg to the channel.
type ProgressWriter struct {
	ch chan<- string
}

func NewProgressWriter(ch chan<- string) *ProgressWriter {
	return &ProgressWriter{ch: ch}
}

func (w *ProgressWriter) Write(p []byte) (n int, err error) {
	w.ch <- string(p)
	return len(p), nil
}

// StreamCmd waits for a string from the channel and returns it as a ProgressMsg.
// If the channel is closed, it returns nil.
func StreamCmd(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		out, ok := <-ch
		if !ok {
			return ProgressFinishedMsg{}
		}
		return ProgressMsg{Output: out}
	}
}
