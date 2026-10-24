package tlee

import (
	"log/slog"
	"math"
	"testing"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/trustmodel/trustmodelstructure"
)

func mustOpinion(t *testing.T, b, d, u, a float64) subjectivelogic.Opinion {
	t.Helper()
	o, err := subjectivelogic.NewOpinion(b, d, u, a)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// runGraph runs a graph with a direct edge V_ego->C and a path V_ego->MEC->V_1->C, fused on the edge V_ego->C.
func runGraph(t *testing.T, operator trustmodelstructure.FusionOperator, egoEntry trustmodelstructure.AdjacencyListEntry, direct, mecToV subjectivelogic.Opinion) (map[string]subjectivelogic.QueryableOpinion, error) {
	t.Helper()
	full := mustOpinion(t, 1, 0, 0, 0.5)
	structure := trustmodelstructure.NewTrustGraphDTO(operator, trustmodelstructure.OppositeBeliefDiscount, []trustmodelstructure.AdjacencyListEntry{
		egoEntry,
		trustmodelstructure.NewAdjacencyEntryDTO("V_1", []string{"C"}),
		trustmodelstructure.NewAdjacencyEntryDTO("MEC", []string{"V_1"}),
	})
	values := map[string][]trustmodelstructure.TrustRelationship{
		"C": {
			trustmodelstructure.NewTrustRelationshipDTO("V_1", "C", &full),
			trustmodelstructure.NewTrustRelationshipDTO("V_ego", "C", &direct),
			trustmodelstructure.NewTrustRelationshipDTO("MEC", "V_1", &mecToV),
			trustmodelstructure.NewTrustRelationshipDTO("V_ego", "MEC", &full),
		},
	}
	return SpawnNewTLEE(slog.Default()).RunTLEE("test", 0, 0, structure, values)
}

// expectWeightedMean checks that the result on C is the weighted mean of the direct opinion and the discounted path.
func expectWeightedMean(t *testing.T, results map[string]subjectivelogic.QueryableOpinion, direct, mecToV subjectivelogic.Opinion, weightDirect float64) {
	t.Helper()
	full := mustOpinion(t, 1, 0, 0, 0.5)
	path, err := subjectivelogic.MultiEdgeTrustDisc([]subjectivelogic.Opinion{full, mecToV, full})
	if err != nil {
		t.Fatal(err)
	}
	result := results["C"]
	expected := []float64{
		weightDirect*direct.Belief() + (1-weightDirect)*path.Belief(),
		weightDirect*direct.Disbelief() + (1-weightDirect)*path.Disbelief(),
		weightDirect*direct.Uncertainty() + (1-weightDirect)*path.Uncertainty(),
	}
	actual := []float64{result.Belief(), result.Disbelief(), result.Uncertainty()}
	for i := range expected {
		if math.Abs(expected[i]-actual[i]) > 1e-9 {
			t.Fatalf("expected %v, got %v", expected, actual)
		}
	}
}

func TestProportionalFusion(t *testing.T) {
	direct := mustOpinion(t, 0.6, 0.3, 0.1, 0.5)
	mecToV := mustOpinion(t, 0.2, 0.5, 0.3, 0.5)

	// weights are normalized, so 3:1 corresponds to 0.75:0.25
	egoEntry := trustmodelstructure.NewWeightedAdjacencyEntryDTO("V_ego", map[string]float64{"C": 3, "MEC": 1})
	results, err := runGraph(t, trustmodelstructure.ProportionalFusion, egoEntry, direct, mecToV)
	if err != nil {
		t.Fatal(err)
	}
	expectWeightedMean(t, results, direct, mecToV, 0.75)
}

func TestProportionalFusionDefaultWeights(t *testing.T) {
	direct := mustOpinion(t, 0.6, 0.3, 0.1, 0.5)
	mecToV := mustOpinion(t, 0.2, 0.5, 0.3, 0.5)

	egoEntry := trustmodelstructure.NewAdjacencyEntryDTO("V_ego", []string{"C", "MEC"})
	results, err := runGraph(t, trustmodelstructure.ProportionalFusion, egoEntry, direct, mecToV)
	if err != nil {
		t.Fatal(err)
	}
	expectWeightedMean(t, results, direct, mecToV, 0.5)
}

func TestProportionalFusionInvalidWeights(t *testing.T) {
	direct := mustOpinion(t, 0.6, 0.3, 0.1, 0.5)
	mecToV := mustOpinion(t, 0.2, 0.5, 0.3, 0.5)

	tests := map[string]trustmodelstructure.AdjacencyListEntry{
		"zero sum":     trustmodelstructure.NewWeightedAdjacencyEntryDTO("V_ego", map[string]float64{"C": 0, "MEC": 0}),
		"negative":     trustmodelstructure.NewWeightedAdjacencyEntryDTO("V_ego", map[string]float64{"C": -1, "MEC": 2}),
		"missing edge": trustmodelstructure.NewAdjacencyEntryDTO("V_ego", []string{"C"}),
	}
	for name, egoEntry := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := runGraph(t, trustmodelstructure.ProportionalFusion, egoEntry, direct, mecToV); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestWeightsIgnoredByOtherOperators(t *testing.T) {
	direct := mustOpinion(t, 0.6, 0.3, 0.1, 0.5)
	mecToV := mustOpinion(t, 0.2, 0.5, 0.3, 0.5)

	unweighted, err := runGraph(t, trustmodelstructure.CumulativeFusion, trustmodelstructure.NewAdjacencyEntryDTO("V_ego", []string{"C", "MEC"}), direct, mecToV)
	if err != nil {
		t.Fatal(err)
	}
	weighted, err := runGraph(t, trustmodelstructure.CumulativeFusion, trustmodelstructure.NewWeightedAdjacencyEntryDTO("V_ego", map[string]float64{"C": 3, "MEC": 1}), direct, mecToV)
	if err != nil {
		t.Fatal(err)
	}
	if unweighted["C"].Belief() != weighted["C"].Belief() || unweighted["C"].Disbelief() != weighted["C"].Disbelief() || unweighted["C"].Uncertainty() != weighted["C"].Uncertainty() {
		t.Fatalf("weights changed result of CumulativeFusion: %v vs %v", unweighted["C"], weighted["C"])
	}
}
