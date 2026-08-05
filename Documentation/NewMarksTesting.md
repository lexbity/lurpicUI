# Testing marks
New marks need integration tests that prove the event-routing junction end-to-end
(event -> handler -> store -> pixels), not just handler logic. Use the testkit `Drive*` helpers:

- `testkit.DriveClick`, `DriveKeyPress`, `DriveKeyRelease`, `DriveType`, `DriveDrag`, `DriveScroll`
  run the warmup frame that builds the hit map, then one frame per injected event. The runtime routes
  input against the PREVIOUS frame's hit map, so raw `InjectEvent` before any frame silently drops the
  event — always use the Drive* helpers. See `internal/testkit/drive.go`.
- Mount the mark as the harness root with `testkit.NewStandardHarness(t, width, height, mark)`;
  the mark fills the window. For non-root or clipped mounts, compose with the framework's layout
  containers (`layout.NewSizedBox`) or `facet.AttachLayer`.
- Name the files `*_integration_test.go`, drive the interaction, and assert the store/value mutation
  (load-bearing) plus, where clean, a pixel change.
- Structural contract assertions live in `marks/contracttest` (`AssertValueSurvivesDispose`,
  `AssertScaleInvalidates`, `AssertBindingNotSevered`, ...) and are teeth-tested in
  `marks/contracttest/contracttest_test.go`.
