package rtl

import (
	"math"
	"testing"
)

// TestPTT checks PTT for known RTLs and for each of its three terms.
func TestPTT(t *testing.T) {
	cases := []struct {
		r   RTL
		ptt float64
	}{
		{RTL{0.8, 0.225, 0.4}, 0.90},
		{RTL{0.5, 0.2, 0.4}, 0.80},
		{RTL{0.9, 1, 1}, 0.95},  // b < bt: b just below 0.9, u = 0.1
		{RTL{0, 0.05, 1}, 0.95}, // d > dt: d just above 0.05, b = 0.95
		{RTL{0, 1, 0.04}, 0.98}, // u > ut: u just above 0.04, b = 0.96
	}
	for _, c := range cases {
		if ptt := PTT(c.r, 0.5); math.Abs(ptt-c.ptt) > 1e-9 {
			t.Errorf("%+v: PTT = %.4f, want %.2f", c.r, ptt, c.ptt)
		}
	}
}
