package testkit

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
)

// UTC is the fixed UTC time location for deterministic golden rendering.
// Tests must use this instead of time.Local to ensure reproducible output
// regardless of the machine timezone.
var UTC = time.UTC

// DeterministicTime returns a time.Time in UTC for deterministic golden
// rendering. The intent is explicit: the result is always UTC and must
// not depend on machine local timezone.
func DeterministicTime(year int, month time.Month, day, hour, min, sec, nsec int) time.Time {
	return time.Date(year, month, day, hour, min, sec, nsec, time.UTC)
}

// RenderRTLPair calls fn twice — once with LTR and once with RTL writing
// direction — and returns both resulting surfaces.
func RenderRTLPair(t testing.TB, fn func(t testing.TB, dir facet.WritingDirection) *MemorySurface) (ltr, rtl *MemorySurface) {
	t.Helper()
	ltr = fn(t, facet.WritingDirectionLTR)
	rtl = fn(t, facet.WritingDirectionRTL)
	return
}

// AssertDiffers asserts that two surfaces differ visually beyond the
// standard golden tolerance (2/255 per channel).
func AssertDiffers(t reporter, a, b *MemorySurface, name string) {
	t.Helper()
	imgA := a.Capture()
	imgB := b.Capture()
	if imagesClose(imgA, imgB, 2) {
		t.Errorf("%s: surfaces are identical (expected different output)", name)
	}
}

// AssertGoldenPair asserts that the LTR surface matches <baseName>_default,
// the RTL surface matches <baseName>_rtl, and that the two surfaces differ
// from each other.
func AssertGoldenPair(t testing.TB, ltr, rtl *MemorySurface, baseName string) {
	t.Helper()
	AssertGolden(t, ltr, baseName+"_default")
	AssertGolden(t, rtl, baseName+"_rtl")
	AssertDiffers(t, ltr, rtl, baseName)
}
