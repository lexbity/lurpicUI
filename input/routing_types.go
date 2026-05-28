package input

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

// Pointer events.
type PointerPressEvent struct {
	Position, ScreenPos gfx.Point
	Button              platform.PointerButton
	Modifiers           platform.ModifierKeys
	MarkID              facet.MarkID
	ClickCount          int
}

type PointerMoveEvent struct {
	Position, ScreenPos gfx.Point
	Modifiers           platform.ModifierKeys
	MarkID              facet.MarkID
}

type PointerReleaseEvent struct {
	Position, ScreenPos gfx.Point
	Button              platform.PointerButton
	Modifiers           platform.ModifierKeys
	MarkID              facet.MarkID
}

type PointerEnterEvent struct {
	Position, ScreenPos gfx.Point
	MarkID              facet.MarkID
}

type PointerLeaveEvent struct {
	MarkID facet.MarkID
}

// Gesture events.
type ClickEvent struct {
	Position, ScreenPos gfx.Point
	Button              platform.PointerButton
	Modifiers           platform.ModifierKeys
	MarkID              facet.MarkID
	ClickCount          int
}

type DragStartEvent struct {
	StartPosition, ScreenStart gfx.Point
	Button                     platform.PointerButton
	Modifiers                  platform.ModifierKeys
	MarkID                     facet.MarkID
}

type DragMoveEvent struct {
	Position, ScreenPos gfx.Point
	Delta, ScreenDelta  gfx.Point
	Button              platform.PointerButton
	Modifiers           platform.ModifierKeys
	MarkID              facet.MarkID
}

type DragEndEvent struct {
	Position, ScreenPos gfx.Point
	Button              platform.PointerButton
	Modifiers           platform.ModifierKeys
	MarkID              facet.MarkID
}

type ScrollEvent struct {
	Position  gfx.Point
	DeltaX    float32
	DeltaY    float32
	Precise   bool
	Modifiers platform.ModifierKeys
}

type KeyInputEvent struct {
	Kind      platform.KeyEventKind
	Key       platform.Key
	Modifiers platform.ModifierKeys
}

type TextInputEvent struct {
	Text      string
	Composing bool
}

type DismissEvent struct {
	Trigger    facet.DismissalTrigger
	ScreenPos  gfx.Point
	HitFacetID facet.FacetID
	HitMarkID  facet.MarkID
	HitLayerID facet.LayerID
	HitOrder   int
}

type resolvedHitTarget struct {
	FacetID facet.FacetID
	MarkID  facet.MarkID
	Local   gfx.Point
}

type TouchInputEvent struct {
	Event facet.TouchEvent
}

type FocusGainedEvent struct{}
type FocusLostEvent struct{}

func (PointerPressEvent) isDeliveredEvent()   {}
func (PointerMoveEvent) isDeliveredEvent()    {}
func (PointerReleaseEvent) isDeliveredEvent() {}
func (PointerEnterEvent) isDeliveredEvent()   {}
func (PointerLeaveEvent) isDeliveredEvent()   {}
func (ScrollEvent) isDeliveredEvent()         {}
func (KeyInputEvent) isDeliveredEvent()       {}
func (TextInputEvent) isDeliveredEvent()      {}
func (DismissEvent) isDeliveredEvent()        {}
func (TouchInputEvent) isDeliveredEvent()     {}
func (ClickEvent) isDeliveredEvent()          {}
func (DragStartEvent) isDeliveredEvent()      {}
func (DragMoveEvent) isDeliveredEvent()       {}
func (DragEndEvent) isDeliveredEvent()        {}
func (FocusGainedEvent) isDeliveredEvent()    {}
func (FocusLostEvent) isDeliveredEvent()      {}
