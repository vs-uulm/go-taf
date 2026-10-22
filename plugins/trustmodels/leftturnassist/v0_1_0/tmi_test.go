package leftturnassist_v0_1_0

import (
	"log/slog"
	"math"
	"testing"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/core"
	"github.com/vs-uulm/go-taf/pkg/tlee"
	"github.com/vs-uulm/go-taf/pkg/trustmodel/trustmodelupdate"
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
	cam, _ := subjectivelogic.NewOpinion(0.2, 0.5, 0.3, 0.5)
	tmi.mbdOpinion = &direct
	tmi.tchOpinion = &tch
	tmi.camOpinion = &cam
	tmi.updateValues()

	results, err := tlee.SpawnNewTLEE(slog.Default()).RunTLEE(tmi.ID(), tmi.Version(), tmi.Fingerprint(), tmi.Structure(), tmi.Values())
	if err != nil {
		t.Fatal(err)
	}

	// V_ego -> V_x -> C_x_x, discounted with the default (base rate sensitive) discounting operator
	path, err := subjectivelogic.TrustDiscounting(&tch, &cam)
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

func TestInitializeKeepsTargetVehicleID(t *testing.T) {
	spawner := NewDynamicTrustModelTemplateSpawner(TrustModelTemplate{}, nil)
	instance, err := spawner.OnNewVehicle("1", nil)
	if err != nil {
		t.Fatal(err)
	}
	tmi := instance.(*TrustModelInstance)
	tmi.Initialize(map[string]interface{}{"SourceId": "42"})

	if _, exists := tmi.Values()[objectIdentifier("42", "42")]; !exists {
		t.Fatalf("expected scope %s, got %v", objectIdentifier("42", "42"), tmi.Values())
	}

	opinion, _ := subjectivelogic.NewOpinion(0.7, 0.1, 0.2, 0.5)
	if !tmi.Update(trustmodelupdate.CreateAtomicTrustOpinionUpdate(&opinion, vehicleIdentifier("ego"), vehicleIdentifier("42"), core.TCH)) {
		t.Fatal("expected update for target vehicle to be applied")
	}
}

func TestMbdWindowMean(t *testing.T) {
	spawner := NewDynamicTrustModelTemplateSpawner(TrustModelTemplate{}, map[string]string{"MBD_WINDOW_SIZE": "3"})
	instance, err := spawner.OnNewVehicle("1", nil)
	if err != nil {
		t.Fatal(err)
	}
	tmi := instance.(*TrustModelInstance)
	tmi.Initialize(map[string]interface{}{})

	trusted, _ := subjectivelogic.NewOpinion(0.9, 0.0, 0.1, 0.5)
	outlier, _ := subjectivelogic.NewOpinion(0.0, 0.9, 0.1, 0.5)
	observation := objectIdentifier(tmi.targetVehicleID, tmi.targetVehicleID)
	expected := [][]float64{
		{0.9, 0.0, 0.1},
		{0.9, 0.0, 0.1},
		{0.6, 0.3, 0.1},
		{0.6, 0.3, 0.1}, // oldest dropped
		{0.3, 0.6, 0.1},
	}
	for i, opinion := range []*subjectivelogic.Opinion{&trusted, &trusted, &outlier, &trusted, &outlier} {
		tmi.Update(trustmodelupdate.CreateAtomicTrustOpinionUpdate(opinion, vehicleIdentifier("ego"), observation, core.MBD))
		actual := []float64{tmi.mbdOpinion.Belief(), tmi.mbdOpinion.Disbelief(), tmi.mbdOpinion.Uncertainty()}
		for k := range actual {
			if math.Abs(expected[i][k]-actual[k]) > 1e-9 {
				t.Fatalf("update %d: expected %v, got %v", i, expected[i], actual)
			}
		}
	}
}

func TestMbdWindowDisabledByDefault(t *testing.T) {
	spawner := NewDynamicTrustModelTemplateSpawner(TrustModelTemplate{}, nil)
	instance, err := spawner.OnNewVehicle("1", nil)
	if err != nil {
		t.Fatal(err)
	}
	tmi := instance.(*TrustModelInstance)
	tmi.Initialize(map[string]interface{}{})

	trusted, _ := subjectivelogic.NewOpinion(0.9, 0.0, 0.1, 0.5)
	outlier, _ := subjectivelogic.NewOpinion(0.0, 0.9, 0.1, 0.5)
	observation := objectIdentifier(tmi.targetVehicleID, tmi.targetVehicleID)
	tmi.Update(trustmodelupdate.CreateAtomicTrustOpinionUpdate(&trusted, vehicleIdentifier("ego"), observation, core.MBD))
	tmi.Update(trustmodelupdate.CreateAtomicTrustOpinionUpdate(&outlier, vehicleIdentifier("ego"), observation, core.MBD))
	if tmi.mbdOpinion.Disbelief() != outlier.Disbelief() || tmi.mbdOpinion.Belief() != outlier.Belief() {
		t.Fatalf("expected the newest opinion without windowing, got %v", tmi.mbdOpinion)
	}
}

func TestMbdWindowSizeInvalid(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for MBD_WINDOW_SIZE 0")
		}
	}()
	spawner := NewDynamicTrustModelTemplateSpawner(TrustModelTemplate{}, map[string]string{"MBD_WINDOW_SIZE": "0"})
	spawner.OnNewVehicle("1", nil)
}

// mbdUpdates feeds MBD opinions into a TMI and checks mbdOpinion (b, d, u) after each update
func mbdUpdates(t *testing.T, params map[string]string, opinions [][]float64, expected [][]float64) {
	t.Helper()
	spawner := NewDynamicTrustModelTemplateSpawner(TrustModelTemplate{}, params)
	instance, err := spawner.OnNewVehicle("1", nil)
	if err != nil {
		t.Fatal(err)
	}
	tmi := instance.(*TrustModelInstance)
	tmi.Initialize(map[string]interface{}{})
	observation := objectIdentifier(tmi.targetVehicleID, tmi.targetVehicleID)
	for i, o := range opinions {
		opinion, err := subjectivelogic.NewOpinion(o[0], o[1], o[2], 0.5)
		if err != nil {
			t.Fatal(err)
		}
		tmi.Update(trustmodelupdate.CreateAtomicTrustOpinionUpdate(&opinion, vehicleIdentifier("ego"), observation, core.MBD))
		actual := []float64{tmi.mbdOpinion.Belief(), tmi.mbdOpinion.Disbelief(), tmi.mbdOpinion.Uncertainty()}
		for k := range actual {
			if math.Abs(expected[i][k]-actual[k]) > 1e-9 {
				t.Fatalf("update %d: expected %v, got %v", i, expected[i], actual)
			}
		}
	}
}

func TestMbdWindowLinear(t *testing.T) {
	trusted := []float64{0.9, 0.0, 0.1}
	outlier := []float64{0.0, 0.9, 0.1}
	mbdUpdates(t, map[string]string{"MBD_WINDOW_WEIGHTING": "linear", "MBD_WINDOW_SIZE": "3"},
		[][]float64{trusted, trusted, outlier, trusted},
		[][]float64{
			{0.9, 0.0, 0.1},
			{0.9, 0.0, 0.1},
			{0.45, 0.45, 0.1}, // weights 1, 2, 3
			{0.6, 0.3, 0.1},   // oldest dropped
		})
}

func TestMbdWindowTemporal(t *testing.T) {
	mbdUpdates(t, map[string]string{"MBD_WINDOW_WEIGHTING": "temporal", "MBD_TEMPORAL_GAMMA": "0.5", "MBD_TEMPORAL_TAU": "0.9"},
		[][]float64{{0.9, 0.0, 0.1}, {0.0, 0.9, 0.1}, {0.9, 0.0, 0.1}, {1.0, 0.0, 0.0}, {0.0, 0.0, 1.0}},
		[][]float64{
			{0.9, 0.0, 0.1},
			{0.495, 0.405, 0.1}, // 0.9 - 0.5*(1-0.1)*(0.9-0)
			{0.495, 0.405, 0.1}, // b_new = tau
			{0.545, 0.455, 0.0}, // 0.495 - 0.5*(1-0)*(0.9-1)
			{0.0, 0.0, 1.0},     // limited to 1-u; b_t unchanged, as u_new = 1
		})
}

func TestMbdWindowTemporalStart(t *testing.T) {
	mbdUpdates(t, map[string]string{"MBD_WINDOW_WEIGHTING": "temporal", "MBD_TEMPORAL_GAMMA": "0.5", "MBD_TEMPORAL_TAU": "0.5"},
		[][]float64{{0.0, 0.01, 0.99}, {0.9, 0.0, 0.1}, {0.9, 0.0, 0.1}},
		[][]float64{
			{0.0, 0.01, 0.99}, // nearly vacuous: passed through, the window does not start
			{0.9, 0.0, 0.1},   // the window starts here
			{0.9, 0.0, 0.1},   // 0.9 - 0.5*0.9*(0.5-0.9) > 1: kept
		})
}

func TestMbdWindowTemporalUncertainOpinion(t *testing.T) {
	mbdUpdates(t, map[string]string{"MBD_WINDOW_WEIGHTING": "temporal", "MBD_TEMPORAL_GAMMA": "0.5", "MBD_TEMPORAL_TAU": "0.5"},
		[][]float64{{0.9, 0.0, 0.1}, {0.5, 0.0, 0.5}, {0.9, 0.0, 0.1}},
		[][]float64{
			{0.9, 0.0, 0.1},
			{0.5, 0.0, 0.5}, // b_t = 0.9 limited to 1-u in the result only
			{0.9, 0.0, 0.1}, // b_t = 0.9 kept, no disbelief from the uncertain opinion
		})
}

func TestMbdWindowTemporalMissingGamma(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for missing MBD_TEMPORAL_GAMMA")
		}
	}()
	spawner := NewDynamicTrustModelTemplateSpawner(TrustModelTemplate{}, map[string]string{"MBD_WINDOW_WEIGHTING": "temporal", "MBD_TEMPORAL_TAU": "0.9"})
	spawner.OnNewVehicle("1", nil)
}
