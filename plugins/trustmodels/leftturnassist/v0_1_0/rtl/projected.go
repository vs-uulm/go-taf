package rtl

import "math"

// The projected-probability rule compares the projected probability P = b + a·u of the ATL with two bounds of the RTL:
// above PTT, every opinion meets the RTL; below PUT, none does; in between (the gray zone), P alone cannot tell.

// PTT is the largest projected probability of an opinion that does not meet the RTL:
//
//	PTT = max(bt + a(1 − bt), 1 − dt, 1 − ut(1 − a))
//
// the largest P of an opinion with b < bt, with d > dt, and with u > ut.
func PTT(r RTL, a float64) float64 {
	return math.Max(r.Bt+a*(1-r.Bt), math.Max(1-r.Dt, 1-r.Ut*(1-a)))
}

// PUT is the smallest projected probability of an opinion that meets the RTL:
//
//	PUT = b* + a(1 − b* − d*), d* = min(dt, 1 − bt), b* = max(bt, 1 − d* − ut)
//
// the opinion with the least belief and the most disbelief that the RTL allows.
func PUT(r RTL, a float64) float64 {
	dStar := math.Min(r.Dt, 1-r.Bt)
	bStar := math.Max(r.Bt, 1-dStar-r.Ut)
	return bStar + a*(1-bStar-dStar)
}
