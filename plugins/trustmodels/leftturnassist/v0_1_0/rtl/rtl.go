// Package rtl holds the RTL decision logic of the TAF.
//   - ATL: Subjective Logic opinion (b + d + u = 1).
//   - RTL: independent thresholds (bt, dt, ut in [0,1], no sum constraint).
//   - Decide: accept if b >= bt && d <= dt && u <= ut.
package rtl

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/vs-uulm/go-taf/plugins/trustmodels/leftturnassist/v0_1_0/rtl/rf"
)

// ATL is the Actual Trustworthiness Level (binomial SL opinion).
type ATL struct {
	B, D, U float64
}

// RTL is the Required Trustworthiness Level. Each threshold is independent.
type RTL struct {
	Bt, Dt, Ut float64
}

// Sample is an ATL with known ground truth (training only).
type Sample struct {
	ATL         ATL
	GroundTruth bool // true = authentic, false = attacked
}

// Judger is any model that takes an ATL and returns the decision and the RTL used.
type Judger interface {
	Judge(ATL) (bool, RTL, error)
}

// sumTolerance is the numeric tolerance for b + d + u = 1.
const sumTolerance = 1e-3

// Validate checks that the ATL is a valid SL opinion.
func (a ATL) Validate() error {
	for name, v := range map[string]float64{"b": a.B, "d": a.D, "u": a.U} {
		if v < 0 || v > 1 || math.IsNaN(v) {
			return fmt.Errorf("invalid ATL: %s = %.4f outside [0,1]", name, v)
		}
	}
	if s := a.B + a.D + a.U; math.Abs(s-1) > sumTolerance {
		return fmt.Errorf("invalid ATL: b + d + u = %.4f (must be 1)", s)
	}
	return nil
}

// Validate checks that each threshold is in [0,1]. No sum constraint.
func (r RTL) Validate() error {
	for name, v := range map[string]float64{"bt": r.Bt, "dt": r.Dt, "ut": r.Ut} {
		if v < 0 || v > 1 || math.IsNaN(v) {
			return fmt.Errorf("invalid RTL: %s = %.4f outside [0,1]", name, v)
		}
	}
	return nil
}

// Clamp limits each threshold to [0,1] independently.
func (r RTL) Clamp() RTL {
	c := func(v float64) float64 { return math.Max(0, math.Min(1, v)) }
	return RTL{c(r.Bt), c(r.Dt), c(r.Ut)}
}

// Decide applies the component rule: b >= bt, d <= dt, u <= ut.
func Decide(a ATL, r RTL) bool {
	return a.B >= r.Bt && a.D <= r.Dt && a.U <= r.Ut
}

// ---------------------------------------------------------------------------
// Random Forest training
// ---------------------------------------------------------------------------

// Range is the [Min, Max] interval for a random threshold.
type Range struct{ Min, Max float64 }

// Validate checks 0 <= Min <= Max <= 1.
func (r Range) Validate(name string) error {
	if !(0 <= r.Min && r.Min <= r.Max && r.Max <= 1) {
		return fmt.Errorf("invalid %s range [%g, %g] (need 0 <= min <= max <= 1)", name, r.Min, r.Max)
	}
	return nil
}

// ParseRange parses "min,max".
func ParseRange(name, s string) (Range, error) {
	var r Range
	if _, err := fmt.Sscanf(strings.ReplaceAll(s, " ", ""), "%g,%g", &r.Min, &r.Max); err != nil {
		return r, fmt.Errorf("invalid %s range %q (use min,max, e.g. 0,1)", name, s)
	}
	return r, r.Validate(name)
}

// EdgeTolerance: a value closer than this share of the range width to a range edge counts as "at the edge".
const EdgeTolerance = 0.01

// EdgeHits lists the thresholds of r that lie at an edge of their range.
// At an inner edge (range narrower than [0,1]) the optimum may lie outside the range: widen it.
// At 0 or 1 the threshold is inactive (e.g. ut = 1 accepts any uncertainty).
func EdgeHits(r RTL, bt, dt, ut Range) (narrow, inactive []string) {
	check := func(name string, v float64, rg Range) {
		tol := EdgeTolerance * (rg.Max - rg.Min)
		for _, edge := range []float64{rg.Min, rg.Max} {
			if math.Abs(v-edge) > tol {
				continue
			}
			if edge == 0 || edge == 1 {
				inactive = append(inactive, fmt.Sprintf("%s=%.4f", name, v))
			} else {
				narrow = append(narrow, fmt.Sprintf("%s=%.4f at %g", name, v, edge))
			}
			return
		}
	}
	check("bt", r.Bt, bt)
	check("dt", r.Dt, dt)
	check("ut", r.Ut, ut)
	return
}

// TrainConfig holds RF training parameters.
type TrainConfig struct {
	NEstimators int         // number of trees
	Seed        int64       // reproducibility
	Candidates  int         // random RTLs per ATL (1 = one random RTL per ATL)
	Sampler     SamplerKind // uniform | lhs | halton
	MinLeaf     int         // minimum samples per leaf; 1 grows full trees (very large model files)
	BtRange     Range
	DtRange     Range
	UtRange     Range
}

// FullRange is the complete threshold range [0,1]: no assumption on where the RTL lies.
var FullRange = Range{0, 1}

// DefaultTrainConfig searches the full range [0,1] for every threshold.
func DefaultTrainConfig() TrainConfig {
	return TrainConfig{
		NEstimators: 100,
		Seed:        42,
		Candidates:  1,
		Sampler:     SamplerHalton,
		MinLeaf:     50,
		BtRange:     FullRange,
		DtRange:     FullRange,
		UtRange:     FullRange,
	}
}

// TrainingRow is one (ATL, random RTL) pair, exported to CSV.
type TrainingRow struct {
	ATL         ATL
	RTL         RTL
	Decision    bool
	GroundTruth bool
}

// TrainReport summarizes an RF training run.
type TrainReport struct {
	Samples   int // ATLs received
	Generated int // (ATL, RTL) pairs generated
	Used      int // pairs where Decide == ground truth (training set)
	Wrong     int // pairs where Decide != ground truth
	Sampler   SamplerKind
	Coverage  Coverage
	Accuracy  float64 // model accuracy on the received ATLs
	TP, TN    int
	FP, FN    int
	Rows      []TrainingRow
}

// Model is the trained RF that predicts the RTL from the ATL.
type Model struct {
	forest *rf.Forest
}

// Train compares each ATL with a random RTL, keeps the correct pairs and fits the RF.
func Train(samples []Sample, cfg TrainConfig) (*Model, TrainReport, error) {
	rep := TrainReport{Samples: len(samples), Sampler: cfg.Sampler}
	if len(samples) == 0 {
		return nil, rep, errors.New("no samples to train")
	}
	if cfg.Candidates < 1 {
		cfg.Candidates = 1
	}
	for _, x := range []struct {
		n string
		r Range
	}{{"bt", cfg.BtRange}, {"dt", cfg.DtRange}, {"ut", cfg.UtRange}} {
		if err := x.r.Validate(x.n); err != nil {
			return nil, rep, err
		}
	}
	total := len(samples) * cfg.Candidates
	smp, err := newSampler(cfg.Sampler, total, cfg.Seed)
	if err != nil {
		return nil, rep, err
	}
	scale := func(r Range, v float64) float64 { return r.Min + v*(r.Max-r.Min) }

	// Draw all points first, then assign them to the ATLs in random order: the sampler keeps its coverage of the
	// RTL space, and the RTL does not depend on the row position (Halton's base-2 axis alternates per index, and
	// the data is ordered by time and vehicle).
	units := make([][3]float64, total)
	for k := range units {
		units[k] = smp.Next()
	}
	assign := rand.New(rand.NewSource(cfg.Seed + 1)).Perm(total)

	var X, Y [][]float64
	k := 0
	for i, s := range samples {
		if err := s.ATL.Validate(); err != nil {
			return nil, rep, fmt.Errorf("sample %d: %w", i, err)
		}
		for c := 0; c < cfg.Candidates; c++ {
			// ATL is fixed (data); only the RTL is random.
			p := units[assign[k]]
			k++
			cand := RTL{scale(cfg.BtRange, p[0]), scale(cfg.DtRange, p[1]), scale(cfg.UtRange, p[2])}
			dec := Decide(s.ATL, cand)
			rep.Rows = append(rep.Rows, TrainingRow{s.ATL, cand, dec, s.GroundTruth})
			rep.Generated++
			if dec == s.GroundTruth {
				X = append(X, []float64{s.ATL.B, s.ATL.D, s.ATL.U})
				Y = append(Y, []float64{cand.Bt, cand.Dt, cand.Ut})
			} else {
				rep.Wrong++
			}
		}
	}
	rep.Used = len(X)
	rep.Coverage = measureCoverage(units)
	if rep.Used == 0 {
		return nil, rep, errors.New("no random RTL produced a correct decision; adjust the ranges")
	}

	m := &Model{forest: rf.Train(X, Y, rf.Params{
		NEstimators:    cfg.NEstimators,
		MinSamplesLeaf: cfg.MinLeaf,
		Seed:           cfg.Seed,
	})}

	// Evaluate the model on the received ATLs.
	for _, s := range samples {
		ok, _, _ := m.Judge(s.ATL)
		switch {
		case ok && s.GroundTruth:
			rep.TP++
		case !ok && !s.GroundTruth:
			rep.TN++
		case ok && !s.GroundTruth:
			rep.FP++ // attacked message accepted
		default:
			rep.FN++ // authentic message rejected
		}
	}
	rep.Accuracy = float64(rep.TP+rep.TN) / float64(len(samples))
	return m, rep, nil
}

// PredictRTL returns the trained RTL for an ATL.
func (m *Model) PredictRTL(a ATL) RTL {
	p := m.forest.Predict([]float64{a.B, a.D, a.U})
	return RTL{p[0], p[1], p[2]}.Clamp()
}

// Judge predicts the RTL for the ATL and applies Decide.
func (m *Model) Judge(a ATL) (bool, RTL, error) {
	if err := a.Validate(); err != nil {
		return false, RTL{}, err
	}
	r := m.PredictRTL(a)
	return Decide(a, r), r, nil
}

// Save writes the model as JSON.
func (m *Model) Save(path string) error { return m.forest.Save(path) }

// LoadModel reads the RF model from JSON.
func LoadModel(path string) (*Model, error) {
	f, err := rf.Load(path)
	if err != nil {
		return nil, err
	}
	return &Model{forest: f}, nil
}

// ---------------------------------------------------------------------------
// Risk-based (manual RTL, e.g. derived from the TARA)
// ---------------------------------------------------------------------------

// RiskBased is a fixed RTL set by parameters.
type RiskBased struct {
	RTL RTL
}

// NewRiskBased validates and wraps a manual RTL.
func NewRiskBased(r RTL) (*RiskBased, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return &RiskBased{RTL: r}, nil
}

// Judge compares the ATL with the manual RTL.
func (m *RiskBased) Judge(a ATL) (bool, RTL, error) {
	if err := a.Validate(); err != nil {
		return false, RTL{}, err
	}
	return Decide(a, m.RTL), m.RTL, nil
}
