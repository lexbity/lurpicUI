package facets

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/voicedsp"
)

type DeviceSelectorFacet struct {
	baseVoiceFacet
}

func NewDeviceSelectorFacet(service voiceux.VoiceService) *DeviceSelectorFacet {
	f := &DeviceSelectorFacet{baseVoiceFacet: newBaseVoiceFacet(service)}
	f.AddRole(&facet.LayoutRole{
		OnMeasure: func(facet.Constraints) gfx.Size { return gfx.Size{W: 320, H: 280} },
		OnArrange: func(bounds gfx.Rect) { f.setBounds(bounds) },
	})
	f.AddRole(&facet.RenderRole{OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
		ds := f.deviceSnapshot()
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x121821FF))})
		list.Add(gfx.FillRect{Rect: leftCard(bounds, 0), Brush: gfx.SolidBrush(gfx.ColorFromHex(0x1E2630FF))})
		list.Add(gfx.FillRect{Rect: leftCard(bounds, 1), Brush: gfx.SolidBrush(gfx.ColorFromHex(0x1E2630FF))})
		list.Add(gfx.FillRect{Rect: leftCard(bounds, 2), Brush: gfx.SolidBrush(gfx.ColorFromHex(0x1E2630FF))})
		_ = ds
	}})
	f.AddRole(&facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
		if f.Bounds().Contains(p) {
			return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
		}
		return facet.HitResult{}
	}})
	f.AddRole(&facet.InputRole{OnPointer: func(e facet.PointerEvent) bool {
		return f.Bounds().Contains(e.Position)
	}})
	f.AddRole(&facet.FocusRole{Focusable: func() bool { return true }})
	return f
}

func (f *DeviceSelectorFacet) OnAttach(ctx facet.AttachContext) {
	if f.stores == nil {
		return
	}
	trackChange(&f.Facet, &f.stores.Devices.OnChange, facet.DirtyAll, "voiceux.device_selector.devices")
	trackChange(&f.Facet, &f.stores.SelectedInput.OnChange, facet.DirtyProjection, "voiceux.device_selector.selected_input")
	trackChange(&f.Facet, &f.stores.SelectedOutput.OnChange, facet.DirtyProjection, "voiceux.device_selector.selected_output")
	trackChange(&f.Facet, &f.stores.SelectedMonitor.OnChange, facet.DirtyProjection, "voiceux.device_selector.selected_monitor")
}

func (f *DeviceSelectorFacet) OnDetach()     {}
func (f *DeviceSelectorFacet) OnActivate()   {}
func (f *DeviceSelectorFacet) OnDeactivate() {}

// RefreshDevices asks the host to republish the device snapshot.
func (f *DeviceSelectorFacet) RefreshDevices() error {
	return f.dispatchAction("refresh_devices", nil)
}

// SelectInput updates the selected input device.
func (f *DeviceSelectorFacet) SelectInput(id string) error {
	if f.stores != nil && f.stores.SelectedInput != nil {
		f.stores.SelectedInput.Set(voicedsp.DeviceID(id))
	}
	return f.dispatchAction("select_input", map[string]any{"id": id})
}

// SelectOutput updates the selected output device.
func (f *DeviceSelectorFacet) SelectOutput(id string) error {
	if f.stores != nil && f.stores.SelectedOutput != nil {
		f.stores.SelectedOutput.Set(voicedsp.DeviceID(id))
	}
	return f.dispatchAction("select_output", map[string]any{"id": id})
}

// SelectMonitor updates the selected monitor device.
func (f *DeviceSelectorFacet) SelectMonitor(id string) error {
	if f.stores != nil && f.stores.SelectedMonitor != nil {
		f.stores.SelectedMonitor.Set(voicedsp.DeviceID(id))
	}
	return f.dispatchAction("select_monitor", map[string]any{"id": id})
}

// DeviceSnapshot returns the current device view for tests.
func (f *DeviceSelectorFacet) Snapshot() voiceux.AudioDeviceSnapshot {
	return f.deviceSnapshot()
}

func (f *DeviceSelectorFacet) deviceSnapshot() voiceux.AudioDeviceSnapshot {
	if f == nil || f.stores == nil || f.stores.Devices == nil {
		return voiceux.AudioDeviceSnapshot{}
	}
	return f.stores.Devices.Get()
}

func leftCard(bounds gfx.Rect, index int) gfx.Rect {
	width := bounds.Width() / 3
	x := bounds.Min.X + width*float32(index)
	return gfx.RectFromXYWH(x+4, bounds.Min.Y+4, width-8, bounds.Height()-8)
}
