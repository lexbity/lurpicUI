package equivalence

import (
	"math"
	"sort"
)

// computeDiff returns, for two RGBA buffers:
//   - PSNR over all RGBA channels in dB
//   - the 99th percentile of per-channel absolute differences
//   - the fraction of pixels whose max-channel difference is <= tol.MaxDiff
//   - the observed maximum per-pixel max-channel difference
//   - the count of pixels whose max-channel difference exceeds tol.MaxDiff
func computeDiff(a, b []byte, width, height int, tol EquivalenceTolerance) (psnr, p99, within, maxObserved float64, outliers int) {
	pixelCount := width * height

	var mse float64
	perChannelDiffs := make([]float64, 0, pixelCount*4)

	withinCount := 0
	maxObserved = 0
	for i := 0; i < pixelCount; i++ {
		off := i * 4
		pixelMax := 0.0
		for c := 0; c < 4; c++ {
			d := math.Abs(float64(a[off+c]) - float64(b[off+c]))
			mse += d * d
			perChannelDiffs = append(perChannelDiffs, d)
			if d > pixelMax {
				pixelMax = d
			}
		}
		if pixelMax > maxObserved {
			maxObserved = pixelMax
		}
		if pixelMax <= tol.MaxDiff {
			withinCount++
		}
	}

	mse /= float64(pixelCount * 4)
	if mse == 0 {
		psnr = math.Inf(1)
	} else {
		psnr = 10 * math.Log10(255*255/mse)
	}

	sort.Float64s(perChannelDiffs)
	idx := int(math.Ceil(0.99*float64(len(perChannelDiffs)))) - 1
	if idx < 0 {
		idx = 0
	}
	p99 = perChannelDiffs[idx]
	within = float64(withinCount) / float64(pixelCount)
	outliers = pixelCount - withinCount
	return
}
