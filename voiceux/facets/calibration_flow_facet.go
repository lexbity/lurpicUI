package facets

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/lurpicui/voiceux/input"
	"codeburg.org/lexbit/voicedsp"
)

type CalibrationFlowFacet struct {
	baseVoiceFacet
	stepIndex int
}

func NewCalibrationFlowFacet(service voiceux.VoiceService) *CalibrationFlowFacet {
	f := &CalibrationFlowFacet{baseVoiceFacet: newBaseVoiceFacet(service)}
	f.AddRole(&facet.LayoutRole{
		OnMeasure: func(facet.Constraints) gfx.Size { return gfx.Size{W: 420, H: 320} },
		OnArrange: func(bounds gfx.Rect) { f.setBounds(bounds) },
	})
	f.AddRole(&facet.RenderRole{OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
		panel := f.Projection(bounds)
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x161B22FF))})
		list.Add(gfx.FillRect{Rect: panel.ProgressRect, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x355070FF))})
		list.Add(gfx.FillRect{Rect: panel.AcceptRect, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x588157FF))})
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
	f.AddRole(&facet.TickRole{})
	return f
}

func (f *CalibrationFlowFacet) OnAttach(ctx facet.AttachContext) {
	if f.stores == nil {
		return
	}
	trackChange(&f.Facet, &f.stores.Calibration.OnChange, facet.DirtyAll, "voiceux.calibration.session")
	trackChange(&f.Facet, &f.stores.Params.OnChange, facet.DirtyProjection, "voiceux.calibration.params")
	trackReplace(&f.Facet, f.stores.Diagnostics, facet.DirtyProjection, "voiceux.calibration.diagnostics")
}

func (f *CalibrationFlowFacet) OnDetach()     {}
func (f *CalibrationFlowFacet) OnActivate()   {}
func (f *CalibrationFlowFacet) OnDeactivate() {}

func (f *CalibrationFlowFacet) Projection(bounds gfx.Rect) CalibrationPanel {
	state := f.calibrationState()
	steps := input.DefaultCalibrationSteps()
	if len(steps) == 0 {
		return CalibrationPanel{Bounds: bounds}
	}
	step := steps[f.stepIndex%len(steps)]
	progress := float32(state.Progress.Completed)
	total := float32(maxInt(1, state.Progress.Total))
	progressWidth := bounds.Width() * clamp01(progress/total)
	if progressWidth < 24 {
		progressWidth = 24
	}
	return CalibrationPanel{
		Bounds:       bounds,
		Title:        step.Label,
		Step:         step,
		ProgressRect: gfx.RectFromXYWH(bounds.Min.X+12, bounds.Max.Y-28, progressWidth-24, 16),
		AcceptRect:   gfx.RectFromXYWH(bounds.Max.X-104, bounds.Max.Y-36, 92, 24),
		CanCommit:    state.CanCommit,
		Quality:      state.Progress.Quality,
		Message:      state.Progress.Message,
	}
}

func (f *CalibrationFlowFacet) Start() error {
	return f.dispatch(voiceux.StartCalibrationCommand{Config: f.calibrationConfigValue()})
}

func (f *CalibrationFlowFacet) Cancel() error {
	return f.dispatch(voiceux.CancelCalibrationCommand{})
}

func (f *CalibrationFlowFacet) Commit() error {
	return f.dispatch(voiceux.CommitCalibrationCommand{Calibration: voicedsp.VowelCalibration{
		SpeakerID: "speaker",
		Method:    voicedsp.NormalizationLobanov,
	}})
}

func (f *CalibrationFlowFacet) Advance() {
	steps := input.DefaultCalibrationSteps()
	if len(steps) == 0 {
		return
	}
	f.stepIndex = (f.stepIndex + 1) % len(steps)
	f.Invalidate(facet.DirtyProjection)
}

func (f *CalibrationFlowFacet) calibrationState() voiceux.CalibrationStateView {
	if f == nil || f.stores == nil || f.stores.Calibration == nil {
		return voiceux.CalibrationStateView{}
	}
	return f.stores.Calibration.Get()
}

func (f *CalibrationFlowFacet) calibrationConfigValue() voicedsp.CalibrationConfig {
	steps := input.DefaultCalibrationSteps()
	vowels := make([]voicedsp.Vowel, 0, len(steps))
	for _, step := range steps {
		vowels = append(vowels, step.Vowel)
	}
	return voicedsp.CalibrationConfig{
		SessionID:        fmt.Sprintf("session-%d", f.stepIndex),
		SpeakerID:        "speaker",
		DurationMS:       1500,
		MinStableSamples: 12,
		Vowels:           vowels,
		Method:           voicedsp.NormalizationLobanov,
	}
}

// CalibrationPanel is the projection snapshot used by the calibration flow facet.
type CalibrationPanel struct {
	Bounds       gfx.Rect
	Title        string
	Step         input.CalibrationStep
	ProgressRect gfx.Rect
	AcceptRect   gfx.Rect
	CanCommit    bool
	Quality      voicedsp.CalibrationQuality
	Message      string
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
