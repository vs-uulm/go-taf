package leftturnassist_v0_1_0

// Tests that RTL trust decisions panic on any configuration problem, like the other parameters of the trust model, and
// decide with the given RTL or model. Runs through OnNewVehicle, the same path the TAF uses.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/core"
)

func op(b, d, u float64) subjectivelogic.QueryableOpinion {
	o, err := subjectivelogic.NewOpinion(b, d, u, 0.5)
	if err != nil {
		panic(err)
	}
	return &o
}

// decide spawns a TMI through OnNewVehicle and runs its trust decision.
func decide(t *testing.T, params map[string]string, atl subjectivelogic.QueryableOpinion) core.TrustDecision {
	t.Helper()
	sp := NewDynamicTrustModelTemplateSpawner(CreateTrustModelTemplate("LTA", "0.1.0").(TrustModelTemplate), nil)
	tmi, err := sp.OnNewVehicle("veh_1", params)
	if err != nil {
		t.Fatal(err)
	}
	return tmi.(*TrustModelInstance).Decide("p", atl, nil)
}

// panicMessage spawns a TMI with params and returns its panic message, or "" if it does not panic.
func panicMessage(t *testing.T, params map[string]string) (msg string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint(r)
		}
	}()
	decide(t, params, probe)
	return ""
}

// probe: P = 0.70 -> default (P > 0.5) accepts; RTL (0.8, 0.225, 0.4) rejects.
var probe = op(0.45, 0.05, 0.5)

func TestPanicOnInvalidParams(t *testing.T) {
	cases := map[string]struct {
		params map[string]string
		want   string
	}{
		"missing RF model":      {map[string]string{"TRUST_DECISION": "RTL_RandomForest"}, "Missing value: TRUST_DECISION_RF_MODEL_PARAM"},
		"missing Optuna values": {map[string]string{"TRUST_DECISION": "RTL_Optuna"}, "Missing value: TRUST_DECISION_BT"},
		"risk missing DT/UT":    {map[string]string{"TRUST_DECISION": "RTL_RiskBased", "TRUST_DECISION_BT": "0.8"}, "Missing value: TRUST_DECISION_DT"},
		"risk out of range":     {map[string]string{"TRUST_DECISION": "RTL_RiskBased", "TRUST_DECISION_BT": "0.8", "TRUST_DECISION_DT": "0.2", "TRUST_DECISION_UT": "1.4"}, "Could not parse TRUST_DECISION_UT"},
		"risk not a number":     {map[string]string{"TRUST_DECISION": "RTL_RiskBased", "TRUST_DECISION_BT": "x", "TRUST_DECISION_DT": "0.2", "TRUST_DECISION_UT": "0.4"}, "Could not parse TRUST_DECISION_BT"},
		"bad logic":             {map[string]string{"TRUST_DECISION": "RTL_RiskBased", "TRUST_DECISION_BT": "0.8", "TRUST_DECISION_DT": "0.2", "TRUST_DECISION_UT": "0.4", "TRUST_DECISION_LOGIC": "foo"}, "Could not parse TRUST_DECISION_LOGIC"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := panicMessage(t, c.params); got != c.want {
				t.Fatalf("panic %q, want %q", got, c.want)
			}
		})
	}
}

func TestCorruptModelPanics(t *testing.T) {
	msg := panicMessage(t, map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_RF_MODEL_PARAM": "{bad"})
	if !strings.HasPrefix(msg, "Could not parse TRUST_DECISION_RF_MODEL_PARAM: ") {
		t.Fatalf("corrupt model should panic, got %q", msg)
	}
}

func TestRiskBasedDecides(t *testing.T) {
	risk := map[string]string{"TRUST_DECISION": "RTL_RiskBased", "TRUST_DECISION_BT": "0.8", "TRUST_DECISION_DT": "0.225", "TRUST_DECISION_UT": "0.4"}
	if decide(t, risk, probe) != core.NOT_TRUSTWORTHY {
		t.Error("risk should reject probe")
	}
	if decide(t, risk, op(0.8, 0.1, 0.1)) != core.TRUSTWORTHY {
		t.Error("components should accept (0.8, 0.1, 0.1)")
	}
	risk["TRUST_DECISION_LOGIC"] = "ProjectedProbability"
	if decide(t, risk, op(0.8, 0.1, 0.1)) != core.NOT_TRUSTWORTHY {
		t.Error("gray zone (P = 0.85, PUT = 0.80, PTT = 0.90) should reject")
	}
	if decide(t, risk, op(0, 0, 1)) != core.UNDECIDABLE {
		t.Error("u = 1 should be undecidable")
	}
}

// TestModelPerConfig: each configuration decides with its own Random Forest model.
func TestModelPerConfig(t *testing.T) {
	// model is a forest with a single leaf, which predicts the RTL (bt, 1, 1) for every ATL.
	model := func(bt float64) string {
		return fmt.Sprintf(`{"n_features": 3, "n_outputs": 3, "trees": [{"nodes": [{"f": -1, "v": [%g, 1, 1]}]}]}`, bt)
	}
	atl := op(0.85, 0.05, 0.1)
	if decide(t, map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_RF_MODEL_PARAM": model(0.9)}, atl) != core.NOT_TRUSTWORTHY {
		t.Error("model with bt 0.9 should reject b = 0.85")
	}
	if decide(t, map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_RF_MODEL_PARAM": model(0.8)}, atl) != core.TRUSTWORTHY {
		t.Error("model with bt 0.8 should accept b = 0.85")
	}
}
