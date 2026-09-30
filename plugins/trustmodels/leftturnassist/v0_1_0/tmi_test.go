package leftturnassist_v0_1_0

import (
	"log/slog"
	"math"
	"testing"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/tlee"
)

func TestProportionalFusionWeightEgo(t *testing.T) {
	spawner := NewDynamicTrustModelTemplateSpawner(TrustModelTemplate{}, map[string]string{
		"FUSION_OPERATOR":                "Proportional",
		"PROPORTIONAL_FUSION_WEIGHT_EGO": "0.75",
	})
	instance, err := spawner.OnNewVehicle("1", nil)
	if err != nil {
		t.Fatal(err)
	}
	tmi := instance.(*TrustModelInstance)
	tmi.Initialize(map[string]interface{}{})

	direct, _ := subjectivelogic.NewOpinion(0.6, 0.3, 0.1, 0.5)
	tch, _ := subjectivelogic.NewOpinion(0.7, 0.1, 0.2, 0.5)
	ntm, _ := subjectivelogic.NewOpinion(0.2, 0.5, 0.3, 0.5)
	tmi.mbdOpinion = &direct
	tmi.tchOpinion = &tch
	tmi.ntmOpinion = &ntm
	tmi.updateValues()

	results, err := tlee.SpawnNewTLEE(slog.Default()).RunTLEE(tmi.ID(), tmi.Version(), tmi.Fingerprint(), tmi.Structure(), tmi.Values())
	if err != nil {
		t.Fatal(err)
	}

	// V_ego -> V_x -> C_x_x, discounted with the default (base rate sensitive) discounting operator
	path, err := subjectivelogic.TrustDiscounting(&tch, &ntm)
	if err != nil {
		t.Fatal(err)
	}
	result := results[objectIdentifier(tmi.targetVehicleID, tmi.targetVehicleID)]
	expected := []float64{
		0.75*direct.Belief() + 0.25*path.Belief(),
		0.75*direct.Disbelief() + 0.25*path.Disbelief(),
		0.75*direct.Uncertainty() + 0.25*path.Uncertainty(),
	}
	actual := []float64{result.Belief(), result.Disbelief(), result.Uncertainty()}
	for i := range expected {
		if math.Abs(expected[i]-actual[i]) > 1e-9 {
			t.Fatalf("expected %v, got %v", expected, actual)
		}
	}
}

func TestProportionalFusionWeightEgoMissing(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing PROPORTIONAL_FUSION_WEIGHT_EGO")
		}
	}()
	spawner := NewDynamicTrustModelTemplateSpawner(TrustModelTemplate{}, map[string]string{"FUSION_OPERATOR": "Proportional"})
	spawner.OnNewVehicle("1", nil)
}
