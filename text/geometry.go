package text

import "reflect"

// Width returns the width of a shaped layout, or 0 for nil.
func Width(layout *TextLayout) float32 {
	if layout == nil {
		return 0
	}
	return layout.Bounds.Width()
}

// Height returns the height of a shaped layout, or 0 for nil.
func Height(layout *TextLayout) float32 {
	if layout == nil {
		return 0
	}
	return layout.Bounds.Height()
}

// MaxWidth returns the maximum layout width from the supplied layouts.
func MaxWidth(layouts ...*TextLayout) float32 {
	var out float32
	for _, layout := range layouts {
		if width := Width(layout); width > out {
			out = width
		}
	}
	return out
}

// MaxHeight returns the maximum layout height from the supplied layouts.
func MaxHeight(layouts ...*TextLayout) float32 {
	var out float32
	for _, layout := range layouts {
		if height := Height(layout); height > out {
			out = height
		}
	}
	return out
}

// CenterY returns the top edge needed to vertically center content within bounds.
func CenterY[T interface{ Height() float32 }](bounds T, contentH float32) float32 {
	_, minY, ok := rectMinXY(bounds)
	if !ok {
		return 0
	}
	return minY + maxFloat32(0, (bounds.Height()-contentH)*0.5)
}

// CenterRect returns a rectangle with the provided size centered in bounds.
func CenterRect[T interface {
	Width() float32
	Height() float32
}](bounds T, width, height float32) T {
	minX, minY, ok := rectMinXY(bounds)
	if !ok {
		var zero T
		return zero
	}
	return rectFromXYWH(bounds,
		minX+maxFloat32(0, (bounds.Width()-width)*0.5),
		minY+maxFloat32(0, (bounds.Height()-height)*0.5),
		width,
		height,
	)
}

// AlignRectY returns the same rect shifted so its vertical center stays within
// the provided content box.
func AlignRectY[T interface {
	Width() float32
	Height() float32
}](r T, top, contentH float32) T {
	if isEmptyRect(r) {
		return r
	}
	minX, _, ok := rectMinXY(r)
	if !ok {
		return r
	}
	delta := maxFloat32(0, (contentH-r.Height())*0.5)
	return rectFromXYWH(r, minX, top+delta, r.Width(), r.Height())
}

func rectMinXY[T any](r T) (float32, float32, bool) {
	rv := reflect.ValueOf(r)
	if !rv.IsValid() {
		return 0, 0, false
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return 0, 0, false
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return 0, 0, false
	}
	min := rv.FieldByName("Min")
	if !min.IsValid() || min.Kind() != reflect.Struct {
		return 0, 0, false
	}
	x := min.FieldByName("X")
	y := min.FieldByName("Y")
	if !x.IsValid() || !y.IsValid() {
		return 0, 0, false
	}
	return float32(x.Float()), float32(y.Float()), true
}

func isEmptyRect[T interface {
	Width() float32
	Height() float32
}](r T) bool {
	return r.Width() <= 0 || r.Height() <= 0
}

func rectFromXYWH[T any](template T, x, y, w, h float32) T {
	rv := reflect.ValueOf(template)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	out := reflect.New(rv.Type()).Elem()
	out.Set(rv)
	min := out.FieldByName("Min")
	max := out.FieldByName("Max")
	if min.IsValid() && min.Kind() == reflect.Struct {
		if fx := min.FieldByName("X"); fx.IsValid() && fx.CanSet() {
			fx.SetFloat(float64(x))
		}
		if fy := min.FieldByName("Y"); fy.IsValid() && fy.CanSet() {
			fy.SetFloat(float64(y))
		}
	}
	if max.IsValid() && max.Kind() == reflect.Struct {
		if fx := max.FieldByName("X"); fx.IsValid() && fx.CanSet() {
			fx.SetFloat(float64(x + w))
		}
		if fy := max.FieldByName("Y"); fy.IsValid() && fy.CanSet() {
			fy.SetFloat(float64(y + h))
		}
	}
	return out.Interface().(T)
}
