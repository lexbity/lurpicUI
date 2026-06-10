package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func testStatusState() *state.AppState {
	rows := make([]dataset.Row, 40)
	for i := 0; i < 40; i++ {
		rows[i] = dataset.Row{
			Revenue: float64(1000 + i*100),
			Users:   float64(100 + i*10),
			Region:  []string{"NA", "EU", "APAC", "LATAM"}[i%4],
		}
	}
	return state.NewAppState(rows)
}

func TestStatusBarConstructs(t *testing.T) {
	s := testStatusState()
	sb := NewStatusBar(s)
	if sb == nil {
		t.Fatal("NewStatusBar returned nil")
	}
	if sb.ProgressBar() == nil {
		t.Fatal("StatusBar has no ProgressBar")
	}
	if sb.ProgressRing() == nil {
		t.Fatal("StatusBar has no ProgressRing")
	}
	if sb.StatusLight() == nil {
		t.Fatal("StatusBar has no StatusLight")
	}
	if sb.Badge() == nil {
		t.Fatal("StatusBar has no Badge")
	}
	if sb.Text() == nil {
		t.Fatal("StatusBar has no Text")
	}
}

func TestStatusBarInitialProgress(t *testing.T) {
	s := testStatusState()
	sb := NewStatusBar(s)
	if sb.IsReloading() {
		t.Error("should not be reloading initially")
	}
	if s.JobProgress.Get() != 0 {
		t.Errorf("expected JobProgress 0, got %f", s.JobProgress.Get())
	}
}

func TestStatusBarStartReloadSetsConnecting(t *testing.T) {
	s := testStatusState()
	s.Connection.Set(state.ConnDisconnected)
	s.JobProgress.Set(0)
	sb := NewStatusBar(s)
	sb.StartReload()
	if s.Connection.Get() != state.ConnConnecting {
		t.Errorf("expected Connection Connecting after StartReload, got %v", s.Connection.Get())
	}
	if s.JobProgress.Get() != 0 {
		t.Errorf("expected JobProgress 0 after StartReload, got %f", s.JobProgress.Get())
	}
}

func TestStatusBarProgressTicksMonotonically(t *testing.T) {
	s := testStatusState()
	sb := NewStatusBar(s)
	sb.StartReload()

	prev := float32(-1)
	for i := 0; i < 25; i++ {
		sb.onTick()
		curr := s.JobProgress.Get()
		if curr < prev {
			t.Errorf("progress went backwards from %f to %f at tick %d", prev, curr, i)
		}
		prev = curr
	}
	if s.JobProgress.Get() != 1.0 {
		t.Errorf("expected JobProgress 1.0 at end, got %f", s.JobProgress.Get())
	}
}

func TestStatusBarProgressReachesOne(t *testing.T) {
	s := testStatusState()
	sb := NewStatusBar(s)
	sb.StartReload()
	for i := 0; i < 30; i++ {
		sb.onTick()
	}
	if s.JobProgress.Get() != 1.0 {
		t.Errorf("expected JobProgress 1.0, got %f", s.JobProgress.Get())
	}
	if s.Connection.Get() != state.ConnConnected {
		t.Errorf("expected Connection Connected, got %v", s.Connection.Get())
	}
	if sb.IsReloading() {
		t.Error("should not be reloading after completion")
	}
}

func TestStatusBarProgressStaysAtOne(t *testing.T) {
	s := testStatusState()
	sb := NewStatusBar(s)
	sb.StartReload()
	for i := 0; i < 30; i++ {
		sb.onTick()
	}
	for i := 0; i < 10; i++ {
		sb.onTick()
	}
	if s.JobProgress.Get() != 1.0 {
		t.Errorf("expected JobProgress to stay at 1.0, got %f", s.JobProgress.Get())
	}
}

func TestStatusBarCancelReload(t *testing.T) {
	s := testStatusState()
	sb := NewStatusBar(s)
	sb.StartReload()
	sb.onTick()
	sb.onTick()
	sb.CancelReload()
	if sb.IsReloading() {
		t.Error("should not be reloading after cancel")
	}
	if s.JobProgress.Get() != 0 {
		t.Errorf("expected JobProgress 0 after cancel, got %f", s.JobProgress.Get())
	}
	if s.Connection.Get() != state.ConnDisconnected {
		t.Errorf("expected Connection Disconnected after cancel, got %v", s.Connection.Get())
	}
}

func TestStatusBarCancelPreventsFurtherProgress(t *testing.T) {
	s := testStatusState()
	sb := NewStatusBar(s)
	sb.StartReload()
	sb.onTick()
	sb.CancelReload()
	sb.onTick()
	if s.JobProgress.Get() != 0 {
		t.Errorf("expected JobProgress 0 after cancel even with more ticks, got %f", s.JobProgress.Get())
	}
}

func TestStatusBarStartReloadInitializes(t *testing.T) {
	s := testStatusState()
	sb := NewStatusBar(s)
	s.JobProgress.Set(0.5)
	sb.StartReload()
	if s.JobProgress.Get() != 0 {
		t.Errorf("expected JobProgress reset to 0, got %f", s.JobProgress.Get())
	}
	if s.Connection.Get() != state.ConnConnecting {
		t.Errorf("expected Connection Connecting, got %v", s.Connection.Get())
	}
	if !sb.IsReloading() {
		t.Error("should be reloading after StartReload")
	}
	if sb.Text().Content.Get() != "Reloading..." {
		t.Errorf("expected text 'Reloading...', got %q", sb.Text().Content.Get())
	}
}

func TestStatusBarBadgeAfterReload(t *testing.T) {
	s := testStatusState()
	sb := NewStatusBar(s)
	sb.UpdateBadge(s)
	initialBadge := sb.Badge().Label.Get()
	if initialBadge == "" {
		t.Error("badge should not be empty")
	}
}

func TestStatusBarMeasures(t *testing.T) {
	s := testStatusState()
	sb := NewStatusBar(s)
	result := sb.Base().LayoutRole().Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 32}})
	if result.Size.W != 1280 || result.Size.H != 32 {
		t.Errorf("expected 1280x32, got %v", result.Size)
	}
}
