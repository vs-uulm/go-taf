package rtl

import "math"

// PTT is the largest projected probability P = b + a·u of an opinion that does not meet the RTL:
//
//	PTT = max(bt + a(1 − bt), 1 − dt, 1 − ut(1 − a))
//
// the largest P of an opinion with b < bt, with d > dt, and with u > ut. The projected-probability rule trusts an ATL
// only if its P is above PTT; an ATL whose P is not, but that meets the RTL, is rejected (fail-safe).
func PTT(r RTL, a float64) float64 {
	return math.Max(r.Bt+a*(1-r.Bt), math.Max(1-r.Dt, 1-r.Ut*(1-a)))
}
