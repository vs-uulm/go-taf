package leftturnassist_v0_1_0

// MLP quantification approach for MBD evidence.
// Ported from code_quantification/mlp/golang/mlptrust.go.

import (
	"encoding/json"
	"fmt"
)

type mbdMlpLayer struct {
	W [][]float64 `json:"W"`
	B []float64   `json:"b"`
}

// mbdMlpModel represents the MLP model JSON (content of mlp_model.json)
type mbdMlpModel struct {
	Features     []string      `json:"features"`
	FeatureMeans []float64     `json:"feature_means"`
	FeatureStds  []float64     `json:"feature_stds"`
	UMin         float64       `json:"u_min"`
	Layers       []mbdMlpLayer `json:"layers"`
}

// loadMbdMlpModel parses and validates the MLP model JSON (content of mlp_model.json)
func loadMbdMlpModel(rawJSON string) (*mbdMlpModel, error) {
	var m mbdMlpModel
	if err := json.Unmarshal([]byte(rawJSON), &m); err != nil {
		return nil, fmt.Errorf("parsing MBD MLP model: %w", err)
	}
	if err := m.finalize(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *mbdMlpModel) finalize() error {
	if len(m.Features) != mbdNumFeatures {
		return fmt.Errorf("features has length %d, expected %d", len(m.Features), mbdNumFeatures)
	}
	for i, name := range m.Features {
		if name != mbdModelFeatureOrder[i] {
			return fmt.Errorf("unexpected features[%d] %q, expected %q", i, name, mbdModelFeatureOrder[i])
		}
	}
	if len(m.FeatureMeans) != mbdNumFeatures || len(m.FeatureStds) != mbdNumFeatures {
		return fmt.Errorf("feature_means/feature_stds length != %d", mbdNumFeatures)
	}
	if len(m.Layers) == 0 {
		return fmt.Errorf("no layers")
	}
	if m.UMin == 0 {
		m.UMin = 1e-3
	}
	// Dimension check: first layer input == number of features, layers chain, last output == 2
	inDim := mbdNumFeatures
	for li, L := range m.Layers {
		out := len(L.W)
		if out == 0 {
			return fmt.Errorf("layer %d: W is empty", li)
		}
		if len(L.B) != out {
			return fmt.Errorf("layer %d: b length %d != out %d", li, len(L.B), out)
		}
		for r := range L.W {
			if len(L.W[r]) != inDim {
				return fmt.Errorf("layer %d, row %d: input dimension %d != %d", li, r, len(L.W[r]), inDim)
			}
		}
		inDim = out
	}
	if inDim != 2 {
		return fmt.Errorf("last layer must have 2 outputs, has %d", inDim)
	}
	return nil
}

func (m *mbdMlpModel) computeTrustOpinion(features [mbdNumFeatures]float64) mbdOpinion {
	h := make([]float64, mbdNumFeatures)
	for j := range features {
		h[j] = (features[j] - m.FeatureMeans[j]) / m.FeatureStds[j]
	}
	last := len(m.Layers) - 1
	for li := 0; li < last; li++ {
		h = mbdLinear(m.Layers[li], h)
		for i := range h {
			if h[i] < 0 {
				h[i] = 0
			}
		}
	}
	out := mbdLinear(m.Layers[last], h) // 2 outputs, no ReLU
	eB, eA := mbdSoftplus(out[0]), mbdSoftplus(out[1])

	S := eB + eA + 2.0
	u := 2.0 / S
	if u < m.UMin {
		u = m.UMin
	}
	return mbdOpinion{B: eB / S, D: eA / S, U: u}
}

func mbdLinear(L mbdMlpLayer, h []float64) []float64 {
	out := make([]float64, len(L.W))
	for i, row := range L.W {
		s := L.B[i]
		for j := range row {
			s += row[j] * h[j]
		}
		out[i] = s
	}
	return out
}
