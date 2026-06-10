# Golden Image Policy

This file records the cross-cutting policies that govern golden-image testing
in the LurpicUI project. Contributors must understand these before adding or
modifying golden tests.

## P-Golden: Goldens assert

- Golden images **assert** correctness. A missing golden is a **test failure**,
  not a signal to auto-create one.
- Baselines are created only by running `go test -update-golden` and must be
  visually reviewed in the PR.
- `*_actual.png` mismatch dumps are diagnostic-only and must never be tracked
  in git.

## P-Det: Deterministic rendering

- No `time.Now`. No machine-local timezone. Pinned fonts. Fixed seeds.
- Time formatting always honors the scale's configured `loc`. Tests pass
  identically under any `TZ` environment variable.
- Any test that calls `time.Now()` or depends on the system clock is a
  determinism bug. Use `testkit.DeterministicTime` instead.

## P-RTL: Directional non-identity

- Every mark that declares itself directionally asymmetric must produce
  different pixels under LTR vs RTL.
- Symmetric marks (e.g. `progress_ring`, `status_light`) are explicitly listed
  in `marks/rtl_conformance_test.go`'s `rtlExempt` table with a reason.
- Adding a mark to `rtlExempt` requires a justification. Removing one requires
  that its RTL golden test asserts non-identity with the LTR golden.

## P-Stroke: Segment-normal stroke geometry

- Border/stroke offset is computed per-segment: straight edges offset along
  their perpendicular; curve control points offset along the miter bisector.
- The old centroid-radial offset (which caused corner-bracket artifacts) is
  superseded by `gfx.OffsetContour`.
- Stroke-geometry correctness is validated by a dedicated unit test
  (`gfx/offset_contour_test.go`) independent of any mark golden.

## P-Pin: No behavior flips without a test

- Any change that alters tick selection, label elision, or mirroring lands
  with a unit test that pins the new behavior and a regenerated, reviewed
  golden.
