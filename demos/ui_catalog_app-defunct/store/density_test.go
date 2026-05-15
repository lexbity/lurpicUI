package store

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/signal"
)

func TestDensityMode_String(t *testing.T) {
	tests := []struct {
		mode DensityMode
		want string
	}{
		{DensityCompact, "Compact"},
		{DensityNormal, "Normal"},
		{DensityComfortable, "Comfortable"},
		{DensityMode(99), "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("DensityMode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDensityMode_SpacingScale(t *testing.T) {
	tests := []struct {
		mode  DensityMode
		want  float32
		delta float32
	}{
		{DensityCompact, 0.75, 0.01},
		{DensityNormal, 1.0, 0.01},
		{DensityComfortable, 1.25, 0.01},
		{DensityMode(99), 1.0, 0.01}, // Unknown defaults to 1.0
	}
	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			got := tt.mode.SpacingScale()
			if got < tt.want-tt.delta || got > tt.want+tt.delta {
				t.Errorf("DensityMode.SpacingScale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllDensityModes(t *testing.T) {
	modes := AllDensityModes()
	if len(modes) != 3 {
		t.Errorf("AllDensityModes() returned %d modes, want 3", len(modes))
	}
	expected := []DensityMode{DensityCompact, DensityNormal, DensityComfortable}
	for i, mode := range modes {
		if mode != expected[i] {
			t.Errorf("AllDensityModes()[%d] = %v, want %v", i, mode, expected[i])
		}
	}
}

func TestSetDensity_PreservesSelection(t *testing.T) {
	// Reset to known state
	SetDensity(DensityNormal)

	// Set density should change the value
	SetDensity(DensityCompact)
	if got := GetDensity(); got != DensityCompact {
		t.Errorf("GetDensity() = %v, want %v", got, DensityCompact)
	}

	// Set density again
	SetDensity(DensityComfortable)
	if got := GetDensity(); got != DensityComfortable {
		t.Errorf("GetDensity() = %v, want %v", got, DensityComfortable)
	}
}

func TestGetSpacing(t *testing.T) {
	SetDensity(DensityNormal)
	if got := GetSpacing(100); got != 100 {
		t.Errorf("GetSpacing(100) with DensityNormal = %v, want 100", got)
	}

	SetDensity(DensityCompact)
	if got := GetSpacing(100); got != 75 {
		t.Errorf("GetSpacing(100) with DensityCompact = %v, want 75", got)
	}

	SetDensity(DensityComfortable)
	if got := GetSpacing(100); got != 125 {
		t.Errorf("GetSpacing(100) with DensityComfortable = %v, want 125", got)
	}
}

func TestDensityChange_Notification(t *testing.T) {
	SetDensity(DensityNormal)

	var notified bool
	sub := DensityStore.OnChange.Subscribe(func(change signal.Change[DensityMode]) {
		notified = true
		if change.New != DensityCompact {
			t.Errorf("Expected change.New = DensityCompact, got %v", change.New)
		}
		if change.Old != DensityNormal {
			t.Errorf("Expected change.Old = DensityNormal, got %v", change.Old)
		}
	})
	defer DensityStore.OnChange.Unsubscribe(sub)

	SetDensity(DensityCompact)

	if !notified {
		t.Error("DensityStore.OnChange was not triggered")
	}
}

func TestDensityStore_ThreadSafety(t *testing.T) {
	// Run multiple goroutines setting different densities
	done := make(chan bool, 3)

	go func() {
		SetDensity(DensityCompact)
		done <- true
	}()
	go func() {
		SetDensity(DensityNormal)
		done <- true
	}()
	go func() {
		SetDensity(DensityComfortable)
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Final value should be one of the valid densities
	final := GetDensity()
	if final != DensityCompact && final != DensityNormal && final != DensityComfortable {
		t.Errorf("GetDensity() returned invalid value: %v", final)
	}
}
