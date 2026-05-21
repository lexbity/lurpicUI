package uistruct

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/theme"
)

func TestResolveCardRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveCardRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "CardSurface", "HeaderOptional", "MediaOptional", "Body", "ActionsOptional", "FocusRingOptional"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected card slot source for %s", name)
		}
	}
	if slots.Root.Base.Opacity != 0 {
		t.Fatal("expected transparent root slot")
	}
	if slots.CardSurface.Base.Fills == nil {
		t.Fatal("expected card surface fill")
	}
	if slots.Body.Base.Opacity != 0 {
		t.Fatal("expected transparent body slot")
	}
}

func TestResolveListRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveListRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "ListContainer", "ListItems", "SectionHeaderOptional", "EmptyStateOptional"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected list slot source for %s", name)
		}
	}
	if slots.Root.Base.Opacity != 0 {
		t.Fatal("expected transparent root slot")
	}
	if slots.ListContainer.Base.Fills == nil {
		t.Fatal("expected list container fill")
	}
}

func TestResolveTableRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveTableRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "TableSurface", "HeaderRow", "HeaderCell", "BodyRows", "BodyCell", "SelectionColumnOptional", "SortIndicator", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected table slot source for %s", name)
		}
	}
	if slots.Root.Base.Opacity != 0 {
		t.Fatal("expected transparent root slot")
	}
	if slots.TableSurface.Base.Fills == nil {
		t.Fatal("expected table surface fill")
	}
	if slots.FocusRing.Base.Strokes == nil {
		t.Fatal("expected table focus ring stroke")
	}
}
