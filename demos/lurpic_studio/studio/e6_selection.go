package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// playSelectFamily is the Selection playground: checkbox, switch, slider,
// turn_dial, radio_group, dropdown_select, button_group, list_item. Every
// value mark owns its truth (RX-2 Q2) — the family holds no ceremony stores;
// the accessors expose each mark's live store for the inspector and tests.
type playSelectFamily struct {
	scroll *structure.ScrollRegion

	checkbox *selection.Checkbox

	toggle *selection.Switch

	slider *selection.Slider

	dial *selection.TurnDial

	radio *selection.RadioGroup

	dropdown *selection.DropdownSelect

	segments *selection.ButtonGroup

	item         *selection.ListItem
	itemSelected *store.ValueStore[bool]
	itemHits     *store.ValueStore[int]
}

// newPlaySelectFamily builds the Selection family playground.
func newPlaySelectFamily() *playSelectFamily {
	f := &playSelectFamily{}

	f.checkbox = selection.NewCheckbox("Show grid", nil)
	f.toggle = selection.NewSwitch("Live updates", nil)
	f.slider = selection.NewSlider("Opacity", 0, 100, 5, nil)
	f.dial = selection.NewTurnDial("Smoothing", 0, 100, 1, nil)
	f.radio = selection.NewRadioGroup("Chart type", []selection.RadioOption{
		{Value: "replay", Label: "Rolling"},
		{Value: "hist", Label: "Histogram"},
		{Value: "stack", Label: "Stacked"},
	}, nil)
	f.dropdown = selection.NewDropdownSelect("Aggregation", []selection.DropdownOption{
		{Value: "day", Label: "Daily"},
		{Value: "week", Label: "Weekly"},
		{Value: "month", Label: "Monthly"},
	}, nil)
	f.segments = selection.NewButtonGroup("Time range", []selection.ButtonGroupOption{
		{Key: "day", Label: "1D"},
		{Key: "week", Label: "1W"},
		{Key: "month", Label: "1M"},
	}, nil)
	f.segments.Mode = marks.Const(selection.ButtonGroupExclusive)

	f.itemSelected = store.NewValueStore(false)
	f.itemHits = store.NewValueStore(0)
	f.item = selection.NewListItem(marks.Const("Selected source row"))
	f.item.ShowSelectionIndicator = marks.Const(true)
	f.item.Selected = marks.FromStore(f.itemSelected, facet.DirtyProjection)

	f.scroll = newPlayScroll(listGap,
		playgroundCard("checkbox — toggle grid", f.checkbox),
		playgroundCard("switch — toggle live", f.toggle),
		playgroundCard("slider — drag opacity", f.slider),
		playgroundCard("turn_dial — drag smoothing", f.dial),
		playgroundCard("radio_group — choose chart type", f.radio),
		playgroundCard("dropdown_select — choose aggregation", f.dropdown),
		playgroundCard("button_group — choose range", f.segments),
		playgroundCard("list_item — click to select", f.item),
	)
	return f
}

// wire subscribes the list_item's activation (a click toggles its selection —
// the mark itself has no store write-back, so the family owns the loop).
func (f *playSelectFamily) wire() func() {
	if f.item == nil {
		return nil
	}
	itemID := f.item.Activated.Subscribe(func(signal.Unit) {
		f.itemHits.Set(f.itemHits.Get() + 1)
		f.itemSelected.Set(!f.itemSelected.Get())
	})
	return func() { f.item.Activated.Unsubscribe(itemID) }
}

// CheckboxState returns the checkbox's live store.
func (f *playSelectFamily) CheckboxState() *store.ValueStore[selection.CheckboxState] {
	return f.checkbox.Store()
}

// Toggle returns the switch's live store.
func (f *playSelectFamily) Toggle() *store.ValueStore[bool] { return f.toggle.Store() }

// SliderValue returns the slider's live store.
func (f *playSelectFamily) Slider() *store.ValueStore[float64] { return f.slider.Store() }

// Dial returns the turn_dial's live store.
func (f *playSelectFamily) Dial() *store.ValueStore[float64] { return f.dial.Store() }

// Radio returns the radio_group's live store.
func (f *playSelectFamily) Radio() *store.ValueStore[string] { return f.radio.Store() }

// Dropdown returns the dropdown's live store.
func (f *playSelectFamily) Dropdown() *store.ValueStore[string] { return f.dropdown.Store() }

// ButtonGroup returns the button_group's live store.
func (f *playSelectFamily) ButtonGroup() *store.ValueStore[[]string] { return f.segments.Store() }

// ItemSelected returns the list_item's selection store.
func (f *playSelectFamily) ItemSelected() *store.ValueStore[bool] { return f.itemSelected }

// ItemHits returns the list_item's activation count.
func (f *playSelectFamily) ItemHits() *store.ValueStore[int] { return f.itemHits }

// Item returns the list_item mark.
func (f *playSelectFamily) Item() *selection.ListItem { return f.item }
