// Package rf implements a multi-output Random Forest Regressor in pure Go.
// Bootstrap sampling, all features per split, squared-error criterion; leaf size and depth are parameters.
package rf

import (
	"encoding/json"
	"math/rand"
	"os"
	"sort"
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

// Params are the training hyperparameters.
type Params struct {
	NEstimators    int   // number of trees
	MinSamplesLeaf int   // minimum samples per leaf
	MaxDepth       int   // 0 = unlimited
	Seed           int64 // reproducibility
}

// Train fits the forest. X: [n][features], Y: [n][outputs].
func Train(X, Y [][]float64, p Params) *Forest {
	rng := rand.New(rand.NewSource(p.Seed))
	f := &Forest{NFeatures: len(X[0]), NOutputs: len(Y[0])}
	n := len(X)
	for t := 0; t < p.NEstimators; t++ {
		// Bootstrap sample with replacement.
		idx := make([]int, n)
		for i := range idx {
			idx[i] = rng.Intn(n)
		}
		b := &builder{X: X, Y: Y, p: p}
		b.build(idx, 0)
		f.Trees = append(f.Trees, Tree{Nodes: b.nodes})
	}
	return f
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

// Save writes the forest as JSON.
func (f *Forest) Save(path string) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Load reads a forest from JSON.
func Load(path string) (*Forest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f Forest
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// builder grows one tree.
type builder struct {
	X, Y  [][]float64
	p     Params
	nodes []Node
}

// leaf adds a leaf with the mean of the outputs.
func (b *builder) leaf(idx []int) int {
	m := make([]float64, len(b.Y[0]))
	for _, i := range idx {
		for k, v := range b.Y[i] {
			m[k] += v
		}
	}
	for k := range m {
		m[k] /= float64(len(idx))
	}
	b.nodes = append(b.nodes, Node{Feature: -1, Value: m})
	return len(b.nodes) - 1
}

// build splits recursively until no split improves the error.
func (b *builder) build(idx []int, depth int) int {
	minLeaf := max(b.p.MinSamplesLeaf, 1)
	if len(idx) < 2*minLeaf || (b.p.MaxDepth > 0 && depth >= b.p.MaxDepth) {
		return b.leaf(idx)
	}
	feat, thr, ok := b.bestSplit(idx, minLeaf)
	if !ok {
		return b.leaf(idx)
	}
	var L, R []int
	for _, i := range idx {
		if b.X[i][feat] <= thr {
			L = append(L, i)
		} else {
			R = append(R, i)
		}
	}
	if len(L) == 0 || len(R) == 0 {
		return b.leaf(idx)
	}
	me := len(b.nodes)
	b.nodes = append(b.nodes, Node{Feature: feat, Threshold: thr})
	l := b.build(L, depth+1)
	r := b.build(R, depth+1)
	b.nodes[me].Left, b.nodes[me].Right = l, r
	return me
}

// bestSplit finds the cut that minimizes the summed squared error over all outputs.
func (b *builder) bestSplit(idx []int, minLeaf int) (int, float64, bool) {
	n := len(idx)
	K := len(b.Y[0])
	totS, totQ := make([]float64, K), make([]float64, K)
	for _, i := range idx {
		for k, v := range b.Y[i] {
			totS[k] += v
			totQ[k] += v * v
		}
	}
	best := sse(totS, totQ, n)
	bestF, bestT := -1, 0.0

	sorted := make([]int, n)
	lS, lQ := make([]float64, K), make([]float64, K)
	rS, rQ := make([]float64, K), make([]float64, K)

	for f := 0; f < len(b.X[0]); f++ {
		copy(sorted, idx)
		sort.Slice(sorted, func(a, c int) bool { return b.X[sorted[a]][f] < b.X[sorted[c]][f] })
		for k := 0; k < K; k++ {
			lS[k], lQ[k] = 0, 0
		}
		for j := 0; j < n-1; j++ {
			i := sorted[j]
			for k, v := range b.Y[i] {
				lS[k] += v
				lQ[k] += v * v
			}
			nl, nr := j+1, n-j-1
			if nl < minLeaf || nr < minLeaf {
				continue
			}
			x0, x1 := b.X[i][f], b.X[sorted[j+1]][f]
			if !(x0 < x1) {
				continue
			}
			for k := 0; k < K; k++ {
				rS[k], rQ[k] = totS[k]-lS[k], totQ[k]-lQ[k]
			}
			if s := sse(lS, lQ, nl) + sse(rS, rQ, nr); s < best-1e-12 {
				best, bestF, bestT = s, f, midpoint(x0, x1)
			}
		}
	}
	return bestF, bestT, bestF >= 0
}

// sse is the sum of squared errors from running sums.
func sse(s, q []float64, n int) float64 {
	var t float64
	for k := range s {
		t += q[k] - s[k]*s[k]/float64(n)
	}
	return t
}

// midpoint returns a threshold t with x0 <= t < x1, so that the split keeps x0 left and x1 right;
// (x0+x1)/2 rounds to x1 when x0 and x1 are adjacent floats.
func midpoint(x0, x1 float64) float64 {
	if t := x0 + (x1-x0)/2; t < x1 {
		return t
	}
	return x0
}
