package leftturnassist_v0_1_0

// B-spline quantification approach for MBD evidence.
// Ported from code_quantification/bspline/golang/bsplinetrust.go.

import (
	"encoding/json"
	"fmt"
	"math"
)

const mbdSoftplusThreshold = 20.0

var mbdModelFeatureOrder = [mbdNumFeatures]string{
	"pos_err", "speed_err", "acc_err", "road_edge", "time_err", "heading_err",
}

// mbdBsplineModel represents the B-spline model JSON (content of bspline_model.json)
type mbdBsplineModel struct {
	Variant       string      `json:"variant"`
	Features      []string    `json:"features"`
	Degree        int         `json:"degree"`
	Extrapolation string      `json:"extrapolation"`
	UMin          float64     `json:"u_min"`
	BaseRate      float64     `json:"base_rate"`
	Threshold     float64     `json:"threshold"`
	Temperature   float64     `json:"temperature"`
	Knots         [][]float64 `json:"knots"`
	NBasis        []int       `json:"n_basis"`
	WB            []float64   `json:"w_b"`
	WU            []float64   `json:"w_u"`

	offsets  []int
	totalDim int
}

// loadMbdBsplineModel parses and validates the B-spline model JSON (content of bspline_model.json)
func loadMbdBsplineModel(rawJSON string) (*mbdBsplineModel, error) {
	var m mbdBsplineModel
	if err := json.Unmarshal([]byte(rawJSON), &m); err != nil {
		return nil, fmt.Errorf("parsing MBD B-spline model: %w", err)
	}
	if err := m.finalize(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *mbdBsplineModel) finalize() error {
	if len(m.Features) != mbdNumFeatures {
		return fmt.Errorf("features has length %d, expected %d", len(m.Features), mbdNumFeatures)
	}
	for i, name := range m.Features {
		if name != mbdModelFeatureOrder[i] {
			return fmt.Errorf("unexpected features[%d] %q, expected %q", i, name, mbdModelFeatureOrder[i])
		}
	}
	if len(m.Knots) != mbdNumFeatures || len(m.NBasis) != mbdNumFeatures {
		return fmt.Errorf("knots/n_basis length != %d", mbdNumFeatures)
	}
	if m.Degree < 1 {
		return fmt.Errorf("degree < 1")
	}
	if m.Extrapolation != "constant" {
		return fmt.Errorf("only extrapolation=constant is supported, not %q", m.Extrapolation)
	}
	m.offsets = make([]int, mbdNumFeatures)
	m.totalDim = 0
	for f := 0; f < mbdNumFeatures; f++ {
		nb := len(m.Knots[f]) - m.Degree - 1
		if nb != m.NBasis[f] {
			return fmt.Errorf("feature %d: len(knots)-degree-1=%d != n_basis=%d", f, nb, m.NBasis[f])
		}
		m.offsets[f] = m.totalDim
		m.totalDim += nb
	}
	if len(m.WB) != m.totalDim || len(m.WU) != m.totalDim {
		return fmt.Errorf("w_b/w_u length != totalDim %d", m.totalDim)
	}
	if m.UMin == 0 {
		m.UMin = 1e-3
	}
	if m.BaseRate == 0 {
		m.BaseRate = 0.5
	}
	if m.Threshold == 0 {
		m.Threshold = 0.5
	}
	if m.Temperature == 0 {
		m.Temperature = 1.0
	}
	return nil
}

func (m *mbdBsplineModel) computeTrustOpinion(features [mbdNumFeatures]float64) mbdOpinion {
	phi := make([]float64, m.totalDim)
	for f := 0; f < mbdNumFeatures; f++ {
		row := mbdDesignRow(features[f], m.Knots[f], m.Degree)
		copy(phi[m.offsets[f]:m.offsets[f]+len(row)], row)
	}
	eb := mbdSoftplus(mbdDot(phi, m.WB))
	ea := mbdSoftplus(mbdDot(phi, m.WU))
	S := eb + ea + 2.0
	b := eb / S
	d := ea / S
	u := 2.0 / S
	if u < m.UMin {
		u = m.UMin
	}
	return mbdOpinion{B: b, D: d, U: u}
}

// mbdDesignRow computes the B-spline basis row like sklearn SplineTransformer(extrapolation="constant"):
//
//	x < xmin -> only the first degree columns are set to basis(xmin)[:degree], all others are 0
//	x > xmax -> only the last degree columns are set to basis(xmax)[n-degree:], all others are 0
//	otherwise -> full basis row using the Cox-de Boor recursion
//
// where xmin = t[degree] and xmax = t[len(t) - degree - 1].
func mbdDesignRow(x float64, t []float64, k int) []float64 {
	n := len(t) - k - 1
	xmin, xmax := t[k], t[n]
	row := make([]float64, n)
	switch {
	case x < xmin:
		fmin := mbdCoxDeBoor(xmin, t, k)
		copy(row[:k], fmin[:k])
	case x > xmax:
		fmax := mbdCoxDeBoor(xmax, t, k)
		copy(row[n-k:], fmax[n-k:])
	default:
		return mbdCoxDeBoor(x, t, k)
	}
	return row
}

// mbdCoxDeBoor evaluates the full basis row for x in [t[k], t[n]] (NURBS book FindSpan A2.1 + BasisFuns A2.2)
func mbdCoxDeBoor(x float64, t []float64, k int) []float64 {
	n := len(t) - k - 1
	hi := t[n]

	var span int
	if x >= hi {
		span = n - 1
	} else {
		low, high := k, n
		for high-low > 1 {
			mid := (low + high) / 2
			if x < t[mid] {
				high = mid
			} else {
				low = mid
			}
		}
		span = low
	}

	N := make([]float64, k+1)
	left := make([]float64, k+1)
	right := make([]float64, k+1)
	N[0] = 1.0
	for j := 1; j <= k; j++ {
		left[j] = x - t[span+1-j]
		right[j] = t[span+j] - x
		saved := 0.0
		for r := 0; r < j; r++ {
			denom := right[r+1] + left[j-r]
			var temp float64
			if denom != 0 {
				temp = N[r] / denom
			}
			N[r] = saved + right[r+1]*temp
			saved = left[j-r] * temp
		}
		N[j] = saved
	}

	row := make([]float64, n)
	copy(row[span-k:span+1], N)
	return row
}

func mbdSoftplus(z float64) float64 {
	if z > mbdSoftplusThreshold {
		return z
	}
	return math.Log1p(math.Exp(z))
}

func mbdDot(a, b []float64) float64 {
	s := 0.0
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}
