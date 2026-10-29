// Package rtl holds the RTL decision logic of the TAF.
//   - ATL: Subjective Logic opinion (b + d + u = 1).
//   - RTL: independent thresholds (bt, dt, ut in [0,1], no sum constraint).
//   - Decide: accept if b >= bt && d <= dt && u <= ut.
//
// The Random Forest models are trained outside the TAF (rtl-trainer of the evaluation repository); the TAF only
// parses them and predicts the RTL.
package rtl

import (
	"fmt"
	"math"

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

// Clamp limits each threshold to [0,1] independently.
func (r RTL) Clamp() RTL {
	c := func(v float64) float64 { return math.Max(0, math.Min(1, v)) }
	return RTL{c(r.Bt), c(r.Dt), c(r.Ut)}
}

// Decide applies the component rule: b >= bt, d <= dt, u <= ut.
func Decide(a ATL, r RTL) bool {
	return a.B >= r.Bt && a.D <= r.Dt && a.U <= r.Ut
}

// Model is the trained RF that predicts the RTL from the ATL.
type Model struct {
	forest *rf.Forest
}

// ParseModel reads the RF model from JSON (rf_model.json of the trainer).
func ParseModel(data []byte) (*Model, error) {
	f, err := rf.Parse(data)
	if err != nil {
		return nil, err
	}
	if f.NFeatures != 3 || f.NOutputs != 3 {
		return nil, fmt.Errorf("RF model maps %d features to %d outputs, want 3 to 3", f.NFeatures, f.NOutputs)
	}
	return &Model{forest: f}, nil
}

// PredictRTL returns the trained RTL for an ATL.
func (m *Model) PredictRTL(a ATL) RTL {
	p := m.forest.Predict([]float64{a.B, a.D, a.U})
	return RTL{p[0], p[1], p[2]}.Clamp()
}
