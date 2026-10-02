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
var allMarksFactories = map[string]func() facet.FacetImpl{
	"action_bar": func() facet.FacetImpl { return action.NewActionBar("actions", nil) },
	"action_group": func() facet.FacetImpl {
		return action.NewActionGroup(marks.Const("group"), marks.Const([]action.ActionGroupAction(nil)))
	},
	"button": func() facet.FacetImpl {
		return action.NewButton(marks.Const("Save"), marks.Const(uiinput.ButtonFilled))
	},
	"command_palette": func() facet.FacetImpl {
		return action.NewCommandPalette(marks.Const("palette"), nil, store.NewValueStore(false))
	},
	"icon_button":   func() facet.FacetImpl { return action.NewIconButton(primitive.IconSVG("M0 0h1v1z")) },
	"menu_button":   func() facet.FacetImpl { return action.NewMenuButton("menu", nil) },
	"popup_palette": func() facet.FacetImpl { return action.NewPopupPalette("tools", nil, store.NewValueStore(false)) },
	"radial_menu":   func() facet.FacetImpl { return action.NewRadialMenu("radial", nil, nil) },
	"ribbon":        func() facet.FacetImpl { return action.NewRibbon("ribbon", nil) },
	"split_button":  func() facet.FacetImpl { return action.NewSplitButton("split", nil) },
	"toolbar":       func() facet.FacetImpl { return action.NewToolbar(marks.Const("toolbar"), nil, nil) },
	"alert":         func() facet.FacetImpl { return feedback.NewAlert("title", "message") },
	"scrim":         func() facet.FacetImpl { return feedback.NewScrim() },
	"dialog":        func() facet.FacetImpl { return feedback.NewDialog("title", "body", nil, store.NewValueStore(false)) },
	"notification": func() facet.FacetImpl {
		return feedback.NewNotification("title", "message", store.NewValueStore(false))
	},
	"tooltip":      func() facet.FacetImpl { return feedback.NewTooltip("content", store.NewValueStore(false)) },
	"color_picker": func() facet.FacetImpl { return input.NewColorPicker("color", nil) },
	"number_field": func() facet.FacetImpl { return input.NewNumberField("count", nil) },
	"text_field":   func() facet.FacetImpl { return input.NewTextField("name", uiinput.TextInputOutlined, nil) },
	"breadcrumbs":  func() facet.FacetImpl { return navigation.NewBreadcrumbs("crumbs", nil, store.NewValueStore(0)) },
	"nav_drawer": func() facet.FacetImpl {
		return navigation.NewNavDrawer("drawer", nil, store.NewValueStore(false), store.NewValueStore(0))
	},
	"nav_rail":       func() facet.FacetImpl { return navigation.NewNavRail("rail", nil, store.NewValueStore(0)) },
	"pagination":     func() facet.FacetImpl { return navigation.NewPagination("pages", nil, store.NewValueStore(0)) },
	"tabs":           func() facet.FacetImpl { return navigation.NewTabs("tabs", nil, store.NewValueStore(0)) },
	"tree_navigator": func() facet.FacetImpl { return navigation.NewTreeNavigator("tree", nil, store.NewValueStore("")) },
	"icon":           func() facet.FacetImpl { return primitive.NewIcon(primitive.IconSVG("M0 0h1v1z")) },
	"text":           func() facet.FacetImpl { return primitive.NewText(marks.Const("hello")) },
	"button_group":   func() facet.FacetImpl { return selection.NewButtonGroup("mode", nil, store.NewValueStore([]string{})) },
	"checkbox": func() facet.FacetImpl {
		return selection.NewCheckbox("check", store.NewValueStore(selection.CheckboxStateOff))
	},
	"dropdown_select": func() facet.FacetImpl { return selection.NewDropdownSelect("pick", nil, store.NewValueStore("")) },
	"list_item":       func() facet.FacetImpl { return selection.NewListItem(marks.Const("item")) },
	"radio_group":     func() facet.FacetImpl { return selection.NewRadioGroup("radio", nil, store.NewValueStore("")) },
	"slider":          func() facet.FacetImpl { return selection.NewSlider("gain", 0, 1, 0.01, store.NewValueStore(0.5)) },
	"switch":          func() facet.FacetImpl { return selection.NewSwitch("on", store.NewValueStore(true)) },
	"turn_dial":       func() facet.FacetImpl { return selection.NewTurnDial("dial", 0, 1, 0.01, store.NewValueStore(0.5)) },
	"badge":           func() facet.FacetImpl { return status.NewBadge("badge") },
	"progress_bar":    func() facet.FacetImpl { return status.NewProgressBar("progress") },
	"progress_ring":   func() facet.FacetImpl { return status.NewProgressRing("progress") },
	"status_light":    func() facet.FacetImpl { return status.NewStatusLight("light") },
	"card":            func() facet.FacetImpl { return structure.NewCard("card") },
	"column":          func() facet.FacetImpl { return structure.NewColumn(nil, structure.AxisConfig{}) },
	"divider":         func() facet.FacetImpl { return structure.NewDivider() },
	"list":            func() facet.FacetImpl { return structure.NewList("list", nil) },
	"row":             func() facet.FacetImpl { return structure.NewRow(nil, structure.AxisConfig{}) },
	"scroll_region":   func() facet.FacetImpl { return structure.NewScrollRegion("scroll") },
	"table":           func() facet.FacetImpl { return structure.NewTable("table", structure.TableData{}, nil) },
	"area":            func() facet.FacetImpl { return viz.NewArea[int](nil, nil, nil, nil, nil) },
	"axis":            func() facet.FacetImpl { return viz.NewAxis(nil, marks.Const(viz.AxisBottom), nil) },
	"bar":             func() facet.FacetImpl { return viz.NewBar[int](nil, nil, nil, nil) },
	"line":            func() facet.FacetImpl { return viz.NewLine[int](nil, nil, nil, nil, nil) },
	"point":           func() facet.FacetImpl { return viz.NewPoint[int](nil, nil, nil, nil, nil) },
	"rule":            func() facet.FacetImpl { return viz.NewRule(marks.Const(float64(0)), viz.RuleHorizontal, nil) },
}

// expectedMarkCount mirrors the coverage catalog's standardMarks count
// (demos/lurpic_studio/studio/coverage_test.go). When a mark joins the
// collection, both lists grow together — a mismatch here means the
// conformance walk is silently skipping a mark.
const expectedMarkCount = 52

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
			testkit.AssertBindingsAlive(t, mk)
		})
	}
}
