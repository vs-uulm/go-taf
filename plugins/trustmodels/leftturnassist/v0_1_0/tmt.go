package leftturnassist_v0_1_0

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/core"
	"github.com/vs-uulm/go-taf/pkg/trustdecision"
	"github.com/vs-uulm/go-taf/pkg/trustmodel/trustmodelstructure"
)

var FullBelief, _ = subjectivelogic.NewOpinion(1, 0, 0, 0.5)
var FullUncertainty, _ = subjectivelogic.NewOpinion(0, 0, 1, 0.5)
var RTL, _ = subjectivelogic.NewOpinion(1, 0, 0, 0.5)

var DEFAULT_FUSION_OPERATOR = trustmodelstructure.CumulativeFusion
var DEFAULT_DISCOUNTING_OPERATOR = trustmodelstructure.BaseRateSensitiveDiscount

type TrustModelTemplate struct {
	name          string
	version       string
	evidenceTypes []core.EvidenceType
	params        map[string]string
}

func CreateTrustModelTemplate(name string, version string) core.TrustModelTemplate {
	return TrustModelTemplate{
		name:    name,
		version: version,
		evidenceTypes: append(append(slices.Clone(mbdEvidence), tchEvidence...),
			core.V2X_POSITION_OPINION,
		),
	}
}

func (t TrustModelTemplate) Version() string {
	return t.version
}

func (t TrustModelTemplate) TemplateName() string {
	return t.name
}

func (t TrustModelTemplate) Spawn(params map[string]string, context core.TafContext) ([]core.TrustSourceQuantifier, core.TrustModelInstance, core.DynamicTrustModelInstanceSpawner, error) {
	tsqs, err := createTrustSourceQuantifiers(params)
	if err != nil {
		return nil, nil, nil, err
	} else {
		spawner := NewDynamicTrustModelTemplateSpawner(t, params)
		//the TMIs apply the quantifier for CAM position opinions themselves (see TrustModelInstance.Update)
		for _, tsq := range tsqs {
			if tsq.TrustSource == core.V2X {
				spawner.camQuantifier = tsq
			}
		}
		return tsqs, nil, spawner, nil
	}
}

func (t TrustModelTemplate) Description() string {
	return "Left-turn Assist Model."
}

func (t TrustModelTemplate) Type() core.TrustModelTemplateType {
	return core.VEHICLE_TRIGGERED_TRUST_MODEL
}

func (t TrustModelTemplate) Identifier() string {
	return fmt.Sprintf("%s@%s", t.TemplateName(), t.Version())
}

func (t TrustModelTemplate) EvidenceTypes() []core.EvidenceType {
	return t.evidenceTypes
}

func (tmt TrustModelTemplate) SigningHash() string {
	return SigningHash
}

type DynamicTrustModelTemplateSpawner struct {
	template      TrustModelTemplate
	params        map[string]string
	camQuantifier core.TrustSourceQuantifier
}

func NewDynamicTrustModelTemplateSpawner(template TrustModelTemplate, params map[string]string) DynamicTrustModelTemplateSpawner {
	return DynamicTrustModelTemplateSpawner{
		template: template,
		params:   params,
	}
}

func (t DynamicTrustModelTemplateSpawner) OnNewTrustee(identifier string, params map[string]string) (core.TrustModelInstance, error) {
	return nil, nil
}

/*
OnNewVehicle spawns a new TMI upon an arriving vehicle. It has the following optional parameters to be used in the TAS_INIT

	FUSION_OPERATOR 		: Aleatory_Cumulative|Epistemic_Cumulative|Weighted|Consensus_Compromise|Proportional
								Proportional
									Mandatory Params:
												- PROPORTIONAL_FUSION_WEIGHT_EGO (weight in [0,1] of V_ego's own opinion on C_x_x; the opinion via V_x gets 1-weight)
	DISCOUNTING_OPERATOR 	: Uncertainty_Favouring|Base_Rate_Sensitive|Disbelief_Favouring
	MBD_WINDOW_WEIGHTING	: equal (default)|linear|temporal
								equal, linear
									Optional Params:
												- MBD_WINDOW_SIZE (number of recent MBD opinions, default 1 = off)
								temporal
									Mandatory Params:
												- MBD_TEMPORAL_GAMMA (rate in [0,1])
												- MBD_TEMPORAL_TAU (reference belief in [0,1])
	TRUST_DECISION			: 	ProjectedProbability
									Mandatory Params:
												- TRUST_DECISION_RTL_PP (projected prob. of RTL)
								Uncertainty
									Mandatory Params:
												- TRUST_DECISION_M (u <= m)
												- TRUST_DECISION_DELTA (|b-d| <= delta)
								Belief
									Mandatory Params:
												- TRUST_DECISION_K (b >= k)
								RTL_RiskBased
									Mandatory Params:
												- TRUST_DECISION_BT, TRUST_DECISION_DT, TRUST_DECISION_UT (manual RTL, each in [0,1])
									Optional Params:
												- TRUST_DECISION_LOGIC (Components (default) | ProjectedProbability)
								RTL_RandomForest, RTL_Optuna: the trained model of this TAF configuration is used (panics if it cannot be read),
								<TRUST_DECISION_MODEL_DIR>/<TD>-<TF>-<TQM>[-<MBD window>]/rf_model.json (RF) or rtl_optuna.json (Optuna),
								e.g. /opt/rtl-models/BR-ACF-bspline/rf_model.json, /opt/rtl-models/BR-ACF-bspline-temporal-g0.2-t0.5/
									Optional Params:
												- TRUST_DECISION_MODEL_DIR (default /opt/rtl-models)
												- TRUST_DECISION_RF_MODEL / TRUST_DECISION_OPTUNA_MODEL (explicit model path, overrides the selection)
												- TRUST_DECISION_LOGIC (Components (default) | ProjectedProbability)
*/
func (t DynamicTrustModelTemplateSpawner) OnNewVehicle(identifier string, params map[string]string) (core.TrustModelInstance, error) {
	initialParams := t.params
	newParams := params
	params = map[string]string{}
	//add parameters set at Spawn() call
	if initialParams != nil {
		for key, value := range initialParams {
			params[key] = value
		}
	}
	//add/overwrite parameters set at OnNewVehicle() call
	if newParams != nil {
		for key, value := range newParams {
			params[key] = value
		}
	}

	/*  Aleatory Cumulative Fusion, Epistemic Cumulative Fusion, Weighted Fusion, Consenses and Compromise Fusion, Proportional Fusion */
	fusionOperator := DEFAULT_FUSION_OPERATOR
	fusionWeightEgo := .5
	paramFusionOperator, exists := params["FUSION_OPERATOR"]
	if exists {
		switch paramFusionOperator {
		case "Aleatory_Cumulative":
			fusionOperator = trustmodelstructure.CumulativeFusion
		case "Epistemic_Cumulative":
			fusionOperator = trustmodelstructure.EpistemicCumulativeFusion
		case "Weighted":
			fusionOperator = trustmodelstructure.WeightedFusion
		case "Consensus_Compromise":
			fusionOperator = trustmodelstructure.ConsensusAndCompromiseFusion
		case "Proportional":
			fusionOperator = trustmodelstructure.ProportionalFusion
			paramFusionWeightEgo, exists := params["PROPORTIONAL_FUSION_WEIGHT_EGO"]
			if exists {
				if value, err := strconv.ParseFloat(paramFusionWeightEgo, 64); err == nil && value >= 0 && value <= 1 {
					fusionWeightEgo = value
				} else {
					panic("Could not parse PROPORTIONAL_FUSION_WEIGHT_EGO")
				}
			} else {
				panic("Missing value: PROPORTIONAL_FUSION_WEIGHT_EGO")
			}
		}
	}

	/* - Uncertainty-favouring Trust Discounting, Base-rate-sensitive Trust Discounting, and the Disbelief-favouring*/
	discountingOperator := DEFAULT_DISCOUNTING_OPERATOR
	paramTrustDiscounter, exists := params["DISCOUNTING_OPERATOR"]
	if exists {
		switch paramTrustDiscounter {
		case "Uncertainty_Favouring":
			discountingOperator = trustmodelstructure.UncertaintyFavouringDiscount
		case "Base_Rate_Sensitive":
			discountingOperator = trustmodelstructure.BaseRateSensitiveDiscount
		case "Disbelief_Favouring":
			discountingOperator = trustmodelstructure.DisbeliefFavouringDiscount
		}
	}

	mbdWindowing := newMbdWindowing(params)

	//default func: ProjectedProbability with .5
	trustDecider := func(proposition string, atl subjectivelogic.QueryableOpinion, rtl subjectivelogic.QueryableOpinion) core.TrustDecision {
		if atl.Uncertainty() == 1 {
			return core.UNDECIDABLE
		} else {
			var probabilisticAtl = trustdecision.ProjectProbability(atl)
			if probabilisticAtl > 0.5 {
				return core.TRUSTWORTHY
			} else {
				return core.NOT_TRUSTWORTHY
			}
		}
	}
	// if other TRUST_DECISION is set, change function
	paramTrustDecider, exists := params["TRUST_DECISION"]
	if exists {
		switch paramTrustDecider {
		case "ProjectedProbability":
			rtlPP := .5
			paramTrustDeciderRTL, exists := params["TRUST_DECISION_RTL_PP"]
			if exists {
				if value, err := strconv.ParseFloat(paramTrustDeciderRTL, 64); err == nil {
					rtlPP = value
				} else {
					panic("Could not parse TRUST_DECISION_RTL_PP")
				}
			} else {
				panic("Missing value: TRUST_DECISION_RTL_PP")
			}

			//default func: ProjectedProbability with .5
			trustDecider = func(proposition string, atl subjectivelogic.QueryableOpinion, rtl subjectivelogic.QueryableOpinion) core.TrustDecision {
				if atl.Uncertainty() == 1 {
					return core.UNDECIDABLE
				} else {
					var probabilisticAtl = trustdecision.ProjectProbability(atl)
					if probabilisticAtl > rtlPP {
						return core.TRUSTWORTHY
					} else {
						return core.NOT_TRUSTWORTHY
					}
				}
			}
		case "Uncertainty":
			m := .1
			paramTrustDeciderM, exists := params["TRUST_DECISION_M"]
			if exists {
				if value, err := strconv.ParseFloat(paramTrustDeciderM, 64); err == nil {
					m = value
				} else {
					panic("Could not parse TRUST_DECISION_M")
				}
			} else {
				panic("Missing value: TRUST_DECISION_M") // TODO: add to others
			}
			delta := .1
			paramTrustDeciderDelta, exists := params["TRUST_DECISION_DELTA"]
			if exists {
				if value, err := strconv.ParseFloat(paramTrustDeciderDelta, 64); err == nil {
					delta = value
				} else {
					panic("Could not parse TRUST_DECISION_DELTA")
				}
			} else {
				panic("Missing value: TRUST_DECISION_DELTA") // TODO: add to others
			}
			trustDecider = func(proposition string, atl subjectivelogic.QueryableOpinion, rtl subjectivelogic.QueryableOpinion) core.TrustDecision {
				if atl.Uncertainty() <= m && math.Abs(atl.Belief()-atl.Disbelief()) <= delta {
					return core.TRUSTWORTHY
				} else {
					return core.NOT_TRUSTWORTHY
				}
			}
		case "Belief":
			k := .5
			paramTrustDeciderDelta, exists := params["TRUST_DECISION_K"]
			if exists {
				if value, err := strconv.ParseFloat(paramTrustDeciderDelta, 64); err == nil {
					k = value
				} else {
					panic("Could not parse TRUST_DECISION_K")
				}
			} else {
				panic("Missing value: TRUST_DECISION_K") // TODO: add to others
			}
			trustDecider = func(proposition string, atl subjectivelogic.QueryableOpinion, rtl subjectivelogic.QueryableOpinion) core.TrustDecision {
				if atl.Belief() >= k {
					return core.TRUSTWORTHY
				} else {
					return core.NOT_TRUSTWORTHY
				}
			}
		// RTL thresholds (bt, dt, ut): manual, Random Forest or Optuna. See rtl_decision.go.
		case "RTL_RiskBased", "RTL_RandomForest", "RTL_Optuna":
			trustDecider = newRTLTrustDecider(paramTrustDecider, params)
		}
	}

	return &TrustModelInstance{
		id:                  identifier,
		version:             0,
		template:            t.template,
		staticRTL:           &RTL,
		fusionOperator:      fusionOperator,
		fusionWeightEgo:     fusionWeightEgo,
		discountingOperator: discountingOperator,
		mbdWindowing:        mbdWindowing,
		trustDecision:       trustDecider,
		camQuantifier:       t.camQuantifier,
	}, nil
}
