package testkit

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// CapturedCommand records a single gfx.Command with its type and key fields.
type CapturedCommand struct {
	Kind     string      // "FillRect", "FillPath", "GlyphRun", etc.
	Bounds   gfx.Rect    // for FillRect, FillPath
	Color    gfx.Color   // for FillRect, FillPath
	Text     string      // for GlyphRun
	Position gfx.Point   // for GlyphRun origin
	Opacity  float32     // for PushOpacity
}

// CommandCapture iterates a CommandList and records each command.
type CommandCapture struct {
	Commands []CapturedCommand
}

// Capture walks cmds and stores a snapshot.
func (c *CommandCapture) Capture(cmds *gfx.CommandList) {
	c.Commands = c.Commands[:0]
	if cmds == nil {
		return
	}
	for _, cmd := range cmds.Commands {
		switch v := cmd.(type) {
		case gfx.FillRect:
			c.Commands = append(c.Commands, CapturedCommand{
				Kind:   "FillRect",
				Bounds: v.Rect,
				Color:  v.Brush.Color,
			})
		case gfx.FillPath:
			c.Commands = append(c.Commands, CapturedCommand{
				Kind:  "FillPath",
				Color: v.Brush.Color,
			})
		case gfx.DrawGlyphRun:
			c.Commands = append(c.Commands, CapturedCommand{
				Kind:     "GlyphRun",
				Text:     v.Run.Text,
				Position: v.Origin,
				Color:    v.Brush.Color,
			})
		case gfx.PushOpacity:
			c.Commands = append(c.Commands, CapturedCommand{
				Kind:    "PushOpacity",
				Opacity: v.Alpha,
			})
		case gfx.StrokeRect:
			c.Commands = append(c.Commands, CapturedCommand{
				Kind:   "StrokeRect",
				Bounds: v.Rect,
				Color:  v.Brush.Color,
			})
		case gfx.StrokePath:
			c.Commands = append(c.Commands, CapturedCommand{
				Kind:  "StrokePath",
				Color: v.Brush.Color,
			})
		case gfx.DrawSelectionRects:
			c.Commands = append(c.Commands, CapturedCommand{
				Kind: "SelectionRects",
			})
		default:
			c.Commands = append(c.Commands, CapturedCommand{Kind: "other"})
		}
	}
}

// FindGlyphRunWithText returns the first GlyphRun command whose text contains substr.
func (c *CommandCapture) FindGlyphRunWithText(substr string) (CapturedCommand, bool) {
	for _, cmd := range c.Commands {
		if cmd.Kind == "GlyphRun" && contains(cmd.Text, substr) {
			return cmd, true
		}
	}
	return CapturedCommand{}, false
}

// FindFillRectWithColor returns the first FillRect command whose color matches.
func (c *CommandCapture) FindFillRectWithColor(want gfx.Color) (CapturedCommand, bool) {
	for _, cmd := range c.Commands {
		if cmd.Kind == "FillRect" && colorsClose(cmd.Color, want, 2) {
			return cmd, true
		}
	}
	return CapturedCommand{}, false
}

// HasFillRect reports whether any FillRect command exists.
func (c *CommandCapture) HasFillRect() bool {
	for _, cmd := range c.Commands {
		if cmd.Kind == "FillRect" {
			return true
		}
	}
	return false
}

// HasGlyphRun reports whether any GlyphRun command exists.
func (c *CommandCapture) HasGlyphRun() bool {
	for _, cmd := range c.Commands {
		if cmd.Kind == "GlyphRun" {
			return true
		}
	}
	return false
}

// HasFillPath reports whether any FillPath command exists.
func (c *CommandCapture) HasFillPath() bool {
	for _, cmd := range c.Commands {
		if cmd.Kind == "FillPath" {
			return true
		}
	}
	return false
}

// AssertHasGlyphRun fails if no GlyphRun command is present.
func (c *CommandCapture) AssertHasGlyphRun(t *testing.T) {
	t.Helper()
	if !c.HasGlyphRun() {
		t.Error("expected at least one GlyphRun command")
	}
}

// AssertHasFillPath fails if no FillPath command is present.
func (c *CommandCapture) AssertHasFillPath(t *testing.T) {
	t.Helper()
	if !c.HasFillPath() {
		t.Error("expected at least one FillPath command")
	}
}

// AssertHasFillRect fails if no FillRect command is present.
func (c *CommandCapture) AssertHasFillRect(t *testing.T) {
	t.Helper()
	if !c.HasFillRect() {
		t.Error("expected at least one FillRect command")
	}
}

// AssertGlyphRunText fails if no GlyphRun contains substr.
func (c *CommandCapture) AssertGlyphRunText(t *testing.T, substr string) {
	t.Helper()
	if _, ok := c.FindGlyphRunWithText(substr); !ok {
		t.Errorf("expected GlyphRun containing %q", substr)
	}
}

// AssertFillRectColor fails if no FillRect with the given color is found.
func (c *CommandCapture) AssertFillRectColor(t *testing.T, want gfx.Color) {
	t.Helper()
	if _, ok := c.FindFillRectWithColor(want); !ok {
		t.Errorf("expected FillRect with color %#v", want)
	}
}

func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func absUint8(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func colorsClose(a, b gfx.Color, tol uint8) bool {
	return absUint8(uint8(a.R*255), uint8(b.R*255)) <= tol &&
		absUint8(uint8(a.G*255), uint8(b.G*255)) <= tol &&
		absUint8(uint8(a.B*255), uint8(b.B*255)) <= tol &&
		absUint8(uint8(a.A*255), uint8(b.A*255)) <= tol
}
