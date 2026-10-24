package rtl

import (
	"math"
	"testing"
)

// TestRTLIndependentOfRowOrder: with identical ATLs in a fixed order, the random bt must not
// depend on the row position (e.g. even vs odd rows).
func TestRTLIndependentOfRowOrder(t *testing.T) {
	const n = 4000
	samples := make([]Sample, n)
	for i := range samples {
		samples[i] = Sample{ATL: ATL{0.6, 0.2, 0.2}, GroundTruth: i%2 == 0}
	}
	for _, kind := range AllSamplers {
		cfg := DefaultTrainConfig()
		cfg.Sampler = kind
		_, rep, err := Train(samples, cfg)
		if err != nil {
			t.Fatal(err)
		}
		var sum [2]float64
		var cnt [2]int
		for i, r := range rep.Rows {
			sum[i%2] += r.RTL.Bt
			cnt[i%2]++
		}
		diff := math.Abs(sum[0]/float64(cnt[0]) - sum[1]/float64(cnt[1]))
		t.Logf("%-7s mean bt even-odd rows: %.4f", kind, diff)
		if diff > 0.02 {
			t.Errorf("%s: bt depends on row parity (diff %.4f)", kind, diff)
		}
	}
}
