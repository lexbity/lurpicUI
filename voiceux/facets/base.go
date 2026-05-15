package facets

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/voiceux"
)

type baseVoiceFacet struct {
	facet.Facet
	service voiceux.VoiceService
	stores  *voiceux.VoiceStores
	theme   voiceux.ThemeSlots
	bounds  gfx.Rect
	lastErr string
}

func newBaseVoiceFacet(service voiceux.VoiceService) baseVoiceFacet {
	var stores *voiceux.VoiceStores
	if service != nil {
		stores = service.Stores()
	}
	return baseVoiceFacet{
		Facet:   facet.NewFacet(),
		service: service,
		stores:  stores,
		theme:   voiceux.Theme(),
	}
}

func (f *baseVoiceFacet) Base() *facet.Facet {
	if f == nil {
		return nil
	}
	return &f.Facet
}

func (f *baseVoiceFacet) setBounds(bounds gfx.Rect) {
	if f == nil {
		return
	}
	f.bounds = bounds
}

func (f *baseVoiceFacet) Bounds() gfx.Rect {
	if f == nil {
		return gfx.Rect{}
	}
	return f.bounds
}

func (f *baseVoiceFacet) dispatch(cmd voiceux.VoiceCommand) error {
	if f == nil || f.service == nil {
		return nil
	}
	err := f.service.DispatchVoiceCommand(cmd)
	if err != nil {
		f.lastErr = err.Error()
	}
	return err
}

func (f *baseVoiceFacet) dispatchAction(actionID string, args map[string]any) error {
	if f == nil || f.service == nil {
		return nil
	}
	err := f.service.DispatchAction(actionID, args)
	if err != nil {
		f.lastErr = err.Error()
	}
	return err
}

func (f *baseVoiceFacet) LastError() string {
	if f == nil {
		return ""
	}
	return f.lastErr
}

func trackChange[T any](f *facet.Facet, sig *signal.Signal[signal.Change[T]], flags facet.DirtyFlags, source string) {
	if f == nil || sig == nil {
		return
	}
	signal.Track(f.Subs(), sig, func(signal.Change[T]) {
		f.InvalidateWithSource(flags, source)
	})
}

func trackReplace[T any](f *facet.Facet, col *store.CollectionStore[T], flags facet.DirtyFlags, source string) {
	if f == nil || col == nil {
		return
	}
	f.Subs().Add(col.OnReplaceSubscribe(func(signal.Unit) {
		f.InvalidateWithSource(flags, source)
	}))
}

func addLayoutRenderRoles(f *facet.Facet, sizeW, sizeH float32) {
	f.AddRole(&facet.LayoutRole{
		OnMeasure: func(facet.Constraints) gfx.Size {
			return gfx.Size{W: sizeW, H: sizeH}
		},
	})
	f.AddRole(&facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {},
	})
}
