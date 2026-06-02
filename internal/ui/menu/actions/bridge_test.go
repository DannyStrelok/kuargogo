package actions

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestProgressWriter(t *testing.T) {
	ch := make(chan string, 5)
	writer := NewProgressWriter(ch)

	testStr := "Hello, World!"
	n, err := writer.Write([]byte(testStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(testStr) {
		t.Errorf("expected %d bytes written, got %d", len(testStr), n)
	}

	select {
	case out := <-ch:
		if out != testStr {
			t.Errorf("expected %q from channel, got %q", testStr, out)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for message from channel")
	}
}

func TestStreamCmd(t *testing.T) {
	ch := make(chan string, 5)
	cmd := StreamCmd(ch)

	// Send message
	testStr := "Progress update"
	ch <- testStr

	msg := cmd()
	progressMsg, ok := msg.(ProgressMsg)
	if !ok {
		t.Fatalf("expected ProgressMsg, got %T", msg)
	}
	if progressMsg.Output != testStr {
		t.Errorf("expected output %q, got %q", testStr, progressMsg.Output)
	}

	// Close channel
	close(ch)
	msg = cmd()
	if _, ok := msg.(ProgressFinishedMsg); !ok {
		t.Errorf("expected ProgressFinishedMsg after channel close, got %T", msg)
	}
}

func TestStreamingActionPattern(t *testing.T) {
	// Simulate what an action does
	action := func() tea.Cmd {
		return func() tea.Msg {
			ch := make(chan string, 10)
			go func() {
				defer close(ch)
				ch <- "Step 1"
				ch <- "Step 2"
			}()
			return ActionStartedMsg{ProgressChan: ch}
		}
	}

	cmd := action()
	msg := cmd()
	startedMsg, ok := msg.(ActionStartedMsg)
	if !ok {
		t.Fatalf("expected ActionStartedMsg, got %T", msg)
	}

	// Wait for first progress
	progressCmd := StreamCmd(startedMsg.ProgressChan)
	msg1 := progressCmd()
	p1, ok := msg1.(ProgressMsg)
	if !ok || p1.Output != "Step 1" {
		t.Errorf("expected 'Step 1', got %v (ok: %t)", p1.Output, ok)
	}

	// Wait for second progress
	msg2 := progressCmd()
	p2, ok := msg2.(ProgressMsg)
	if !ok || p2.Output != "Step 2" {
		t.Errorf("expected 'Step 2', got %v (ok: %t)", p2.Output, ok)
	}

	// Wait for completion
	msg3 := progressCmd()
	if _, ok := msg3.(ProgressFinishedMsg); !ok {
		t.Errorf("expected ProgressFinishedMsg after completion, got %v", msg3)
	}
}
