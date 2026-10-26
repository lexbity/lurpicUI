package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// VizProbe is the isolated prove-viz-first chart (Slice P3), now a thin
// wrapper over the production ChartCanvas: it seeds its own stores from the
// CSV rows and hands them to the canvas. The P3 tests exercise the same wiring
// the gallery's chart uses.
type VizProbe struct {
	*ChartCanvas
}

// NewVizProbe builds the standalone chart over the given seed rows (Slice P3).
func NewVizProbe(scope *store.Scope, seed []dataset.Row, service *text.Service, themeCtx theme.ResolvedContext) *VizProbe {
	rows := store.NewCollectionStore(scope, vizRowID)
	for i := range seed {
		seed[i].ID = uint64(i + 1)
		rows.Insert(seed[i])
	}
	xExtent := reactive.DomainFromCollection(scope, rows, vizRowTime)
	yDomain := reactive.DomainFromCollection(scope, rows, vizRowValue)
	canvas := NewChartCanvas(scope, ChartConfig{
		TextService: service,
		Theme:       themeCtx,
		Rows:        rows,
		XDomain:     store.NewValueStore(scope, xExtent.Get()),
		XRange:      store.NewValueStore(scope, [2]float64{0, 1}),
		YDomain:     yDomain,
		YRange:      store.NewValueStore(scope, [2]float64{1, 0}),
		RuleValue:   store.NewValueStore(scope, meanValue(seed)),
	})
	return &VizProbe{ChartCanvas: canvas}
}

func meanValue(rows []dataset.Row) float64 {
	if len(rows) == 0 {
		return 0
	}
	sum := 0.0
	for _, r := range rows {
		sum += r.Value
	}
	return sum / float64(len(rows))
}
