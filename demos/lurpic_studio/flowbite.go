package main

import (
	"fmt"
	"io"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/gfx"
	. "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/runtime"
)

type flowbiteResolver struct{}

func newFlowbiteResolver() *flowbiteResolver {
	return &flowbiteResolver{}
}

func (r *flowbiteResolver) ResolveIcon(ref string) (runtime.IconAsset, bool) {
	data, err := app.Asset("icons/" + ref + ".svg")
	if err != nil {
		return runtime.IconAsset{}, false
	}

	doc, err := ParseSVG(data)
	if err != nil {
		return runtime.IconAsset{}, false
	}

	var allSegs []gfx.PathSegment
	for _, el := range doc.Elements {
		allSegs = append(allSegs, el.Path.Segments...)
	}
	if len(allSegs) == 0 {
		return runtime.IconAsset{}, false
	}

	vb := gfx.Rect{
		Min: gfx.Point{X: doc.ViewBox.Min.X, Y: doc.ViewBox.Min.Y},
		Max: gfx.Point{X: doc.ViewBox.Max.X, Y: doc.ViewBox.Max.Y},
	}
	path := gfx.Path{Segments: allSegs}
	return runtime.NewIconAsset(ref, 1, path, vb), true
}

var _ io.Reader = (*io.LimitedReader)(nil)
var _ = fmt.Sprintf("")
