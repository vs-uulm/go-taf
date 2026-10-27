package rtl

import "math"

// Projected probability decision with the bounds PUT and PTT of the RTL:
//   P   = b + a·u
//   PUT = b* + a(1 − b* − d*), d* = min(dt, 1 − bt), b* = max(bt, 1 − d* − ut)
//   PTT = max(bt + a(1 − bt), 1 − dt, 1 − ut(1 − a))
//   P > PTT -> trustworthy; P < PUT -> untrustworthy; PUT ≤ P ≤ PTT -> gray zone (rejected).

// Zone is the projected probability outcome.
type Zone int

const (
	ZoneUntrustworthy Zone = iota
	ZoneGray
	ZoneTrustworthy
)

func (z Zone) String() string {
	switch z {
	case ZoneTrustworthy:
		return "TRUSTWORTHY"
	case ZoneGray:
		return "GRAY ZONE"
	}
	return "UNTRUSTWORTHY"
}

// PPResult holds the projected probability decision details.
type PPResult struct {
	P, PUT, PTT float64
	Zone        Zone
	Trusted     bool // true only in ZoneTrustworthy
}

// ProjectedProbability returns P = b + a·u.
func ProjectedProbability(atl ATL, a float64) float64 {
	return atl.B + a*atl.U
}

// PBounds converts the RTL into PUT, the smallest P of an opinion that meets the RTL, and PTT, the largest P of an
// opinion that does not.
func PBounds(r RTL, a float64) (put, ptt float64) {
	dStar := math.Min(r.Dt, 1-r.Bt)
	bStar := math.Max(r.Bt, 1-dStar-r.Ut)
	put = bStar + a*(1-bStar-dStar)

	pFailB := r.Bt + a*(1-r.Bt)
	pFailD := 1 - r.Dt
	pFailU := 1 - r.Ut*(1-a)
	ptt = math.Max(pFailB, math.Max(pFailD, pFailU))
	return
}

// DecidePP decides on the projected probability. The gray zone is rejected (fail-safe).
func DecidePP(atl ATL, r RTL, a float64) PPResult {
	p := ProjectedProbability(atl, a)
	put, ptt := PBounds(r, a)
	res := PPResult{P: p, PUT: put, PTT: ptt}
	switch {
	case p > ptt:
		res.Zone, res.Trusted = ZoneTrustworthy, true
	case p < put:
		res.Zone = ZoneUntrustworthy
	default:
		res.Zone = ZoneGray
	}
	return res
}
