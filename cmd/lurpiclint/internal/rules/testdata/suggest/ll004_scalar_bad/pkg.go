package ll004_scalar_bad

import (
	"codeburg.org/lexbit/lurpicui/marks/viz"
	"codeburg.org/lexbit/lurpicui/scale"
)

type BarBucket struct {
	Region  string
	Revenue float64
}

func newChart() *viz.Bar[BarBucket] {
	data := []BarBucket{{"NA", 100.0}, {"EU", 200.0}}
	s := scale.NewLinear()

	// LL004 (info): scalar accessor closures; prefer data.Encoding.
	return viz.NewBar(data,
		func(b BarBucket) string  { return b.Region },
		func(b BarBucket) float64 { return b.Revenue },
		s)
}
