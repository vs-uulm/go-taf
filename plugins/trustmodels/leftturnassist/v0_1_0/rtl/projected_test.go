package rtl

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// pp is the projected probability P = b + a·u of an ATL with base rate a = 0.5.
func pp(w ATL) float64 { return w.B + 0.5*w.U }

// TestPBounds checks PUT and PTT for known RTLs.
func TestPBounds(t *testing.T) {
	cases := []struct {
		r        RTL
		put, ptt float64
	}{
		{RTL{0.8, 0.225, 0.4}, 0.80, 0.90},
		{RTL{0.5, 0.2, 0.4}, 0.65, 0.80},
	}
	for _, c := range cases {
		put, ptt := PUT(c.r, 0.5), PTT(c.r, 0.5)
		if !near(put, c.put) || !near(ptt, c.ptt) {
			t.Errorf("%+v: PUT=%.4f PTT=%.4f, want %.2f / %.2f", c.r, put, ptt, c.put, c.ptt)
		}
	}
}

// TestGrayZone checks ATLs in and around the gray zone.
func TestGrayZone(t *testing.T) {
	// ω1 passes the component rule but lies in the gray zone.
	r := RTL{0.5, 0.2, 0.4}
	w1 := ATL{0.6, 0.2, 0.2}
	if p := pp(w1); p < PUT(r, 0.5) || p > PTT(r, 0.5) {
		t.Errorf("ω1: P = %.4f is not in the gray zone", p)
	}
	if !Decide(w1, r) {
		t.Error("ω1 should pass the component rule")
	}
	// (0.8,0.1,0.1) and (0.7,0,0.3) both have P = 0.85: gray zone.
	r2 := RTL{0.8, 0.225, 0.4}
	for _, w := range []ATL{{0.8, 0.1, 0.1}, {0.7, 0, 0.3}} {
		if p := pp(w); p < PUT(r2, 0.5) || p > PTT(r2, 0.5) {
			t.Errorf("%+v: P = %.4f is not in the gray zone", w, p)
		}
	}
	if p := pp(ATL{0.95, 0, 0.05}); p <= PTT(r2, 0.5) {
		t.Errorf("P=0.975 should be trustworthy (above PTT): P = %.4f", p)
	}
	if p := pp(ATL{0.3, 0.6, 0.1}); p >= PUT(r2, 0.5) {
		t.Errorf("P=0.35 should be untrustworthy (below PUT): P = %.4f", p)
	}
}
