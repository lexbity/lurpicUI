package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// playSelectFamily is the Selection playground: checkbox, switch, slider,
// turn_dial, radio_group, dropdown_select, button_group, list_item. Each mark
// binds a ValueStore that its own pointer/key interaction writes (the
// selection family's distinctive behavior: exclusive vs multiple store
// write-back).
type playSelectFamily struct {
	scroll *demoList

	checkbox *selection.Checkbox

	toggle *selection.Switch
	tgl    *store.ValueStore[bool]

	slider    *selection.Slider
	sliderVal *store.ValueStore[float64]

	dial    *selection.TurnDial
	dialVal *store.ValueStore[float64]

	radio    *selection.RadioGroup
	radioVal *store.ValueStore[string]

	dropdown *selection.DropdownSelect
	ddVal    *store.ValueStore[string]

	segments *selection.ButtonGroup
	groupVal *store.ValueStore[[]string]

	item         *selection.ListItem
	itemSelected *store.ValueStore[bool]
	itemHits     *store.ValueStore[int]
}

// newPlaySelectFamily builds the Selection family playground.
func newPlaySelectFamily() *playSelectFamily {
	f := &playSelectFamily{
		tgl:       store.NewValueStore(false),
		sliderVal: store.NewValueStore(60.0),
		dialVal:   store.NewValueStore(40.0),
		radioVal:  store.NewValueStore("revenue"),
		ddVal:     store.NewValueStore("day"),
		groupVal:  store.NewValueStore([]string{"day"}),
	}

	f.checkbox = selection.NewCheckbox("Show grid", store.NewValueStore(selection.CheckboxStateOff))
	f.toggle = selection.NewSwitch("Live updates", f.tgl)
	f.slider = selection.NewSlider("Opacity", 0, 100, 5, f.sliderVal)
	f.dial = selection.NewTurnDial("Smoothing", 0, 100, 1, f.dialVal)
	f.radio = selection.NewRadioGroup("Chart type", []selection.RadioOption{
		{Value: "replay", Label: "Rolling"},
		{Value: "hist", Label: "Histogram"},
		{Value: "stack", Label: "Stacked"},
	}, f.radioVal)
	f.dropdown = selection.NewDropdownSelect("Aggregation", []selection.DropdownOption{
		{Value: "day", Label: "Daily"},
		{Value: "week", Label: "Weekly"},
		{Value: "month", Label: "Monthly"},
	}, f.ddVal)
	f.segments = selection.NewButtonGroup("Time range", []selection.ButtonGroupOption{
		{Key: "day", Label: "1D"},
		{Key: "week", Label: "1W"},
		{Key: "month", Label: "1M"},
	}, f.groupVal)
	f.segments.Mode = marks.Const(selection.ButtonGroupExclusive)

	f.itemSelected = store.NewValueStore(false)
	f.itemHits = store.NewValueStore(0)
	f.item = selection.NewListItem(marks.Const("Selected source row"))
	f.item.ShowSelectionIndicator = marks.Const(true)
	f.item.Selected = marks.FromStore(f.itemSelected, facet.DirtyProjection)

	f.scroll = newDemoList(listGap,
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

// Checkbox returns the checkbox state store.
func (f *playSelectFamily) CheckboxState() *store.ValueStore[selection.CheckboxState] {
	return f.checkbox.Value
}

// Toggle returns the switch's store.
func (f *playSelectFamily) Toggle() *store.ValueStore[bool] { return f.tgl }

// SliderValue returns the slider's store.
func (f *playSelectFamily) Slider() *store.ValueStore[float64] { return f.sliderVal }

// Dial returns the turn_dial's store.
func (f *playSelectFamily) Dial() *store.ValueStore[float64] { return f.dialVal }

// Radio returns the radio_group's store.
func (f *playSelectFamily) Radio() *store.ValueStore[string] { return f.radioVal }

// Dropdown returns the dropdown store.
func (f *playSelectFamily) Dropdown() *store.ValueStore[string] { return f.ddVal }

// ButtonGroup returns the button_group's store.
func (f *playSelectFamily) ButtonGroup() *store.ValueStore[[]string] { return f.groupVal }

// ItemSelected returns the list_item's selection store.
func (f *playSelectFamily) ItemSelected() *store.ValueStore[bool] { return f.itemSelected }

// ItemHits returns the list_item's activation count.
func (f *playSelectFamily) ItemHits() *store.ValueStore[int] { return f.itemHits }

// Item returns the list_item mark.
func (f *playSelectFamily) Item() *selection.ListItem { return f.item }
