package gfx

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPathBuilder_build_empty(t *testing.T) {
	got := NewPath().Build()
	if len(got.Segments) != 0 {
		t.Fatalf("expected empty path, got %#v", got)
	}
}

func TestPathBuilder_moveto_lineto_close(t *testing.T) {
	got := NewPath().
		MoveTo(Point{1, 2}).
		LineTo(Point{3, 4}).
		Close().
		Build()

	if len(got.Segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(got.Segments))
	}
	if got.Segments[0].Verb != PathMoveTo || got.Segments[0].Pts[0] != (Point{1, 2}) {
		t.Fatalf("unexpected first segment: %+v", got.Segments[0])
	}
	if got.Segments[1].Verb != PathLineTo || got.Segments[1].Pts[0] != (Point{3, 4}) {
		t.Fatalf("unexpected second segment: %+v", got.Segments[1])
	}
	if got.Segments[2].Verb != PathClose {
		t.Fatalf("unexpected third segment: %+v", got.Segments[2])
	}
}

func TestRectPath_segment_count(t *testing.T) {
	got := RectPath(RectFromXYWH(0, 0, 10, 20))
	if len(got.Segments) != 5 {
		t.Fatalf("expected 5 segments, got %d", len(got.Segments))
	}
	if got.Segments[0].Verb != PathMoveTo || got.Segments[1].Verb != PathLineTo || got.Segments[2].Verb != PathLineTo || got.Segments[3].Verb != PathLineTo || got.Segments[4].Verb != PathClose {
		t.Fatalf("unexpected rect path verbs: %#v", got.Segments)
	}
}

func TestRoundedRectPath_corner_count(t *testing.T) {
	got := RoundedRectPath(RectFromXYWH(0, 0, 20, 10), 2)
	if len(got.Segments) == 0 {
		t.Fatal("expected non-empty rounded rect path")
	}
	if got.Segments[len(got.Segments)-1].Verb != PathClose {
		t.Fatalf("expected rounded rect to be closed, got last segment %#v", got.Segments[len(got.Segments)-1])
	}

	corners := 0
	for _, seg := range got.Segments {
		if seg.Verb == PathQuadTo || seg.Verb == PathCubicTo {
			corners++
		}
	}
	if corners != 4 {
		t.Fatalf("expected 4 curved corners, got %d (%#v)", corners, got.Segments)
	}
}

func TestCirclePath_is_closed(t *testing.T) {
	got := CirclePath(Point{5, 5}, 3)
	if len(got.Segments) == 0 {
		t.Fatal("expected non-empty circle path")
	}
	if got.Segments[len(got.Segments)-1].Verb != PathClose {
		t.Fatalf("expected circle path to be closed, got last segment %#v", got.Segments[len(got.Segments)-1])
	}
}

func TestPolylinePath_closed_and_open(t *testing.T) {
	pts := []Point{{0, 0}, {1, 1}, {2, 0}}

	open := PolylinePath(pts, false)
	if len(open.Segments) != 3 {
		t.Fatalf("expected open polyline to have 3 segments, got %d", len(open.Segments))
	}
	if open.Segments[len(open.Segments)-1].Verb == PathClose {
		t.Fatal("expected open polyline to remain open")
	}

	closed := PolylinePath(pts, true)
	if closed.Segments[len(closed.Segments)-1].Verb != PathClose {
		t.Fatalf("expected closed polyline to end with close, got %#v", closed.Segments[len(closed.Segments)-1])
	}
}

func TestCommandInterface_externalTypes_rejected(t *testing.T) {
	cmd := exec.Command("go", "test", "-tags=commandnegative", "./testdata/commandexternal")
	cacheDir := t.TempDir()
	tmpDir := t.TempDir()
	cmd.Env = append(cmd.Environ(), "GOCACHE="+cacheDir, "GOTMPDIR="+tmpDir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected commandnegative package to fail compilation")
	}
	if !strings.Contains(string(out), "does not implement") {
		t.Fatalf("expected compile failure mentioning interface mismatch, got:\n%s", out)
	}
}
