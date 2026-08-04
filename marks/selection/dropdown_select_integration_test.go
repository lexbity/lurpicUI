package selection

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

// The integration tests prove the open -> keyboard-navigate -> select junction
// for the DropdownSelect mark through the runtime (Q7 path 1: mark-as-root).
// The listbox is opened with a pointer click, navigated with the keyboard, and
// committed with Enter; the store-mutation assertion is load-bearing.

func cityOptions() []DropdownOption {
	return []DropdownOption{
		{Value: "sydney", Label: "Sydney"},
		{Value: "melbourne", Label: "Melbourne"},
		{Value: "brisbane", Label: "Brisbane"},
	}
}

// mountDropdown mounts the dropdown as the harness root and runs the warmup
// frame so the trigger bounds exist.
func mountDropdown(t *testing.T, ds *DropdownSelect) *testkit.Harness {
	t.Helper()
	h := testkit.NewStandardHarness(t, 320, 140, ds)
	testkit.Warmup(h)
	return h
}

// openDropdown clicks the trigger to open the listbox. Reusing the same harness
// keeps the dropdown mounted once per test.
func openDropdown(t *testing.T, h *testkit.Harness, ds *DropdownSelect) {
	t.Helper()
	trigger := ds.cachedTriggerBounds
	if trigger.IsEmpty() {
		t.Fatal("expected trigger bounds after warmup")
	}
	cx := trigger.Min.X + trigger.Width()/2
	cy := trigger.Min.Y + trigger.Height()/2

	testkit.DriveClick(h, cx, cy)
	if !ds.open {
		t.Fatal("expected the trigger click to open the listbox")
	}
}

// commitWithEnter drives an Enter press+release, which selects the active option
// and closes the listbox.
func commitWithEnter(h *testkit.Harness) {
	testkit.DriveKeyPress(h, platform.KeyEnter, 0)
	testkit.DriveKeyRelease(h, platform.KeyEnter, 0)
}

func TestDropdownSelectIntegration_OpenNavigateSelect(t *testing.T) {
	value := store.NewValueStore("")
	ds := NewDropdownSelect("City", cityOptions(), value)

	h := mountDropdown(t, ds)
	openDropdown(t, h, ds)

	// Navigate two steps down, then commit with Enter.
	testkit.DriveKeyPress(h, platform.KeyDown, 0)
	testkit.DriveKeyPress(h, platform.KeyDown, 0)
	commitWithEnter(h)

	if got := value.Get(); got != "brisbane" {
		t.Fatalf("expected the selected value to be %q, got %q", "brisbane", got)
	}
	if ds.open {
		t.Fatal("expected Enter to close the listbox after selecting")
	}
}

func TestDropdownSelectIntegration_ReopenSelectsDifferentOption(t *testing.T) {
	value := store.NewValueStore("sydney")
	ds := NewDropdownSelect("City", cityOptions(), value)

	h := mountDropdown(t, ds)
	openDropdown(t, h, ds)

	// Move one option down and commit — replaces the current selection.
	testkit.DriveKeyPress(h, platform.KeyDown, 0)
	commitWithEnter(h)

	if got := value.Get(); got != "melbourne" {
		t.Fatalf("expected the selected value to change to %q, got %q", "melbourne", got)
	}
}

func TestDropdownSelectIntegration_SequenceChangesSelectionTwice(t *testing.T) {
	value := store.NewValueStore("")
	ds := NewDropdownSelect("City", cityOptions(), value)

	h := mountDropdown(t, ds)

	// First sequence: open, navigate to the last option, commit.
	openDropdown(t, h, ds)
	testkit.DriveKeyPress(h, platform.KeyDown, 0)
	testkit.DriveKeyPress(h, platform.KeyDown, 0)
	commitWithEnter(h)

	first := value.Get()
	if first != "brisbane" {
		t.Fatalf("expected the first selection to be %q, got %q", "brisbane", first)
	}

	// Second sequence: reopen on the same harness (active index syncs to the
	// current selection), jump to the first option, commit.
	openDropdown(t, h, ds)
	testkit.DriveKeyPress(h, platform.KeyHome, 0)
	commitWithEnter(h)

	second := value.Get()
	if second != "sydney" {
		t.Fatalf("expected the second selection to be %q, got %q", "sydney", second)
	}

	trace := []string{first, second}
	want := []string{"brisbane", "sydney"}
	for i := range want {
		if trace[i] != want[i] {
			t.Fatalf("selection trace[%d] = %q, want %q (full trace %v)", i, trace[i], want[i], trace)
		}
	}
}
