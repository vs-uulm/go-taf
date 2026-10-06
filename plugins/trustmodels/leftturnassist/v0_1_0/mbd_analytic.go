package leftturnassist_v0_1_0

// Analytic (F1-optimized) quantification approach for MBD evidence.
// Ported from code_quantification/hyperparameter/final/sltrust.

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/vs-uulm/go-taf/pkg/core"
)

const mbdNumFeatures = 6

var mbdFeatureOrder = [mbdNumFeatures]string{
	"pos_dist", "speed_dist", "acc_dist", "road_edge", "time_err", "heading",
}

const (
	mbdEpsFuse = 1e-12
	mbdEpsNorm = 1e-12
)

// mbdOpinion is a binomial SL opinion with base rate 0.5
type mbdOpinion struct {
	B float64
	D float64
	U float64
}

type mbdFusionOp string

const (
	mbdFusionCBF mbdFusionOp = "cbf"
	mbdFusionAVG mbdFusionOp = "avg"
	mbdFusionWBF mbdFusionOp = "wbf"
)

type mbdSLParams struct {
	Thr  [mbdNumFeatures]float64
	AlpB [mbdNumFeatures]float64
	BetB [mbdNumFeatures]float64
	AlpD [mbdNumFeatures]float64
	BetD [mbdNumFeatures]float64

	Fusion      mbdFusionOp
	DecisionThr float64
}

// mbdSLModel represents the parameters JSON (e.g. content of parameters_f1_only.json)
type mbdSLModel struct {
	Variant      string    `json:"variant"`
	FeatureOrder []string  `json:"feature_order"`
	AlpB         []float64 `json:"alpB"`
	BetB         []float64 `json:"betB"`
	AlpD         []float64 `json:"alpD"`
	BetD         []float64 `json:"betD"`
	Thr          []float64 `json:"thr"`
	Trust        []float64 `json:"trust"`
	FusionOp     string    `json:"fusion_op"`
	DecisionThr  float64   `json:"decision_thr"`
}

// loadMbdSLParams parses the parameters JSON (content of e.g. parameters_f1_only.json) and converts it into mbdSLParams
func loadMbdSLParams(rawJSON string) (mbdSLParams, error) {
	var p mbdSLParams
	var m mbdSLModel
	if err := json.Unmarshal([]byte(rawJSON), &m); err != nil {
		return p, fmt.Errorf("parsing MBD quantification parameters: %w", err)
	}
	for name, arr := range map[string][]float64{
		"alpB": m.AlpB, "betB": m.BetB, "alpD": m.AlpD, "betD": m.BetD, "thr": m.Thr,
	} {
		if len(arr) != mbdNumFeatures {
			return p, fmt.Errorf("field %q has length %d, expected %d", name, len(arr), mbdNumFeatures)
		}
	}
	if m.FeatureOrder != nil {
		if len(m.FeatureOrder) != mbdNumFeatures {
			return p, fmt.Errorf("feature_order has length %d, expected %d", len(m.FeatureOrder), mbdNumFeatures)
		}
		for i, name := range m.FeatureOrder {
			if name != mbdFeatureOrder[i] {
				return p, fmt.Errorf("unexpected feature_order[%d] %q, expected %q", i, name, mbdFeatureOrder[i])
			}
		}
	}
	if len(m.Trust) != 0 {
		return p, fmt.Errorf("trust discounting is not supported")
	}

	copy(p.Thr[:], m.Thr)
	copy(p.AlpB[:], m.AlpB)
	copy(p.BetB[:], m.BetB)
	copy(p.AlpD[:], m.AlpD)
	copy(p.BetD[:], m.BetD)

	switch mbdFusionOp(m.FusionOp) {
	case mbdFusionCBF, mbdFusionAVG, mbdFusionWBF:
		p.Fusion = mbdFusionOp(m.FusionOp)
	default:
		return p, fmt.Errorf("unknown fusion operator %q", m.FusionOp)
	}

	p.DecisionThr = m.DecisionThr
	if p.DecisionThr == 0 {
		p.DecisionThr = 0.5
	}
	return p, nil
}

// mbdFeatures derives the six model features from the raw MBD evidence, following
// generate_feature_output_abs.py of the MBD. The MBD evidence values are the absolute
// prediction errors |y_hat - y| per output dimension (sin/cos normalized to [0, 1]).
// Vector-valued errors are reduced to their L2 norm, the heading error to the angular
// error on the unit circle normalized to [0, 1].
func mbdFeatures(m map[core.EvidenceType]interface{}) ([mbdNumFeatures]float64, error) {
	var features [mbdNumFeatures]float64
	values := make(map[core.EvidenceType]float64, len(mbdEvidence))
	for _, evidenceType := range mbdEvidence {
		raw, ok := m[evidenceType]
		if !ok {
			return features, fmt.Errorf("missing evidence %s", evidenceType)
		}
		v, ok := raw.(float64)
		if !ok {
			return features, fmt.Errorf("evidence %s has unexpected type %T", evidenceType, raw)
		}
		values[evidenceType] = v
	}

	features[0] = math.Hypot(values[core.MBD_RELATIVE_POSITION_ERROR_X], values[core.MBD_RELATIVE_POSITION_ERROR_Y])
	features[1] = math.Hypot(values[core.MBD_SENDER_SPEED_ERROR_X], values[core.MBD_SENDER_SPEED_ERROR_Y])
	features[2] = math.Hypot(values[core.MBD_SENDER_ACCELERATION_ERROR_X], values[core.MBD_SENDER_ACCELERATION_ERROR_Y])
	features[3] = math.Abs(values[core.MBD_DISTANCE_TO_ROAD_EDGE_ERROR])
	features[4] = math.Abs(values[core.MBD_RECEIVER_TIME_ERROR])
	features[5] = mbdAngularError(values[core.MBD_SENDER_HEADING_ERROR_SIN], values[core.MBD_SENDER_HEADING_ERROR_COS])

	// Same sanitizing as in the training data pipeline
	for i, v := range features {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			features[i] = 0.0
		}
	}
	return features, nil
}

// mbdAngularError reconstructs acos(dot(pred, true)) / pi from the absolute errors of the
// normalized sin/cos components. Denormalizing ((v*2)-1) doubles the errors, and for unit
// vectors |pred - true|^2 = 2 - 2*dot(pred, true) holds. Exact if the predicted sin/cos
// form a unit vector, otherwise an approximation.
func mbdAngularError(sinErr, cosErr float64) float64 {
	dot := 1.0 - 2.0*(sinErr*sinErr+cosErr*cosErr)
	dot = math.Max(-1.0, math.Min(1.0, dot))
	return math.Acos(dot) / math.Pi
}

func (p mbdSLParams) computeTrustOpinion(features [mbdNumFeatures]float64) mbdOpinion {
	fuse := mbdFuserFor(p.Fusion)

	acc := mbdFeatureOpinion(features[0], p.Thr[0], p.AlpB[0], p.BetB[0], p.AlpD[0], p.BetD[0])
	for k := 1; k < mbdNumFeatures; k++ {
		op := mbdFeatureOpinion(features[k], p.Thr[k], p.AlpB[k], p.BetB[k], p.AlpD[k], p.BetD[k])
		acc = fuse(acc, op)
	}
	return acc
}

// mbdFeatureOpinion computes the opinion for a single MBD feature
func mbdFeatureOpinion(x, thr, alpB, betB, alpD, betD float64) mbdOpinion {
	delta := x - thr
	deltaSq := delta * delta
	if x <= thr {
		b := alpB * (1.0 - math.Exp(-betB*deltaSq))
		return mbdOpinion{B: b, D: 0.0, U: 1.0 - b}
	}
	d := alpD * (1.0 - math.Exp(-betD*deltaSq))
	return mbdOpinion{B: 0.0, D: d, U: 1.0 - d}
}

type mbdFuseFn func(a, b mbdOpinion) mbdOpinion

func mbdFuserFor(op mbdFusionOp) mbdFuseFn {
	switch op {
	case mbdFusionAVG:
		return mbdFuseAVG
	case mbdFusionWBF:
		return mbdFuseWBF
	default:
		return mbdFuseCBF
	}
}

func mbdNormalize(b, d, u float64) mbdOpinion {
	s := b + d + u
	if s > mbdEpsNorm {
		return mbdOpinion{B: b / s, D: d / s, U: u / s}
	}
	return mbdOpinion{B: 0.0, D: 0.0, U: 1.0}
}

func mbdFuseCBF(o1, o2 mbdOpinion) mbdOpinion {
	k := o1.U + o2.U - o1.U*o2.U
	kSafe := math.Max(k, mbdEpsFuse)
	b := (o1.B*o2.U + o2.B*o1.U) / kSafe
	d := (o1.D*o2.U + o2.D*o1.U) / kSafe
	u := (o1.U * o2.U) / kSafe
	return mbdNormalize(b, d, u)
}

func mbdFuseAVG(o1, o2 mbdOpinion) mbdOpinion {
	k := o1.U + o2.U
	if k < mbdEpsFuse {
		return mbdNormalize(0.5*(o1.B+o2.B), 0.5*(o1.D+o2.D), 0.0)
	}
	b := (o1.B*o2.U + o2.B*o1.U) / k
	d := (o1.D*o2.U + o2.D*o1.U) / k
	u := (2.0 * o1.U * o2.U) / k
	return mbdNormalize(b, d, u)
}

func mbdFuseWBF(o1, o2 mbdOpinion) mbdOpinion {
	c1 := 1.0 - o1.U
	c2 := 1.0 - o2.U
	k := o1.U + o2.U - 2.0*o1.U*o2.U
	if k < mbdEpsFuse {
		gamma := math.Max(c1+c2, mbdEpsFuse)
		return mbdNormalize((c1*o1.B+c2*o2.B)/gamma, (c1*o1.D+c2*o2.D)/gamma, 0.0)
	}
	b := (o1.B*c1*o2.U + o2.B*c2*o1.U) / k
	d := (o1.D*c1*o2.U + o2.D*c2*o1.U) / k
	u := ((2.0 - o1.U - o2.U) * o1.U * o2.U) / k
	return mbdNormalize(b, d, u)
}
