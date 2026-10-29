package leftturnassist_v0_1_0

// RTL-based trust deciders: risk-based, Random Forest and Optuna.
// Each one supports the component rule or the projected-probability rule (bounds rtl.PTT and rtl.PUT).
// Risk-based and Optuna take the RTL as parameters; the Optuna RTL is trained outside the TAF.
// The Random Forest takes the trained model as parameter content (TRUST_DECISION_RF_MODEL_PARAM), like the MBD
// quantification models; the TAF reads no files.
// A missing or invalid parameter makes the TMI panic, like the other parameters of the trust model.

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strconv"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/core"
	"github.com/vs-uulm/go-taf/pkg/trustdecision"

	"github.com/vs-uulm/go-taf/plugins/trustmodels/leftturnassist/v0_1_0/rtl"
)

// trustDeciderFunc is the signature used by TrustModelInstance.trustDecision.
type trustDeciderFunc = func(proposition string, atl subjectivelogic.QueryableOpinion, rtl subjectivelogic.QueryableOpinion) core.TrustDecision

// rfModel is a parsed Random Forest model and the content of TRUST_DECISION_RF_MODEL_PARAM it was parsed from.
type rfModel struct {
	content string
	model   *rtl.Model
}

// parseRFModel parses the content of TRUST_DECISION_RF_MODEL_PARAM; it panics if it is not a valid model.
func parseRFModel(content string) *rfModel {
	m, err := rtl.ParseModel([]byte(content))
	if err != nil {
		panic("Could not parse TRUST_DECISION_RF_MODEL_PARAM: " + err.Error())
	}
	sum := sha256.Sum256([]byte(content))
	slog.Default().Info("RTL model loaded", "TRUST_DECISION", "RTL_RandomForest", "bytes", len(content),
		"sha256", hex.EncodeToString(sum[:]))
	return &rfModel{content: content, model: m}
}

// sessionRFModel parses the Random Forest model of a session's parameters once, when the session is created (Spawn),
// so that all TMIs of the session share it and it is freed with the session at TAS_TEARDOWN. It returns nil if the
// session does not use RTL_RandomForest or does not pass the model (then it has to come with the vehicle's parameters).
func sessionRFModel(params map[string]string) *rfModel {
	if params["TRUST_DECISION"] != "RTL_RandomForest" {
		return nil
	}
	content, exists := params["TRUST_DECISION_RF_MODEL_PARAM"]
	if !exists {
		return nil
	}
	return parseRFModel(content)
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
// session is the session's Random Forest model (sessionRFModel), used as long as the vehicle's parameters pass the same
// content. It panics if a mandatory parameter is missing or invalid.
func newRTLTrustDecider(kind string, params map[string]string, session *rfModel) trustDeciderFunc {
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
		// The TMIs get the session's parameter string, so this comparison does not compare the content byte by byte.
		model := session
		if model == nil || model.content != content {
			model = parseRFModel(content)
		}
		rtlFor = model.model.PredictRTL
	default:
		panic("Unknown RTL trust decision: " + kind)
	}

	return func(proposition string, atlOpinion subjectivelogic.QueryableOpinion, _ subjectivelogic.QueryableOpinion) core.TrustDecision {
		if atlOpinion.Uncertainty() == 1 {
			return core.UNDECIDABLE
		}
		atl := rtl.ATL{B: atlOpinion.Belief(), D: atlOpinion.Disbelief(), U: atlOpinion.Uncertainty()}
		required := rtlFor(atl)

		if usePP {
			p := trustdecision.ProjectProbability(atlOpinion)
			switch {
			case p > rtl.PTT(required, atlOpinion.BaseRate()):
				return core.TRUSTWORTHY
			case p < rtl.PUT(required, atlOpinion.BaseRate()):
				return core.NOT_TRUSTWORTHY
			default:
				// gray zone: the ATL may or may not meet the RTL; rejected (fail-safe)
				return core.NOT_TRUSTWORTHY
			}
		}
		if rtl.Decide(atl, required) {
			return core.TRUSTWORTHY
		}
		return core.NOT_TRUSTWORTHY
	}
}
