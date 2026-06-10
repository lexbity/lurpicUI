package studio

import (
	"fmt"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/status"
)

type StatusBar struct {
	facet.Facet
	layout      facet.LayoutRole
	tick        facet.TickRole
	appState    *state.AppState

	progressBar *status.ProgressBar
	progressRing *status.ProgressRing
	statusLight *status.StatusLight
	badge       *status.Badge
	text        *primitive.Text

	reloading bool
}

func NewStatusBar(appState *state.AppState) *StatusBar {
	sb := &StatusBar{appState: appState}
	sb.Facet = facet.NewFacet()

	sb.progressBar = status.NewProgressBar("Progress")
	sb.progressRing = status.NewProgressRing("Progress")
	sb.statusLight = status.NewStatusLight("Connection")
	sb.badge = status.NewBadge("0")
	sb.text = primitive.NewText(marks.Const("Ready"))

	sb.Facet.AddChild(sb.progressBar.Base())
	sb.Facet.AddChild(sb.progressRing.Base())
	sb.Facet.AddChild(sb.statusLight.Base())
	sb.Facet.AddChild(sb.badge.Base())
	sb.Facet.AddChild(sb.text.Base())

	sb.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			width := constraints.MaxSize.W
			height := constraints.MaxSize.H
			if width <= 0 { width = 1280 }
			if height <= 0 { height = 32 }

			sb.statusLight.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 120, H: height}})
			sb.text.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 120, H: height}})
			sb.badge.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 60, H: height}})

			var progH float32 = height
			sb.progressBar.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 160, H: progH}})
			sb.progressRing.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: progH, H: progH}})

			return facet.MeasureResult{Size: gfx.Size{W: width, H: height}}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			sb.doArrange(bounds)
		},
	}
	sb.AddRole(&sb.layout)

	sb.tick = facet.TickRole{
		OnTick: func(dt time.Duration) {
			sb.onTick()
		},
	}
	sb.AddRole(&sb.tick)
	return sb
}

func (sb *StatusBar) doArrange(bounds gfx.Rect) {
	w := bounds.Width()
	h := bounds.Height()

	cx := bounds.Min.X + 8
	sb.statusLight.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(cx, bounds.Min.Y, 120, h)
	cx += 124

	sb.progressBar.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(cx, bounds.Min.Y+4, 160, h-8)
	cx += 164

	sb.progressRing.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(cx, bounds.Min.Y, h, h)
	cx += h + 8

	sb.badge.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(cx, bounds.Min.Y, 60, h)
	cx += 64

	sb.text.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(cx, bounds.Min.Y, w-cx-8, h)
}

func (sb *StatusBar) Base() *facet.Facet { sb.Facet.BindImpl(sb); return &sb.Facet }
func (sb *StatusBar) OnAttach(ctx facet.AttachContext)  {}
func (sb *StatusBar) OnDetach()                         {}
func (sb *StatusBar) OnActivate()                       {}
func (sb *StatusBar) OnDeactivate()                     {}

func (sb *StatusBar) onTick() {
	if !sb.reloading {
		return
	}
	progress := sb.appState.JobProgress.Get()
	progress += 0.05
	if progress >= 1.0 {
		progress = 1.0
		sb.appState.JobProgress.Set(progress)
		sb.appState.Connection.Set(state.ConnConnected)
		sb.reloading = false
		badgeText := rowCountStr(sb.appState)
		sb.badge.Label = marks.Const(badgeText)
		sb.badge.Base().Invalidate(facet.DirtyProjection)
		sb.text.Content = marks.Const("Ready")
		sb.text.Base().Invalidate(facet.DirtyProjection)
		return
	}
	sb.appState.JobProgress.Set(progress)
}

func (sb *StatusBar) StartReload() {
	sb.appState.JobProgress.Set(0)
	sb.appState.Connection.Set(state.ConnConnecting)
	sb.reloading = true
	sb.text.Content = marks.Const("Reloading...")
	sb.text.Base().Invalidate(facet.DirtyProjection)
}

func (sb *StatusBar) CancelReload() {
	sb.reloading = false
	sb.appState.JobProgress.Set(0)
	sb.appState.Connection.Set(state.ConnDisconnected)
	sb.text.Content = marks.Const("Cancelled")
	sb.text.Base().Invalidate(facet.DirtyProjection)
}

func (sb *StatusBar) IsReloading() bool { return sb.reloading }

func (sb *StatusBar) UpdateBadge(appState *state.AppState) {
	count := 0
	sel := appState.SelectedSource.Get()
	for _, r := range appState.Rows.All() {
		if sel == "" || r.Region == sel {
			count++
		}
	}
	badgeText := formatCount(count)
	sb.badge.Label = marks.Const(badgeText)
	sb.badge.Base().Invalidate(facet.DirtyProjection)
}

func (sb *StatusBar) ProgressBar() *status.ProgressBar   { return sb.progressBar }
func (sb *StatusBar) ProgressRing() *status.ProgressRing { return sb.progressRing }
func (sb *StatusBar) StatusLight() *status.StatusLight   { return sb.statusLight }
func (sb *StatusBar) Badge() *status.Badge               { return sb.badge }
func (sb *StatusBar) Text() *primitive.Text               { return sb.text }

func rowCountStr(appState *state.AppState) string {
	return fmt.Sprintf("%d", len(appState.Rows.All()))
}

func formatCount(n int) string {
	return fmt.Sprintf("%d", n)
}
