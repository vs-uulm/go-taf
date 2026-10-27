package leftturnassist_v0_1_0

// Tests the RTL trust deciders through OnNewVehicle, the same path the TAF uses: decisions, model selection, and
// panics on a missing or invalid parameter or a trained model that cannot be read.

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
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
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := panicMessage(t, c.params); got != c.want {
				t.Fatalf("panic %q, want %q", got, c.want)
			}
		})
	}
}

// TestUnreadableModelPanics: a trained model that cannot be read makes the TMI panic, also for a second vehicle
// (the error is cached per path).
func TestUnreadableModelPanics(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) string {
		if err := os.WriteFile(dir+"/"+name, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		return dir + "/" + name
	}
	cases := map[string]struct {
		params map[string]string
		reason string
	}{
		"missing RF file":     {map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_RF_MODEL": "/nope/rf_model.json"}, "not found"},
		"no model for config": {map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_QUANTIFICATION_MECHANISM_MBD": "b-spline", "TRUST_DECISION_MODEL_DIR": "/nope"}, "/nope/BR-ACF-bspline/rf_model.json"},
		"TQM not set":         {map[string]string{"TRUST_DECISION": "RTL_Optuna"}, "TRUST_QUANTIFICATION_MECHANISM_MBD is not set"},
		"no model for new op": {map[string]string{"TRUST_DECISION": "RTL_Optuna", "TRUST_QUANTIFICATION_MECHANISM_MBD": "b-spline", "FUSION_OPERATOR": "New_Fusion", "TRUST_DECISION_MODEL_DIR": "/nope"}, "BR-New_Fusion-bspline"},
		"missing Optuna file": {map[string]string{"TRUST_DECISION": "RTL_Optuna", "TRUST_DECISION_OPTUNA_MODEL": "/nope/rtl_optuna.json"}, "not found"},
		"model is a dir":      {map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_RF_MODEL": dir}, "is a directory"},
		"corrupt RF model":    {map[string]string{"TRUST_DECISION": "RTL_RandomForest", "TRUST_DECISION_RF_MODEL": write("bad_rf.json", "{bad")}, "corrupt RF model"},
		"corrupt Optuna":      {map[string]string{"TRUST_DECISION": "RTL_Optuna", "TRUST_DECISION_OPTUNA_MODEL": write("bad_optuna.json", "{bad")}, "corrupt Optuna model"},
		"Optuna without rtl":  {map[string]string{"TRUST_DECISION": "RTL_Optuna", "TRUST_DECISION_OPTUNA_MODEL": write("no_rtl.json", `{}`)}, `no "rtl" field`},
		"Optuna out of range": {map[string]string{"TRUST_DECISION": "RTL_Optuna", "TRUST_DECISION_OPTUNA_MODEL": write("range.json", `{"rtl": {"Bt": 1.5, "Dt": 1, "Ut": 1}}`)}, "outside [0,1]"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			for vehicle := 1; vehicle <= 2; vehicle++ {
				msg := panicMessage(t, c.params)
				if !strings.HasPrefix(msg, "Could not read RTL model: ") || !strings.Contains(msg, c.reason) {
					t.Fatalf("vehicle %d: panic %q does not mention %q", vehicle, msg, c.reason)
				}
			}
		})
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

func TestConfigName(t *testing.T) {
	cases := []struct {
		params map[string]string
		want   string
	}{
		{map[string]string{"TRUST_QUANTIFICATION_MECHANISM_MBD": "b-spline"}, "BR-ACF-bspline"}, // TAF defaults
		{map[string]string{"DISCOUNTING_OPERATOR": "Uncertainty_Favouring", "FUSION_OPERATOR": "Proportional",
			"TRUST_QUANTIFICATION_MECHANISM_MBD": "f1-optimized"}, "UF-PF-analytic_f1"},
		{map[string]string{"DISCOUNTING_OPERATOR": "Disbelief_Favouring", "FUSION_OPERATOR": "Consensus_Compromise",
			"TRUST_QUANTIFICATION_MECHANISM_MBD": "mlp"}, "DF-CCF-mlp"},
		{map[string]string{"FUSION_OPERATOR": "New_Fusion", "TRUST_QUANTIFICATION_MECHANISM_MBD": "new-tqm"}, "BR-New_Fusion-new-tqm"}, // new names as is
		// MBD windowing, as in the window column of eval/results-matrix.sh
		{map[string]string{"TRUST_QUANTIFICATION_MECHANISM_MBD": "b-spline", "MBD_WINDOW_WEIGHTING": "temporal",
			"MBD_TEMPORAL_GAMMA": "0.20", "MBD_TEMPORAL_TAU": "0.5"}, "BR-ACF-bspline-temporal-g0.2-t0.5"},
		{map[string]string{"TRUST_QUANTIFICATION_MECHANISM_MBD": "mlp", "MBD_WINDOW_WEIGHTING": "linear", "MBD_WINDOW_SIZE": "3"}, "BR-ACF-mlp-linear-n3"},
		{map[string]string{"TRUST_QUANTIFICATION_MECHANISM_MBD": "mlp", "MBD_WINDOW_WEIGHTING": "linear", "MBD_WINDOW_SIZE": "1"}, "BR-ACF-mlp"}, // size 1 = off
	}
	for _, c := range cases {
		if got, err := rtlConfigName(c.params); err != nil || got != c.want {
			t.Errorf("%v: got %q, %v; want %q", c.params, got, err, c.want)
		}
	}
}

// TestModelPerConfig: each configuration reads its own Optuna model; an explicit path overrides the selection.
func TestModelPerConfig(t *testing.T) {
	dir := t.TempDir()
	write := func(config string, bt float64) {
		os.MkdirAll(dir+"/"+config, 0o755)
		data := fmt.Sprintf(`{"rtl": {"Bt": %g, "Dt": 1, "Ut": 1}}`, bt)
		if err := os.WriteFile(dir+"/"+config+"/rtl_optuna.json", []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("BR-ACF-bspline", 0.9) // rejects b = 0.85
	write("UF-WBF-mlp", 0.8)     // accepts b = 0.85
	atl := op(0.85, 0.05, 0.1)
	base := map[string]string{"TRUST_DECISION": "RTL_Optuna", "TRUST_DECISION_MODEL_DIR": dir}
	with := func(extra map[string]string) map[string]string {
		p := map[string]string{}
		for k, v := range base {
			p[k] = v
		}
		for k, v := range extra {
			p[k] = v
		}
		return p
	}
	buf := captureLog(t)
	if decide(t, with(map[string]string{"TRUST_QUANTIFICATION_MECHANISM_MBD": "b-spline"}), atl) != core.NOT_TRUSTWORTHY {
		t.Error("BR-ACF-bspline (bt 0.9) should reject")
	}
	uf := with(map[string]string{"DISCOUNTING_OPERATOR": "Uncertainty_Favouring", "FUSION_OPERATOR": "Weighted", "TRUST_QUANTIFICATION_MECHANISM_MBD": "mlp"})
	if decide(t, uf, atl) != core.TRUSTWORTHY {
		t.Error("UF-WBF-mlp (bt 0.8) should accept")
	}
	uf["TRUST_DECISION_OPTUNA_MODEL"] = dir + "/BR-ACF-bspline/rtl_optuna.json"
	if decide(t, uf, atl) != core.NOT_TRUSTWORTHY {
		t.Error("explicit path should override the selection")
	}
	if !strings.Contains(buf.String(), "UF-WBF-mlp/rtl_optuna.json") {
		t.Error(buf.String())
	}
}
