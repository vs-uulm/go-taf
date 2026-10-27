package rtl

import (
	"fmt"
	"math"
	"math/rand"
)

// SamplerKind selects how the random RTL (bt, dt, ut) is drawn for each ATL.
type SamplerKind string

const (
	// SamplerUniform draws independent uniform points (Monte Carlo).
	SamplerUniform SamplerKind = "uniform"
	// SamplerLHS is Latin Hypercube Sampling (McKay, Beckman & Conover, 1979).
	SamplerLHS SamplerKind = "lhs"
	// SamplerHalton is a scrambled Halton sequence (Halton, 1960; Braaten & Weller, 1979).
	SamplerHalton SamplerKind = "halton"
)

// AllSamplers lists the available samplers.
var AllSamplers = []SamplerKind{SamplerUniform, SamplerLHS, SamplerHalton}

// sampler returns points in the unit cube [0,1)^3.
type sampler interface {
	Next() [3]float64
}

// newSampler builds one global sequence of 'total' points (no repeats across ATLs).
func newSampler(kind SamplerKind, total int, seed int64) (sampler, error) {
	rng := rand.New(rand.NewSource(seed))
	switch kind {
	case SamplerUniform, "":
		return &uniformSampler{rng}, nil
	case SamplerLHS:
		return newLHS(total, rng), nil
	case SamplerHalton:
		return newHalton(rng), nil
	}
	return nil, fmt.Errorf("unknown sampler: %q (use uniform | lhs | halton)", kind)
}

// uniformSampler draws independent uniform points.
type uniformSampler struct{ rng *rand.Rand }

func (s *uniformSampler) Next() [3]float64 {
	return [3]float64{s.rng.Float64(), s.rng.Float64(), s.rng.Float64()}
}

// lhsSampler places exactly one point per stratum on each axis.
type lhsSampler struct {
	pts [][3]float64
	i   int
	rng *rand.Rand
}

func newLHS(n int, rng *rand.Rand) *lhsSampler {
	if n < 1 {
		n = 1
	}
	pts := make([][3]float64, n)
	for d := 0; d < 3; d++ {
		perm := rng.Perm(n)
		for j := 0; j < n; j++ {
			pts[j][d] = (float64(perm[j]) + rng.Float64()) / float64(n)
		}
	}
	return &lhsSampler{pts: pts, rng: rng}
}

func (s *lhsSampler) Next() [3]float64 {
	if s.i >= len(s.pts) { // fallback if more points are requested than planned
		return [3]float64{s.rng.Float64(), s.rng.Float64(), s.rng.Float64()}
	}
	p := s.pts[s.i]
	s.i++
	return p
}

// haltonSampler uses bases 2, 3, 5 with random digit permutations.
type haltonSampler struct {
	idx   int
	bases [3]int
	perms [3][]int
}

func newHalton(rng *rand.Rand) *haltonSampler {
	h := &haltonSampler{idx: 1, bases: [3]int{2, 3, 5}} // skip index 0 (origin)
	for d, b := range h.bases {
		// Permute digits 1..b-1; keep 0 -> 0 to avoid infinite digit expansions.
		p := make([]int, b)
		rest := rng.Perm(b - 1)
		for i := 1; i < b; i++ {
			p[i] = rest[i-1] + 1
		}
		h.perms[d] = p
	}
	return h
}

func (h *haltonSampler) Next() [3]float64 {
	var p [3]float64
	for d, b := range h.bases {
		p[d] = radicalInverse(h.idx, b, h.perms[d])
	}
	h.idx++
	return p
}

// radicalInverse mirrors the base-b digits of i around the decimal point.
func radicalInverse(i, base int, perm []int) float64 {
	inv, f := 0.0, 1.0/float64(base)
	for i > 0 {
		inv += float64(perm[i%base]) * f
		i /= base
		f /= float64(base)
	}
	return inv
}

// Coverage measures how well the random RTLs fill the (bt, dt, ut) space.
type Coverage struct {
	Occupancy  float64 // share of occupied cells in a 10x10x10 grid
	Duplicates int     // repeated RTLs at 3 decimal places
	MaxGap     float64 // largest empty run (100 bins), summed over the 3 axes
}

func measureCoverage(units [][3]float64) Coverage {
	const g = 10
	cells := map[[3]int]bool{}
	seen := map[[3]int]bool{}
	var c Coverage
	for _, u := range units {
		cells[[3]int{int(u[0] * g), int(u[1] * g), int(u[2] * g)}] = true
		k := [3]int{int(math.Round(u[0] * 1000)), int(math.Round(u[1] * 1000)), int(math.Round(u[2] * 1000))}
		if seen[k] {
			c.Duplicates++
		}
		seen[k] = true
	}
	c.Occupancy = float64(len(cells)) / math.Min(float64(len(units)), g*g*g)
	for d := 0; d < 3; d++ {
		var bins [100]bool
		for _, u := range units {
			bins[min(int(u[d]*100), 99)] = true
		}
		run, best := 0, 0
		for _, b := range bins {
			if b {
				run = 0
			} else {
				run++
				best = max(best, run)
			}
		}
		c.MaxGap += float64(best) / 100
	}
	return c
}
