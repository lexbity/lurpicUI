package marks_test

// The declared-bindings conformance walk (RX-2 FR-1 / AC-3): every mark in the
// collection, every exported binding field — a dynamic binding swapped in
// pre-attach must be subscribed at attach, must invalidate on a store write,
// and must unsubscribe on dispose. allMarks() is the explicit enumeration
// that replaced marks/registry.go; a mark added to the collection without a
// factory here fails the walk (the count assertion below enforces it).

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/internal/bindingsources"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/status"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/marks/viz"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// allMarksFactories maps one mark factory per collection mark. The walker
// builds a fresh instance per binding field. Keep the map keyed by descriptor
// type name so the count assertion reads like the catalog.
var allMarksFactories = map[string]func(*store.Scope) facet.FacetImpl{
	"action_bar": func(*store.Scope) facet.FacetImpl { return action.NewActionBar("actions", nil) },
	"action_group": func(*store.Scope) facet.FacetImpl {
		return action.NewActionGroup(marks.Const("group"), marks.Const([]action.ActionGroupAction(nil)))
	},
	"button": func(*store.Scope) facet.FacetImpl {
		return action.NewButton(marks.Const("Save"), marks.Const(uiinput.ButtonFilled))
	},
	"command_palette": func(scope *store.Scope) facet.FacetImpl {
		return action.NewCommandPalette(scope, marks.Const("palette"), nil, store.NewValueStore(scope, false))
	},
	"icon_button":  func(*store.Scope) facet.FacetImpl { return action.NewIconButton(primitive.IconSVG("M0 0h1v1z")) },
	"menu_button":  func(*store.Scope) facet.FacetImpl { return action.NewMenuButton("menu", nil) },
	"ribbon":       func(scope *store.Scope) facet.FacetImpl { return action.NewRibbon(scope, "ribbon", nil, nil) },
	"split_button": func(*store.Scope) facet.FacetImpl { return action.NewSplitButton("split", nil) },
	"toolbar":      func(*store.Scope) facet.FacetImpl { return action.NewToolbar(marks.Const("toolbar"), nil, nil) },
	"alert":        func(*store.Scope) facet.FacetImpl { return feedback.NewAlert("title", "message") },
	"scrim":        func(*store.Scope) facet.FacetImpl { return feedback.NewScrim() },
	"dialog": func(scope *store.Scope) facet.FacetImpl {
		return feedback.NewDialog("title", "body", nil, store.NewValueStore(scope, false))
	},
	"notification": func(scope *store.Scope) facet.FacetImpl {
		return feedback.NewNotification("title", "message", store.NewValueStore(scope, false))
	},
	"tooltip": func(scope *store.Scope) facet.FacetImpl {
		return feedback.NewTooltip(scope, "content", store.NewValueStore(scope, false))
	},
	"color_picker": func(scope *store.Scope) facet.FacetImpl { return input.NewColorPicker(scope, "color", nil) },
	"number_field": func(scope *store.Scope) facet.FacetImpl {
		return input.NewNumberField(scope, "count", 0, 0, 1, -1, nil)
	},
	"text_field": func(scope *store.Scope) facet.FacetImpl {
		return input.NewTextField(scope, "name", uiinput.TextInputOutlined, nil)
	},
	"breadcrumbs": func(scope *store.Scope) facet.FacetImpl {
		return navigation.NewBreadcrumbs("crumbs", nil, store.NewValueStore(scope, 0))
	},
	"nav_drawer": func(scope *store.Scope) facet.FacetImpl {
		return navigation.NewNavDrawer(scope, "drawer", nil, store.NewValueStore(scope, false), store.NewValueStore(scope, 0))
	},
	"nav_rail": func(scope *store.Scope) facet.FacetImpl {
		return navigation.NewNavRail("rail", nil, store.NewValueStore(scope, 0))
	},
	"pagination": func(scope *store.Scope) facet.FacetImpl {
		return navigation.NewPagination("pages", nil, store.NewValueStore(scope, 0))
	},
	"tabs": func(scope *store.Scope) facet.FacetImpl {
		return navigation.NewTabs("tabs", nil, store.NewValueStore(scope, 0))
	},
	"tree_navigator": func(scope *store.Scope) facet.FacetImpl {
		return navigation.NewTreeNavigator(scope, "tree", nil, store.NewValueStore(scope, ""))
	},
	"icon": func(*store.Scope) facet.FacetImpl { return primitive.NewIcon(primitive.IconSVG("M0 0h1v1z")) },
	"text": func(*store.Scope) facet.FacetImpl { return primitive.NewText(marks.Const("hello")) },
	"button_group": func(scope *store.Scope) facet.FacetImpl {
		return selection.NewButtonGroup(scope, "mode", nil, store.NewValueStore(scope, []string{}))
	},
	"checkbox": func(scope *store.Scope) facet.FacetImpl {
		return selection.NewCheckbox(scope, "check", store.NewValueStore(scope, selection.CheckboxStateOff))
	},
	"dropdown_select": func(scope *store.Scope) facet.FacetImpl {
		return selection.NewDropdownSelect(scope, "pick", nil, store.NewValueStore(scope, ""))
	},
	"list_item": func(*store.Scope) facet.FacetImpl { return selection.NewListItem(marks.Const("item")) },
	"radio_group": func(scope *store.Scope) facet.FacetImpl {
		return selection.NewRadioGroup(scope, "radio", nil, store.NewValueStore(scope, ""))
	},
	"slider": func(scope *store.Scope) facet.FacetImpl {
		return selection.NewSlider(scope, "gain", 0, 1, 0.01, store.NewValueStore(scope, 0.5))
	},
	"switch": func(scope *store.Scope) facet.FacetImpl {
		return selection.NewSwitch(scope, "on", store.NewValueStore(scope, true))
	},
	"turn_dial": func(scope *store.Scope) facet.FacetImpl {
		return selection.NewTurnDial(scope, "dial", 0, 1, 0.01, store.NewValueStore(scope, 0.5))
	},
	"badge":         func(*store.Scope) facet.FacetImpl { return status.NewBadge("badge") },
	"progress_bar":  func(*store.Scope) facet.FacetImpl { return status.NewProgressBar("progress") },
	"progress_ring": func(*store.Scope) facet.FacetImpl { return status.NewProgressRing("progress") },
	"status_light":  func(*store.Scope) facet.FacetImpl { return status.NewStatusLight("light") },
	"card":          func(*store.Scope) facet.FacetImpl { return structure.NewCard("card") },
	"column":        func(*store.Scope) facet.FacetImpl { return structure.NewColumn(nil, structure.AxisConfig{}) },
	"divider":       func(*store.Scope) facet.FacetImpl { return structure.NewDivider() },
	"list":          func(scope *store.Scope) facet.FacetImpl { return structure.NewList(scope, "list", nil) },
	"row":           func(*store.Scope) facet.FacetImpl { return structure.NewRow(nil, structure.AxisConfig{}) },
	"scroll_region": func(*store.Scope) facet.FacetImpl { return structure.NewScrollRegion("scroll") },
	"table": func(scope *store.Scope) facet.FacetImpl {
		return structure.NewTable(scope, "table", structure.TableData{}, nil)
	},
	"area":  func(*store.Scope) facet.FacetImpl { return viz.NewArea[int](nil, nil, nil, nil, nil) },
	"axis":  func(*store.Scope) facet.FacetImpl { return viz.NewAxis(nil, marks.Const(viz.AxisBottom), nil) },
	"bar":   func(*store.Scope) facet.FacetImpl { return viz.NewBar[int](nil, nil, nil, nil) },
	"line":  func(*store.Scope) facet.FacetImpl { return viz.NewLine[int](nil, nil, nil, nil, nil) },
	"point": func(*store.Scope) facet.FacetImpl { return viz.NewPoint[int](nil, nil, nil, nil, nil) },
	"rule": func(*store.Scope) facet.FacetImpl {
		return viz.NewRule(marks.Const(float64(0)), viz.RuleHorizontal, nil)
	},
}

// expectedMarkCount mirrors the coverage catalog's standardMarks count
// (demos/lurpic_studio/studio/coverage_test.go). When a mark joins the
// collection, both lists grow together — a mismatch here means the
// conformance walk is silently skipping a mark.
const expectedMarkCount = 50

func TestDeclaredBindings_allMarks_alive(t *testing.T) {
	// The walker mints dynamic bindings per element type; the registrations
	// live in bindingsources (kept out of testkit to avoid test import
	// cycles with the marks subpackages).
	bindingsources.Register()
	if len(allMarksFactories) != expectedMarkCount {
		t.Fatalf("allMarksFactories has %d marks, want %d — a mark joined or left the collection without updating the conformance walk", len(allMarksFactories), expectedMarkCount)
	}
	for name, mk := range allMarksFactories {
		t.Run(name, func(t *testing.T) {
			scope := store.NewScope()
			t.Cleanup(scope.Close)
			testkit.AssertBindingsAlive(t, scope, mk)
		})
	}
}
