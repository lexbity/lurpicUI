package testkit

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

func PointerMove(x, y float32) platform.EventPointer {
	return platform.EventPointer{Kind: platform.PointerMove, Position: gfx.Point{X: x, Y: y}}
}

func PointerPress(x, y float32, button platform.PointerButton) platform.EventPointer {
	return platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: x, Y: y}, Button: button}
}

func PointerRelease(x, y float32, button platform.PointerButton) platform.EventPointer {
	return platform.EventPointer{Kind: platform.PointerRelease, Position: gfx.Point{X: x, Y: y}, Button: button}
}

func LeftClick(x, y float32) []platform.Event {
	return []platform.Event{
		PointerPress(x, y, platform.PointerLeft),
		PointerRelease(x, y, platform.PointerLeft),
	}
}

func Drag(fromX, fromY, toX, toY float32) []platform.Event {
	events := []platform.Event{
		PointerPress(fromX, fromY, platform.PointerLeft),
	}
	for i := 1; i <= 5; i++ {
		t := float32(i) / 6
		events = append(events, PointerMove(fromX+(toX-fromX)*t, fromY+(toY-fromY)*t))
	}
	events = append(events, PointerRelease(toX, toY, platform.PointerLeft))
	return events
}

func Scroll(x, y, deltaX, deltaY float32) platform.EventScroll {
	return platform.EventScroll{Position: gfx.Point{X: x, Y: y}, DeltaX: deltaX, DeltaY: deltaY}
}

func KeyPress(key platform.Key, mods platform.ModifierKeys) platform.EventKey {
	return platform.EventKey{Kind: platform.KeyPress, Key: key, Modifiers: mods}
}

func KeyRelease(key platform.Key, mods platform.ModifierKeys) platform.EventKey {
	return platform.EventKey{Kind: platform.KeyRelease, Key: key, Modifiers: mods}
}

func TypeText(s string) platform.EventText {
	return platform.EventText{Text: s}
}

func Chord(key platform.Key, mod platform.ModifierKeys) []platform.Event {
	return []platform.Event{
		KeyPress(key, mod),
		KeyRelease(key, mod),
	}
}
