package store

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/signal"
)

func TestThemeMode_String(t *testing.T) {
	tests := []struct {
		mode ThemeMode
		want string
	}{
		{ThemeLight, "Light"},
		{ThemeDark, "Dark"},
		{ThemeSystem, "System"},
		{ThemeMode(99), "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("ThemeMode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllThemeModes(t *testing.T) {
	modes := AllThemeModes()
	if len(modes) != 3 {
		t.Errorf("AllThemeModes() returned %d modes, want 3", len(modes))
	}
	expected := []ThemeMode{ThemeLight, ThemeDark, ThemeSystem}
	for i, mode := range modes {
		if mode != expected[i] {
			t.Errorf("AllThemeModes()[%d] = %v, want %v", i, mode, expected[i])
		}
	}
}

func TestSetTheme_PreservesSelection(t *testing.T) {
	// Reset to known state
	SetTheme(ThemeLight)

	// Set theme should change the value
	SetTheme(ThemeDark)
	if got := GetTheme(); got != ThemeDark {
		t.Errorf("GetTheme() = %v, want %v", got, ThemeDark)
	}

	// Set theme again
	SetTheme(ThemeSystem)
	if got := GetTheme(); got != ThemeSystem {
		t.Errorf("GetTheme() = %v, want %v", got, ThemeSystem)
	}
}

func TestIsDarkMode(t *testing.T) {
	tests := []struct {
		mode     ThemeMode
		wantDark bool
	}{
		{ThemeLight, false},
		{ThemeDark, true},
		{ThemeSystem, false}, // System defaults to light for now
	}
	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			SetTheme(tt.mode)
			if got := IsDarkMode(); got != tt.wantDark {
				t.Errorf("IsDarkMode() = %v, want %v", got, tt.wantDark)
			}
		})
	}
}

func TestThemeChange_Notification(t *testing.T) {
	SetTheme(ThemeLight)

	var notified bool
	sub := ThemeStore.OnChange.Subscribe(func(change signal.Change[ThemeMode]) {
		notified = true
		if change.New != ThemeDark {
			t.Errorf("Expected change.New = ThemeDark, got %v", change.New)
		}
		if change.Old != ThemeLight {
			t.Errorf("Expected change.Old = ThemeLight, got %v", change.Old)
		}
	})
	defer ThemeStore.OnChange.Unsubscribe(sub)

	SetTheme(ThemeDark)

	if !notified {
		t.Error("ThemeStore.OnChange was not triggered")
	}
}

func TestThemeStore_ThreadSafety(t *testing.T) {
	// Run multiple goroutines setting different themes
	done := make(chan bool, 3)

	go func() {
		SetTheme(ThemeLight)
		done <- true
	}()
	go func() {
		SetTheme(ThemeDark)
		done <- true
	}()
	go func() {
		SetTheme(ThemeSystem)
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Final value should be one of the valid themes
	final := GetTheme()
	if final != ThemeLight && final != ThemeDark && final != ThemeSystem {
		t.Errorf("GetTheme() returned invalid value: %v", final)
	}
}
