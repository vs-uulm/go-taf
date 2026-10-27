package leftturnassist_v0_1_0

// Tests the RTL trust deciders through OnNewVehicle, the same path the TAF uses: decisions, the Random Forest model
// passed as parameter content, and panics on a missing or invalid parameter.

import (
	"bytes"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"testing"
	"weak"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/core"

	"github.com/vs-uulm/go-taf/plugins/trustmodels/leftturnassist/v0_1_0/rtl"
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

func captureLog(t *testing.T) *bytes.Buffer {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

// probe: P = 0.70 -> default (P > 0.5) accepts; RTL (0.8, 0.225, 0.4) rejects.
var probe = op(0.45, 0.05, 0.5)

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

// singleLeafForest is an RF model with one tree of one leaf, which predicts the RTL (bt, 1, 1) for every ATL.
func singleLeafForest(bt float64) string {
	return fmt.Sprintf(`{"n_features": 3, "n_outputs": 3, "trees": [{"nodes": [{"f": -1, "v": [%g, 1, 1]}]}]}`, bt)
}

func TestInvalidParamPanics(t *testing.T) {
	risk := func(bt, dt, ut string) map[string]string {
		p := map[string]string{"TRUST_DECISION": "RTL_RiskBased"}
		for key, value := range map[string]string{"TRUST_DECISION_BT": bt, "TRUST_DECISION_DT": dt, "TRUST_DECISION_UT": ut} {
			if value != "" {
				p[key] = value
			}
		}
		return p
	}
	badLogic := risk("0.8", "0.2", "0.4")
	badLogic["TRUST_DECISION_LOGIC"] = "foo"
	cases := map[string]struct {
		params map[string]string
		want   string
	}{
		"risk missing DT/UT":  {risk("0.8", "", ""), "Missing value: TRUST_DECISION_DT"},
		"risk missing UT":     {risk("0.8", "0.2", ""), "Missing value: TRUST_DECISION_UT"},
		"risk above range":    {risk("0.8", "0.2", "1.4"), "Could not parse TRUST_DECISION_UT"},
		"risk below range":    {risk("-0.1", "0.2", "0.4"), "Could not parse TRUST_DECISION_BT"},
		"risk NaN":            {risk("0.8", "NaN", "0.4"), "Could not parse TRUST_DECISION_DT"},
		"risk not a number":   {risk("x", "0.2", "0.4"), "Could not parse TRUST_DECISION_BT"},
		"bad logic":           {badLogic, "Could not parse TRUST_DECISION_LOGIC"},
		"bad logic before RF": {map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_LOGIC": "foo"}, "Could not parse TRUST_DECISION_LOGIC"},
		"Optuna without RTL":  {map[string]string{"TRUST_DECISION": "RTL_Optuna"}, "Missing value: TRUST_DECISION_BT"},
		"Optuna above range":  {map[string]string{"TRUST_DECISION": "RTL_Optuna", "TRUST_DECISION_BT": "1.5", "TRUST_DECISION_DT": "1", "TRUST_DECISION_UT": "1"}, "Could not parse TRUST_DECISION_BT"},
		"RF without model":    {map[string]string{"TRUST_DECISION": "RTL_RandomForest"}, "Missing value: TRUST_DECISION_RF_MODEL_PARAM"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := panicMessage(t, c.params); got != c.want {
				t.Fatalf("panic %q, want %q", got, c.want)
			}
		})
	}
}

// TestInvalidRFModelPanics: model content that is not a valid forest panics, both when a session with it is created
// and when a vehicle's parameters pass it.
func TestInvalidRFModelPanics(t *testing.T) {
	tree := func(nodes string) string {
		return `{"n_features": 3, "n_outputs": 3, "trees": [{"nodes": [` + nodes + `]}]}`
	}
	leaf := `{"f": -1, "v": [0.5, 0.5, 0.5]}`
	cases := map[string]struct {
		content string
		reason  string
	}{
		"not JSON":           {"{bad", "invalid character"},
		"empty object":       {"{}", "invalid dimensions"},
		"wrong dimensions":   {`{"n_features": 2, "n_outputs": 3, "trees": [{"nodes": [{"f": -1, "v": [1, 1, 1]}]}]}`, "maps 2 features to 3 outputs"},
		"no trees":           {`{"n_features": 3, "n_outputs": 3, "trees": []}`, "no trees"},
		"empty tree":         {tree(""), "tree 0 has no nodes"},
		"leaf values":        {tree(`{"f": -1, "v": [0.5]}`), "leaf has 1 values, want 3"},
		"invalid feature":    {tree(`{"f": 3, "t": 0.5, "l": 1, "r": 2},` + leaf + `,` + leaf), "invalid feature 3"},
		"child before node":  {tree(`{"f": 0, "t": 0.5, "l": 0, "r": 1},` + leaf), "invalid children 0, 1"},
		"child out of range": {tree(`{"f": 0, "t": 0.5, "l": 1, "r": 2},` + leaf), "invalid children 1, 2"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			params := map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_RF_MODEL_PARAM": c.content}
			check := func(where, msg string) {
				if !strings.HasPrefix(msg, "Could not parse TRUST_DECISION_RF_MODEL_PARAM: ") || !strings.Contains(msg, c.reason) {
					t.Fatalf("%s: panic %q does not mention %q", where, msg, c.reason)
				}
			}
			check("vehicle", panicMessage(t, params))
			check("session", func() (msg string) {
				defer func() { msg = fmt.Sprint(recover()) }()
				newSession(params)
				return ""
			}())
		})
	}
}

// TestFixedRTLDecides: risk-based and Optuna decide with the RTL given as parameters.
func TestFixedRTLDecides(t *testing.T) {
	for _, kind := range []string{"RTL_RiskBased", "RTL_Optuna"} {
		t.Run(kind, func(t *testing.T) {
			p := map[string]string{"TRUST_DECISION": kind, "TRUST_DECISION_BT": "0.8", "TRUST_DECISION_DT": "0.225", "TRUST_DECISION_UT": "0.4"}
			if decide(t, p, probe) != core.NOT_TRUSTWORTHY {
				t.Error("should reject probe")
			}
			if decide(t, p, op(0.8, 0.1, 0.1)) != core.TRUSTWORTHY {
				t.Error("components should accept (0.8, 0.1, 0.1)")
			}
			p["TRUST_DECISION_LOGIC"] = "ProjectedProbability"
			if decide(t, p, op(0.8, 0.1, 0.1)) != core.NOT_TRUSTWORTHY {
				t.Error("gray zone (P = 0.85, PUT = 0.80, PTT = 0.90) should reject")
			}
			if decide(t, p, op(0, 0, 1)) != core.UNDECIDABLE {
				t.Error("u = 1 should be undecidable")
			}
		})
	}
}

// newSession creates the spawner of a session with the given parameters, as Spawn does at TAS_INIT.
func newSession(params map[string]string) DynamicTrustModelTemplateSpawner {
	return NewDynamicTrustModelTemplateSpawner(CreateTrustModelTemplate("LTA", "0.1.0").(TrustModelTemplate), params)
}

// decideInSession spawns a TMI for a new vehicle of the session (without own parameters, as the TAF does) and runs its
// trust decision.
func decideInSession(t *testing.T, session DynamicTrustModelTemplateSpawner, vehicle int, atl subjectivelogic.QueryableOpinion) core.TrustDecision {
	t.Helper()
	tmi, err := session.OnNewVehicle(fmt.Sprintf("veh_%d", vehicle), nil)
	if err != nil {
		t.Fatal(err)
	}
	return tmi.(*TrustModelInstance).Decide("p", atl, nil)
}

// TestRFModelPerSession: the Random Forest decides with the model passed as content; a session parses its model once,
// when it is created, and all its vehicles share it.
func TestRFModelPerSession(t *testing.T) {
	buf := captureLog(t)
	atl := op(0.85, 0.05, 0.1)
	strict := newSession(map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_RF_MODEL_PARAM": singleLeafForest(0.9)})
	lenient := newSession(map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_RF_MODEL_PARAM": singleLeafForest(0.8)})
	if n := strings.Count(buf.String(), "RTL model loaded"); n != 2 {
		t.Errorf("want each session's model parsed when the session is created, got %d loads: %s", n, buf.String())
	}
	for vehicle := 1; vehicle <= 3; vehicle++ {
		if decideInSession(t, strict, vehicle, atl) != core.NOT_TRUSTWORTHY {
			t.Error("bt 0.9 should reject b = 0.85")
		}
		if decideInSession(t, lenient, vehicle, atl) != core.TRUSTWORTHY {
			t.Error("bt 0.8 should accept b = 0.85")
		}
	}
	if decideInSession(t, strict, 4, op(0, 0, 1)) != core.UNDECIDABLE {
		t.Error("u = 1 should be undecidable")
	}
	if n := strings.Count(buf.String(), "RTL model loaded"); n != 2 {
		t.Errorf("want no further parsing for the sessions' vehicles, got %d loads: %s", n, buf.String())
	}
	// a vehicle's own model content (not used by the TAF) is parsed for that vehicle
	if decide(t, map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_RF_MODEL_PARAM": singleLeafForest(0.8)}, atl) != core.TRUSTWORTHY {
		t.Error("bt 0.8 should accept b = 0.85")
	}
}

// TestRFModelFreedWithSession: nothing but the session and its TMIs holds the parsed model, so it is freed with them.
func TestRFModelFreedWithSession(t *testing.T) {
	model := func() weak.Pointer[rtl.Model] {
		session := newSession(map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_RF_MODEL_PARAM": singleLeafForest(0.9)})
		decideInSession(t, session, 1, op(0.85, 0.05, 0.1))
		return weak.Make(session.rfModel.model)
	}()
	alive := newSession(map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_RF_MODEL_PARAM": singleLeafForest(0.8)})
	aliveModel := weak.Make(alive.rfModel.model)
	runtime.GC()
	if model.Value() != nil {
		t.Fatal("the model is still reachable after its session was dropped")
	}
	if aliveModel.Value() == nil {
		t.Fatal("the model of a live session was freed")
	}
	runtime.KeepAlive(alive)
}
