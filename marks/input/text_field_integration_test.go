package input

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// The integration tests prove the click-to-focus -> text-input -> store edit
// junction for the TextField mark through the runtime (Q7 path 1: mark-as-root).
// The store-mutation assertion is load-bearing.

// newTextFieldIntegration builds a mounted TextField harness, runs the warmup
// frame, and returns the field-bounds center for the click target.
func newTextFieldIntegration(t *testing.T, value *store.ValueStore[string]) (*testkit.Harness, float32, float32) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	t.Helper()
	tf := NewTextField(scope, "Email", uiinput.TextInputOutlined, value)
	h := testkit.NewStandardHarness(t, scope, 320, 120, tf)
	testkit.Warmup(h)

	field := tf.cachedFieldBounds
	if field.IsEmpty() {
		t.Fatal("expected field bounds after warmup")
	}
	cx := field.Min.X + field.Width()/2
	cy := field.Min.Y + field.Height()/2
	return h, cx, cy
}

func TestTextFieldIntegration_ClickFocusAndType(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	value := store.NewValueStore(scope, "")
	h, cx, cy := newTextFieldIntegration(t, value)

	// Click to focus the field, then type.
	testkit.DriveClickAt(h, cx, cy)
	testkit.DriveType(h, "hello")

	if got := value.Get(); got != "hello" {
		t.Fatalf("expected the typed text to be stored, got %q", got)
	}
}

func TestTextFieldIntegration_TypeAppendsToExistingValue(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	value := store.NewValueStore(scope, "")
	h, cx, cy := newTextFieldIntegration(t, value)

	testkit.DriveClickAt(h, cx, cy)
	testkit.DriveType(h, "hello")
	if got := value.Get(); got != "hello" {
		t.Fatalf("expected the first typed batch to be stored, got %q", got)
	}

	// The caret advanced to the end after the first insert; the second batch
	// appends at the caret.
	testkit.DriveType(h, " world")
	if got := value.Get(); got != "hello world" {
		t.Fatalf("expected the second typed batch to append, got %q", got)
	}
}
