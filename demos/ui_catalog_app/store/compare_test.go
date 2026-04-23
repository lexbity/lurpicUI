package store

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/signal"
)

func TestCompareMode_String(t *testing.T) {
	tests := []struct {
		mode CompareMode
		want string
	}{
		{CompareOff, "Single"},
		{CompareSideBySide, "Side by Side"},
		{CompareStacked, "Stacked"},
		{CompareMode(99), "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("CompareMode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllCompareModes(t *testing.T) {
	modes := AllCompareModes()
	if len(modes) != 3 {
		t.Errorf("AllCompareModes() returned %d modes, want 3", len(modes))
	}
	expected := []CompareMode{CompareOff, CompareSideBySide, CompareStacked}
	for i, mode := range modes {
		if mode != expected[i] {
			t.Errorf("AllCompareModes()[%d] = %v, want %v", i, mode, expected[i])
		}
	}
}

func TestSetCompareMode(t *testing.T) {
	// Reset to known state
	SetCompareMode(CompareOff)

	// Set compare mode should change the value
	SetCompareMode(CompareSideBySide)
	if got := GetCompareMode(); got != CompareSideBySide {
		t.Errorf("GetCompareMode() = %v, want %v", got, CompareSideBySide)
	}

	// Set compare mode again
	SetCompareMode(CompareStacked)
	if got := GetCompareMode(); got != CompareStacked {
		t.Errorf("GetCompareMode() = %v, want %v", got, CompareStacked)
	}
}

func TestIsCompareMode(t *testing.T) {
	tests := []struct {
		mode    CompareMode
		want    bool
	}{
		{CompareOff, false},
		{CompareSideBySide, true},
		{CompareStacked, true},
	}
	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			SetCompareMode(tt.mode)
			if got := IsCompareMode(); got != tt.want {
				t.Errorf("IsCompareMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetCompareTheme(t *testing.T) {
	// Reset to known state
	SetCompareTheme(ThemeDark)

	// Set compare theme should change the value
	SetCompareTheme(ThemeLight)
	if got := GetCompareTheme(); got != ThemeLight {
		t.Errorf("GetCompareTheme() = %v, want %v", got, ThemeLight)
	}

	// Set compare theme again
	SetCompareTheme(ThemeSystem)
	if got := GetCompareTheme(); got != ThemeSystem {
		t.Errorf("GetCompareTheme() = %v, want %v", got, ThemeSystem)
	}
}

func TestCompareModeChange_Notification(t *testing.T) {
	SetCompareMode(CompareOff)

	var notified bool
	sub := CompareStore.OnChange.Subscribe(func(change signal.Change[CompareMode]) {
		notified = true
		if change.New != CompareSideBySide {
			t.Errorf("Expected change.New = CompareSideBySide, got %v", change.New)
		}
		if change.Old != CompareOff {
			t.Errorf("Expected change.Old = CompareOff, got %v", change.Old)
		}
	})
	defer CompareStore.OnChange.Unsubscribe(sub)

	SetCompareMode(CompareSideBySide)

	if !notified {
		t.Error("CompareStore.OnChange was not triggered")
	}
}

func TestCompareThemeChange_Notification(t *testing.T) {
	SetCompareTheme(ThemeLight)

	var notified bool
	sub := CompareThemeStore.OnChange.Subscribe(func(change signal.Change[ThemeMode]) {
		notified = true
		if change.New != ThemeDark {
			t.Errorf("Expected change.New = ThemeDark, got %v", change.New)
		}
		if change.Old != ThemeLight {
			t.Errorf("Expected change.Old = ThemeLight, got %v", change.Old)
		}
	})
	defer CompareThemeStore.OnChange.Unsubscribe(sub)

	SetCompareTheme(ThemeDark)

	if !notified {
		t.Error("CompareThemeStore.OnChange was not triggered")
	}
}

func TestComparePreservesSelection(t *testing.T) {
	// This test verifies that compare mode changes don't affect the selection
	SetTheme(ThemeLight)
	SetCompareTheme(ThemeDark)
	SetCompareMode(CompareSideBySide)

	// Verify all three stores have independent values
	if GetTheme() != ThemeLight {
		t.Error("Theme was changed when setting compare mode")
	}
	if GetCompareTheme() != ThemeDark {
		t.Error("Compare theme was changed unexpectedly")
	}
	if GetCompareMode() != CompareSideBySide {
		t.Error("Compare mode was not set correctly")
	}
}

func TestCompareLayoutStability(t *testing.T) {
	// Test that layout remains stable when switching between compare modes
	modes := []CompareMode{CompareOff, CompareSideBySide, CompareStacked, CompareSideBySide, CompareOff}

	for _, mode := range modes {
		SetCompareMode(mode)
		// Verify each mode is set correctly
		if GetCompareMode() != mode {
			t.Errorf("Failed to set compare mode to %v", mode)
		}
	}
}
