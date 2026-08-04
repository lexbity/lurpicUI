package navigation

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

// The integration tests prove the pointer, keyboard, and focus-traversal
// junctions for the Tabs mark through the runtime. Tabs is mounted as the
// harness root (Q7 path 1) for the interaction tests; the focus-traversal tests
// compose two or three Tabs marks in a blessed layout container (Q7 path 2).

func newIntegrationTabs(t *testing.T, active *store.ValueStore[int]) *Tabs {
	t.Helper()
	return NewTabs("Primary navigation", []TabItem{
		{Key: "overview", Label: "Overview"},
		{Key: "activity", Label: "Activity"},
		{Key: "settings", Label: "Settings"},
	}, active)
}

func TestTabsIntegration_ClickActivatesTab(t *testing.T) {
	active := store.NewValueStore(0)
	tabs := newIntegrationTabs(t, active)

	h := testkit.NewStandardHarness(t, 480, 200, tabs)
	testkit.Warmup(h)

	tab := tabs.cachedTabBounds[1]
	if tab.IsEmpty() {
		t.Fatal("expected the second tab to be arranged after warmup")
	}
	cx := tab.Min.X + tab.Width()/2
	cy := tab.Min.Y + tab.Height()/2

	testkit.DriveClick(h, cx, cy)

	if got := active.Get(); got != 1 {
		t.Fatalf("expected the click to activate the second tab (index 1), got %d", got)
	}
}

func TestTabsIntegration_KeyRightActivatesNextTab(t *testing.T) {
	active := store.NewValueStore(0)
	tabs := newIntegrationTabs(t, active)

	h := testkit.NewStandardHarness(t, 480, 200, tabs)
	h.Runtime().SetFocus(tabs)
	h.RunFrame()

	testkit.DriveKeyPress(h, platform.KeyRight, 0)
	if got := active.Get(); got != 1 {
		t.Fatalf("expected KeyRight to activate the next tab (index 1), got %d", got)
	}

	testkit.DriveKeyPress(h, platform.KeyRight, 0)
	if got := active.Get(); got != 2 {
		t.Fatalf("expected a second KeyRight to activate index 2, got %d", got)
	}
}

func TestTabsIntegration_TabKeyTraversesFocus(t *testing.T) {
	activeA := store.NewValueStore(0)
	activeB := store.NewValueStore(0)
	tabsA := newIntegrationTabs(t, activeA)
	tabsB := newIntegrationTabs(t, activeB)

	// Q7 path 2: compose the two focusable marks in a blessed layout container.
	col := layout.NewColumnLayout()
	col.Add(layout.Fixed(tabsA))
	col.Add(layout.Fixed(tabsB))

	h := testkit.NewStandardHarness(t, 480, 440, col)
	rt := h.Runtime()
	rt.SetFocus(tabsA)
	h.RunFrame()

	testkit.DriveKeyPress(h, platform.KeyTab, 0)

	if got := rt.FocusedID(); got != tabsB.Base().ID() {
		t.Fatalf("expected Tab to move focus to the second Tabs (id %d), got id %d", tabsB.Base().ID(), got)
	}

	testkit.DriveKeyPress(h, platform.KeyTab, 0)
	if got := rt.FocusedID(); got != tabsA.Base().ID() {
		t.Fatalf("expected Tab to wrap focus back to the first Tabs (id %d), got id %d", tabsA.Base().ID(), got)
	}
}

func TestTabsIntegration_TabVisitsEachInOrder(t *testing.T) {
	tabsA := newIntegrationTabs(t, store.NewValueStore(0))
	tabsB := newIntegrationTabs(t, store.NewValueStore(0))
	tabsC := newIntegrationTabs(t, store.NewValueStore(0))

	// Record every focus-gained transition, chaining the mark's own handler.
	// OnFocusGained fires once per transition (synchronously from
	// FocusManager.SetFocus); the redundant routed FocusGainedEvent path that
	// previously caused a double-fire was removed from the input layer.
	var trace []string
	recordFocus := func(name string, tabs *Tabs) {
		prev := tabs.Focus.OnFocusGained
		tabs.Focus.OnFocusGained = func() {
			if prev != nil {
				prev()
			}
			trace = append(trace, name)
		}
	}
	recordFocus("A", tabsA)
	recordFocus("B", tabsB)
	recordFocus("C", tabsC)

	col := layout.NewColumnLayout()
	col.Add(layout.Fixed(tabsA))
	col.Add(layout.Fixed(tabsB))
	col.Add(layout.Fixed(tabsC))

	h := testkit.NewStandardHarness(t, 480, 560, col)
	rt := h.Runtime()
	rt.SetFocus(tabsA)
	h.RunFrame()

	// SetFocus fires A's OnFocusGained immediately; the Tab-driven visits below
	// are what we assert, so clear the setup transition first.
	trace = trace[:0]

	testkit.DriveKeyPress(h, platform.KeyTab, 0)
	testkit.DriveKeyPress(h, platform.KeyTab, 0)

	want := []string{"B", "C"}
	if len(trace) != len(want) {
		t.Fatalf("focus visit trace = %v, want %v", trace, want)
	}
	for i := range want {
		if trace[i] != want[i] {
			t.Fatalf("focus visit trace[%d] = %q, want %q (full trace %v)", i, trace[i], want[i], trace)
		}
	}
}
