// Package viz implements Cartesian data-visualisation marks: axis, point,
// line, area, bar, and rule.
//
// Axis orientations
//
// An axis renders tick marks and numeric labels for a scale on one of four
// sides of the plot area:
//
//   - AxisBottom — ticks extend downward from the bottom edge; labels are
//     centered below each tick.
//   - AxisTop — ticks extend upward from the top edge; labels are centered
//     above each tick.
//   - AxisLeft — ticks extend rightward from the left boundary; labels are
//     placed to the left of each tick, vertically centered.
//   - AxisRight — ticks extend rightward from the right boundary; labels
//     are placed to the right of each tick, vertically centered.
//
// Labels are edge-clamped: the first and last labels are pushed inside the
// axis bounds (or omitted if they do not fit). Labels that would overlap a
// kept neighbor are skipped (collision avoidance).
//
// Time axes
//
// The Axis mark accepts a ReactiveScale backed by a TimeScale. Time-scale
// labels use the location configured on the scale (WithTimeLocation) or UTC
// by default. Labels are a pure function of the scale's location: they do
// not depend on the machine's local timezone. The interval between ticks
// (chooseInterval) selects the pre-defined interval whose approximate tick
// count is closest to the requested count.
package viz
