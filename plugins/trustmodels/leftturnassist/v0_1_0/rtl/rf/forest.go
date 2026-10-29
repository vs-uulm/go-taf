// Package rf evaluates a multi-output Random Forest Regressor in pure Go.
// The forests are trained outside the TAF (rtl-trainer of the evaluation repository), which writes them in this format.
package rf

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Node is a tree node. Feature == -1 marks a leaf; Value holds the mean outputs.
type Node struct {
	Feature   int       `json:"f"`
	Threshold float64   `json:"t"`
	Left      int       `json:"l"`
	Right     int       `json:"r"`
	Value     []float64 `json:"v,omitempty"`
}

// Tree is a flat list of nodes; index 0 is the root.
type Tree struct {
	Nodes []Node `json:"nodes"`
}

// Forest is the trained model.
type Forest struct {
	NFeatures int    `json:"n_features"`
	NOutputs  int    `json:"n_outputs"`
	Trees     []Tree `json:"trees"`
}

// Predict returns the mean prediction over all trees.
func (f *Forest) Predict(x []float64) []float64 {
	out := make([]float64, f.NOutputs)
	for _, t := range f.Trees {
		n := 0
		for t.Nodes[n].Feature >= 0 {
			nd := &t.Nodes[n]
			if x[nd.Feature] <= nd.Threshold {
				n = nd.Left
			} else {
				n = nd.Right
			}
		}
		for k, v := range t.Nodes[n].Value {
			out[k] += v
		}
	}
	for k := range out {
		out[k] /= float64(len(f.Trees))
	}
	return out
}

// Parse reads a forest from JSON and checks its structure, so that Predict cannot fail on it.
func Parse(data []byte) (*Forest, error) {
	var f Forest
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if err := f.validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

// validate checks the dimensions and that every tree is a valid pre-order tree: children follow their parent (as the
// trainer writes them), so Predict always reaches a leaf.
func (f *Forest) validate() error {
	if f.NFeatures < 1 || f.NOutputs < 1 {
		return fmt.Errorf("invalid dimensions: n_features = %d, n_outputs = %d", f.NFeatures, f.NOutputs)
	}
	if len(f.Trees) == 0 {
		return errors.New("no trees")
	}
	for t, tree := range f.Trees {
		if len(tree.Nodes) == 0 {
			return fmt.Errorf("tree %d has no nodes", t)
		}
		for i, n := range tree.Nodes {
			switch {
			case n.Feature == -1:
				if len(n.Value) != f.NOutputs {
					return fmt.Errorf("tree %d, node %d: leaf has %d values, want %d", t, i, len(n.Value), f.NOutputs)
				}
			case n.Feature < 0 || n.Feature >= f.NFeatures:
				return fmt.Errorf("tree %d, node %d: invalid feature %d", t, i, n.Feature)
			case n.Left <= i || n.Right <= i || n.Left >= len(tree.Nodes) || n.Right >= len(tree.Nodes):
				return fmt.Errorf("tree %d, node %d: invalid children %d, %d", t, i, n.Left, n.Right)
			}
		}
	}
	return nil
}
