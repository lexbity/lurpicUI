package scenes

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

type StoreSignalScene struct {
	BaseScene
	count     store.Binding[int]
	activity  signal.Signal[string]
	lastEvent string
	emits     int
	button    *basic.Rect
	status    *basic.Text
	th        theme.Context
}

func NewStoreSignalScene() *StoreSignalScene {
	s := &StoreSignalScene{
		BaseScene: NewBaseScene(
			"store-signal",
			"Store / Signal",
			"Validates store invalidation and signal fanout behavior",
			[]string{"basic"},
		),
		th: theme.Default(),
	}
	s.capability.HasCustomLogs = true
	s.count = store.NewBinding(0)
	s.activity = signal.NewSignal[string]("store-signal")
	return s
}

func (s *StoreSignalScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}
	col := layout.NewColumnLayout()
	col.Gap = 12
	s.root = col

	title := newTextMark("store-title", "Store and signal fanout", 18)
	col.AddChild(title.Base())

	s.button = newActionRect("store-button", s.th.Color(theme.ColorPrimary), func() {
		s.count.Set(s.count.Get() + 1)
		s.emits++
		s.lastEvent = fmt.Sprintf("store changed to %d", s.count.Get())
		s.activity.Emit(s.lastEvent)
		s.refreshStatus()
	})
	col.AddChild(s.button.Base())

	s.status = newTextMark("store-status", "", 13)
	col.AddChild(s.status.Base())
	s.refreshStatus()
	return col
}

func (s *StoreSignalScene) refreshStatus() {
	if s.status == nil {
		return
	}
	updateTextValue(s.status, fmt.Sprintf("Count=%d Emits=%d Last=%s", s.count.Get(), s.emits, s.lastEvent), 13)
}

func (s *StoreSignalScene) ApplyTheme(th theme.Context) {
	s.th = th
	if s.button != nil {
		tintRectStyle(s.button, th.Color(theme.ColorPrimary))
	}
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyProjection)
	}
}

func (s *StoreSignalScene) ApplyDensity(scale float32) {}

func (s *StoreSignalScene) Reset() {
	s.count.Set(0)
	s.emits = 0
	s.lastEvent = ""
	s.refreshStatus()
	s.BaseScene.Reset()
}

func (s *StoreSignalScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id": s.id,
		"count":    s.count.Get(),
		"emits":    s.emits,
		"last":     s.lastEvent,
	}
}

func (s *StoreSignalScene) ImportState(state map[string]any) {
	if v, ok := state["count"].(float64); ok {
		s.count.Set(int(v))
	}
	if v, ok := state["emits"].(float64); ok {
		s.emits = int(v)
	}
	if v, ok := state["last"].(string); ok {
		s.lastEvent = v
	}
	s.refreshStatus()
}

var _ scene.Scene = (*StoreSignalScene)(nil)
