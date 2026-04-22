package uinotification

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/store"
)

func TestPhase5DialogSnackbarProgressGeometry(t *testing.T) {
	dialog := &Dialog{}
	if got := dialog.bounds(); got.Width() != 420 || got.Height() != 280 {
		t.Fatalf("dialog bounds = %#v", got)
	}
	if got := dialog.surfaceBounds(); got.Width() != 372 || got.Height() != 232 {
		t.Fatalf("dialog surface bounds = %#v", got)
	}
	if got := (&Dialog{Variant: DialogFullscreen}).surfaceBounds(); got.Width() != 640 || got.Height() != 480 {
		t.Fatalf("fullscreen dialog surface bounds = %#v", got)
	}

	snackbar := &Snackbar{}
	if got := snackbar.bounds(); got.Width() != 320 || got.Height() != 56 {
		t.Fatalf("snackbar bounds = %#v", got)
	}
	snackbar.Action = &ButtonAction{Label: "Run", OnClick: func() {}}
	if got := snackbar.bounds(); got.Width() != 408 || got.Height() != 56 {
		t.Fatalf("snackbar action bounds = %#v", got)
	}
	if got := snackbar.actionBounds(); got.Width() != 88 || got.Height() != 56 {
		t.Fatalf("snackbar action hit bounds = %#v", got)
	}

	progressLinear := &Progress{Mode: ProgressDeterminate, Value: store.NewBinding(0.5)}
	if got := progressLinear.bounds(); got.Width() != 240 || got.Height() != 12 {
		t.Fatalf("linear progress bounds = %#v", got)
	}
	progressCircular := &Progress{Mode: ProgressDeterminate, Shape: ProgressCircular, Value: store.NewBinding(0.5)}
	if got := progressCircular.bounds(); got.Width() != 48 || got.Height() != 48 {
		t.Fatalf("circular progress bounds = %#v", got)
	}
}
