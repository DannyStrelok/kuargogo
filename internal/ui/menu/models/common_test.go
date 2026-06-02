package models

import "testing"

func TestBaseModel_MarkReady(t *testing.T) {
	b := BaseModel{}
	if b.IsReady() {
		t.Error("expected IsReady() false initially")
	}

	b.MarkReady()
	if !b.IsReady() {
		t.Error("expected IsReady() true after MarkReady()")
	}
}

func TestBaseModel_LoadingView(t *testing.T) {
	b := BaseModel{}
	view := b.LoadingView()
	if view.Content != "Loading..." {
		t.Errorf("expected 'Loading...', got %s", view.Content)
	}
}

func TestAdjustedHeight(t *testing.T) {
	tests := []struct {
		windowHeight int
		margin       int
		expected     int
	}{
		{100, 10, 90},
		{20, 10, 10},
		{10, 10, 3}, // minimum
		{5, 10, 3},  // below minimum, clamped to 3
	}

	for _, tc := range tests {
		result := AdjustedHeight(tc.windowHeight, tc.margin)
		if result != tc.expected {
			t.Errorf("AdjustedHeight(%d, %d) = %d, want %d", tc.windowHeight, tc.margin, result, tc.expected)
		}
	}
}
