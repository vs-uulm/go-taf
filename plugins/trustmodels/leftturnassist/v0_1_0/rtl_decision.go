package leftturnassist_v0_1_0

// RTL-based trust deciders: risk-based, Random Forest and Optuna.
// Each one supports the component rule or the projected-probability rule (PUT/PTT).
// Risk-based and Optuna take the RTL as parameters; the Optuna RTL is trained outside the TAF.
// The Random Forest takes the trained model as parameter content (TRUST_DECISION_RF_MODEL_PARAM), like the MBD
// quantification models; the TAF reads no files.
// A missing or invalid parameter makes the TMI panic, like the other parameters of the trust model.

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strconv"
	"sync"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/core"

	"github.com/vs-uulm/go-taf/plugins/trustmodels/leftturnassist/v0_1_0/rtl"
)

// trustDeciderFunc is the signature used by TrustModelInstance.trustDecision.
type trustDeciderFunc = func(proposition string, atl subjectivelogic.QueryableOpinion, rtl subjectivelogic.QueryableOpinion) core.TrustDecision

// Random Forest models, keyed by the content of TRUST_DECISION_RF_MODEL_PARAM: each model is parsed once (result or
// error) for all TMIs, not once per vehicle. The TMIs share the parameter string, so a lookup does not copy it.
var rfModels = struct {
	sync.Mutex
	models map[string]*rtl.Model
	errs   map[string]error
}{models: map[string]*rtl.Model{}, errs: map[string]error{}}

// rfModel returns the parsed Random Forest model of the given content.
func rfModel(content string) (*rtl.Model, error) {
	rfModels.Lock()
	defer rfModels.Unlock()
	if m, ok := rfModels.models[content]; ok {
		return m, nil
	}
	if err, ok := rfModels.errs[content]; ok {
		return nil, err
	}
	m, err := rtl.ParseModel([]byte(content))
	if err != nil {
		rfModels.errs[content] = err
		return nil, err
	}
	rfModels.models[content] = m
	sum := sha256.Sum256([]byte(content))
	slog.Default().Info("RTL model loaded", "TRUST_DECISION", "RTL_RandomForest", "bytes", len(content),
		"sha256", hex.EncodeToString(sum[:]))
	return m, nil
}

// thresholdParam reads a mandatory threshold in [0,1]; it panics if the parameter is missing or invalid.
func thresholdParam(params map[string]string, key string) float64 {
	raw, exists := params[key]
	if !exists {
		panic("Missing value: " + key)
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || !(value >= 0 && value <= 1) {
		panic("Could not parse " + key)
	}
	return value
}

// newRTLTrustDecider builds the decider for TRUST_DECISION = RTL_RiskBased | RTL_RandomForest | RTL_Optuna.
//
//	TRUST_DECISION_LOGIC: Components (default) | ProjectedProbability
//	RTL_RiskBased    -> TRUST_DECISION_BT, TRUST_DECISION_DT, TRUST_DECISION_UT (all three, set manually)
//	RTL_Optuna       -> TRUST_DECISION_BT, TRUST_DECISION_DT, TRUST_DECISION_UT (all three, found by Optuna)
//	RTL_RandomForest -> TRUST_DECISION_RF_MODEL_PARAM (content of the trained model, rf_model.json)
//
// It panics if a mandatory parameter is missing or invalid.
func newRTLTrustDecider(kind string, params map[string]string) trustDeciderFunc {
	// Decision logic.
	usePP := false
	if logic, exists := params["TRUST_DECISION_LOGIC"]; exists {
		switch logic {
		case "Components":
		case "ProjectedProbability":
			usePP = true
		default:
			panic("Could not parse TRUST_DECISION_LOGIC")
		}
	}

	// RTL source: returns the RTL for a given ATL.
	var rtlFor func(rtl.ATL) rtl.RTL
	switch kind {
	case "RTL_RiskBased", "RTL_Optuna":
		fixed := rtl.RTL{
			Bt: thresholdParam(params, "TRUST_DECISION_BT"),
			Dt: thresholdParam(params, "TRUST_DECISION_DT"),
			Ut: thresholdParam(params, "TRUST_DECISION_UT"),
		}
		rtlFor = func(rtl.ATL) rtl.RTL { return fixed }
	case "RTL_RandomForest":
		content, exists := params["TRUST_DECISION_RF_MODEL_PARAM"]
		if !exists {
			panic("Missing value: TRUST_DECISION_RF_MODEL_PARAM")
		}
		model, err := rfModel(content)
		if err != nil {
			panic("Could not parse TRUST_DECISION_RF_MODEL_PARAM: " + err.Error())
		}
		rtlFor = model.PredictRTL
	default:
		panic("Unknown RTL trust decision: " + kind)
	}

	return func(proposition string, atlOpinion subjectivelogic.QueryableOpinion, _ subjectivelogic.QueryableOpinion) core.TrustDecision {
		if atlOpinion.Uncertainty() == 1 {
			return core.UNDECIDABLE
		}
		atl := rtl.ATL{B: atlOpinion.Belief(), D: atlOpinion.Disbelief(), U: atlOpinion.Uncertainty()}
		required := rtlFor(atl)

		trusted := false
		if usePP {
			// Gray zone (PUT <= P <= PTT) is rejected.
			trusted = rtl.DecidePP(atl, required, atlOpinion.BaseRate()).Trusted
		} else {
			trusted = rtl.Decide(atl, required)
		}
		if trusted {
			return core.TRUSTWORTHY
		}
		return core.NOT_TRUSTWORTHY
	}
}
