package uiinput

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestTextInput_focus_shows_caret(t *testing.T) {
	ti := &TextInput{Value: store.NewBinding("abc")}
	ti.ensureInit()
	ti.state.focused = true
	list := ti.project(facet.ProjectionContext{})
	foundCaret := false
	for _, cmd := range list.Commands {
		if _, ok := cmd.(gfx.FillRect); ok {
			foundCaret = true
			break
		}
	}
	if !foundCaret {
		t.Fatal("expected caret or field fill command")
	}
}

func TestTextInput_typing_updates_bound_store(t *testing.T) {
	ti := &TextInput{Value: store.NewBinding("")}
	ti.ensureInit()
	if !ti.handleText(facet.TextEvent{Text: "abc"}) {
		t.Fatal("expected text event to be handled")
	}
	if got := ti.Value.Get(); got != "abc" {
		t.Fatalf("value = %q, want abc", got)
	}
}

func TestTextInput_selection_geometry_updates_on_drag(t *testing.T) {
	ti := &TextInput{Value: store.NewBinding("abcdef")}
	ti.ensureInit()
	if !ti.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 8, Y: 8}}) {
		t.Fatal("expected press to be handled")
	}
	if !ti.handlePointer(facet.PointerEvent{Kind: platform.PointerMove, Position: gfx.Point{X: 40, Y: 8}}) {
		t.Fatal("expected move to be handled")
	}
	if ti.selection.IsEmpty() {
		t.Fatal("expected selection to be non-empty after drag")
	}
}

func TestTextInput_placeholder_hidden_when_nonempty(t *testing.T) {
	ti := &TextInput{Value: store.NewBinding("x"), Placeholder: "hint"}
	ti.ensureInit()
	list := ti.project(facet.ProjectionContext{})
	for _, cmd := range list.Commands {
		if _, ok := cmd.(gfx.DrawPoints); ok {
			t.Fatal("expected no placeholder draw command when non-empty")
		}
	}
}

func TestTextInput_ime_compose_then_commit(t *testing.T) {
	ti := &TextInput{Value: store.NewBinding("")}
	ti.ensureInit()
	if !ti.handleText(facet.TextEvent{Text: "pre", Composing: true}) {
		t.Fatal("expected compose to be handled")
	}
	if got := ti.Value.Get(); got != "" {
		t.Fatalf("value = %q, want empty during compose", got)
	}
	if !ti.handleText(facet.TextEvent{Text: "done", Composing: false}) {
		t.Fatal("expected commit to be handled")
	}
	if got := ti.Value.Get(); got != "done" {
		t.Fatalf("value = %q, want done", got)
	}
}

func TestTextInput_multiline_wrap_and_caret_positions(t *testing.T) {
	ti := &TextInput{Value: store.NewBinding("ab\ncd"), Multiline: true}
	ti.ensureInit()
	layout := ti.currentLayout()
	if layout.LineCount() != 2 {
		t.Fatalf("line count = %d, want 2", layout.LineCount())
	}
	if rect := layout.CaretRect(layout.PositionAtLineEnd(1)); rect.IsEmpty() {
		t.Fatal("expected non-empty caret rect")
	}
}

func TestTextInput_readonly_blocks_mutation_but_allows_selection(t *testing.T) {
	ti := &TextInput{Value: store.NewBinding("abc"), ReadOnly: true}
	ti.ensureInit()
	ti.state.focused = true
	ti.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyBackspace})
	if got := ti.Value.Get(); got != "abc" {
		t.Fatalf("value = %q, want abc", got)
	}
	if !ti.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 8, Y: 8}}) {
		t.Fatal("expected pointer selection to work")
	}
}

func TestTextInput_custom_assistive_renderer_preserves_focus_contract(t *testing.T) {
	ti := &TextInput{Value: store.NewBinding(""), Assistive: "help"}
	ti.ensureInit()
	ti.state.focused = true
	list := ti.project(facet.ProjectionContext{})
	if list == nil || len(list.Commands) == 0 {
		t.Fatal("expected commands for focused text input")
	}
}
