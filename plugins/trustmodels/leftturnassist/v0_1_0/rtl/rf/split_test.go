package rf

import (
	"math"
	"testing"
)

// Adjacent float values whose midpoint rounds to the larger one must not lead to an empty side.
func TestAdjacentValuesSplit(t *testing.T) {
	a := 0.3
	b := math.Nextafter(a, 1)
	var X, Y [][]float64
	for i := 0; i < 400; i++ {
		x := a
		if i%2 == 1 {
			x = b
		}
		X = append(X, []float64{x, x, x})
		Y = append(Y, []float64{float64(i % 2), 0, 0})
	}
	f := Train(X, Y, Params{NEstimators: 3, MinSamplesLeaf: 50, Seed: 1})
	for _, tr := range f.Trees {
		for _, n := range tr.Nodes {
			if n.Feature == -1 && len(n.Value) == 0 {
				t.Fatal("empty leaf")
			}
			for _, v := range n.Value {
				if math.IsNaN(v) {
					t.Fatal("NaN leaf")
				}
			}
		}
	}
}
