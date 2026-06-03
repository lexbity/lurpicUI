# `scale` Package

Data-visualization scale primitives for mapping abstract data values onto
visual coordinates and back. Part of the lurpicUI engine's grammar of
data-bound marks.

## Package layout

```
scale/                      pure core — stdlib only
  scale.go            Scale, InvertibleScale, Ticker, enums, options
  interpolate.go      lerp, normalize, clamp01
  clamp.go            clamp, clampOutOfRange
  linear.go           LinearScale
  log.go              LogScale (+ reflected-log for negative domains)
  pow.go              PowScale, NewSqrt
  band.go             BandScale, shared ordinalLayout computation
  point.go            PointScale (zero bandwidth, nearest-member invert)
  time.go             TimeScale (float64 Unix-millis, time.Time convenience)
  nice.go             tickStep, ticks, Nice (1/2/5 mantissa)
  format.go           FormatFixed, FormatSignificant, FormatSI
  timeticks.go        calendar-aware time intervals (1s…1y)
  timeformat.go       time label formatting with redundant-field elision
  extent.go           Extent, ExtentBy[T], NiceExtent
  zoom.go             PanDomain, ZoomDomain (semantic zoom transforms)

scale/colorscale/            imports gfx for Color
  colorspace.go       sRGB↔linear, OKLab conversions
  ramp.go             Ramp, InterpolationSpace, built-in ramps
  sequential.go       SequentialColor
  diverging.go        DivergingColor

scale/reactive/              imports store, signal
  reactive.go         ReactiveScale, NewXxxReactive, NewXxxReactiveFromDerived
  domain_source.go    DomainFromCollection[T]
  region.go           RangeFromRegion
  zoom_controller.go  ZoomController (semantic zoom, pan)
```

## Quick start

### Continuous scales

```go
s := scale.NewLinear(
    scale.WithDomain(0, 100),
    scale.WithRange(0, 500),
)
s.Map(50)              // → 250
s.Invert(250)          // → 50
s.Ticks(5)             // → [{0 "0"} {20 "20"} …]
```

Log scale with base 10 (default) and reflected-log for negative domains:

```go
s, _ := scale.NewLog(
    scale.WithDomain(1, 1000),
    scale.WithRange(0, 500),
)
s.Map(10)              // → 166.7
```

Power scale (area-to-radius encoding via sqrt):

```go
s, _ := scale.NewSqrt(
    scale.WithDomain(0, 100),
    scale.WithRange(0, 500),
)
s.Map(25)              // → 250
```

Time scale (domain in Unix milliseconds):

```go
s := scale.NewTime(
    scale.WithTimeDomain(t0, t1),
    scale.WithRange(0, 500),
)
s.Map(float64(t.UnixMilli()))  // linear in ms
s.Ticks(10)                    // calendar-aligned ticks
```

### Ordinal scales

```go
band := scale.NewBand(
    []string{"A", "B", "C"},
    scale.WithRange(0, 300),
    scale.WithPaddingInner(0.1),
)
band.Band("B")          // → (100, 90, true)
band.InvertRange(150)   // → ("B", true)

point := scale.NewPoint(
    []string{"X", "Y", "Z"},
    scale.WithRange(0, 200),
)
point.Position("Y")     // → 100
point.InvertRange(50)   // → ("X", true)  nearest-member
```

### Color scales

```go
s := colorscale.NewSequential(
    0, 100,
    colorscale.RampViridis,
    colorscale.InterpolationOKLab,
)
col := s.Map(50)                // → gfx.Color at perceptual midpoint

d := colorscale.NewDiverging(
    -1, 1, 0,
    colorscale.RampBlueWhiteRedLow,
    colorscale.RampBlueWhiteRedHigh,
    colorscale.InterpolationOKLab,
)
col := d.Map(0)                 // → white (midpoint)
```

### Reactive (live data)

```go
domain := store.NewValueStore([2]float64{0, 100})
rng    := store.NewValueStore([2]float64{0, 500})
rs := reactive.NewLinearReactive(domain, rng)

domain.Set([2]float64{0, 200})  // next Get returns updated scale
```

From a collection:

```go
coll := store.NewCollectionStore(identify)
coll.Insert(item{val: 10})
derivedDomain := reactive.DomainFromCollection(coll, accessor)
rng := reactive.RangeFromRegion(0, 500)
rs := reactive.NewLinearReactiveFromDerived(derivedDomain, rngDerived)
```

Semantic zoom:

```go
zc := reactive.NewZoomController(domain)
zc.Zoom(focalValue, 2)   // zoom in 2x around focal
zc.Pan(50)               // pan right
```

## Design principles

1. **Pure core** — `scale/` imports only the standard library.
2. **`float64` internally** — narrowing to `float32` at the gfx boundary.
3. **Scale maps `data ↔ local layer space`** — never screen space.
4. **Value semantics** — concrete scales are copyable value types.
5. **Total functions** — `Map`/`Invert` never panic.
