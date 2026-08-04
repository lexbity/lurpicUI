package testkit

import "testing"

// The integration smoke tests are the teeth of the whole Drive* helper design:
// if the warmup frame is missing, events are routed against a nil hit map and
// the inside-click test fails; if hit-testing is broken, the outside-click
// test fails. Together they prove the event -> routing -> handler junction.

func TestIntegration_ClickInsideFiresHandler(t *testing.T) {
	root := newClickCounterFacet()
	h := NewHarness(t, testHarnessConfig(t), root)
	DriveClick(h, 5, 5)
	if root.clickCount != 1 {
		t.Fatalf("expected clickCount=1 after DriveClick inside bounds, got %d", root.clickCount)
	}
}

func TestIntegration_ClickOutsideDoesNotFire(t *testing.T) {
	root := newClickCounterFacet()
	h := NewHarness(t, testHarnessConfig(t), root)
	DriveClick(h, 999, 999)
	if root.clickCount != 0 {
		t.Fatalf("expected clickCount=0 after DriveClick outside bounds, got %d", root.clickCount)
	}
}

func TestIntegration_DoubleClickFiresTwice(t *testing.T) {
	root := newClickCounterFacet()
	h := NewHarness(t, testHarnessConfig(t), root)
	DriveClick(h, 5, 5)
	DriveClick(h, 5, 5)
	if root.clickCount != 2 {
		t.Fatalf("expected clickCount=2 after two DriveClicks, got %d", root.clickCount)
	}
}
