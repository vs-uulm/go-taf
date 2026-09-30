package leftturnassist_v0_1_0

import (
	"fmt"
	"math"
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

	//Extract list of used trust sources from TrustSourceQuantifiers
	tsqs, _ := createTrustSourceQuantifiers(nil)
	evidenceMap := make(map[core.EvidenceType]bool)
	for _, quantifier := range tsqs {
		for _, evidence := range quantifier.Evidence {
			evidenceMap[evidence] = true
		}
	}
	evidenceTypes := make([]core.EvidenceType, len(evidenceMap))
	i := 0
	for k := range evidenceMap {
		evidenceTypes[i] = k
		i++
	}

	return TrustModelTemplate{
		name:          name,
		version:       version,
		evidenceTypes: evidenceTypes,
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
	template TrustModelTemplate
	params   map[string]string
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
	DISCOUNTING_OPERATOR 	: Uncertainty_Favouring|Base_Rate_Sensitive|Disbelief_Favouring
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

	//ProjectedProbability as default
	rtlPP := .5 //TODO: default?
	paramTrustDeciderRTL, exists := params["TRUST_DECISION_RTL_PP"]
	if exists {
		if value, err := strconv.ParseFloat(paramTrustDeciderRTL, 64); err == nil {
			if err == nil {
				rtlPP = value
			}
		}
	}
	trustDecider := func(proposition string, atl subjectivelogic.QueryableOpinion, rtl subjectivelogic.QueryableOpinion) core.TrustDecision {
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
	// if other TRUST_DECISION is set, change function
	paramTrustDecider, exists := params["TRUST_DECISION"]
	if exists {
		switch paramTrustDecider {
		case "Uncertainty":
			m := .1 //TODO: default?
			paramTrustDeciderM, exists := params["TRUST_DECISION_M"]
			if exists {
				if value, err := strconv.ParseFloat(paramTrustDeciderM, 64); err == nil {
					if err == nil {
						m = value
					}
				}
			}
			delta := .1 //TODO: default?
			paramTrustDeciderDelta, exists := params["TRUST_DECISION_DELTA"]
			if exists {
				if value, err := strconv.ParseFloat(paramTrustDeciderDelta, 64); err == nil {
					if err == nil {
						delta = value
					}
				}
			}
			trustDecider = func(proposition string, atl subjectivelogic.QueryableOpinion, rtl subjectivelogic.QueryableOpinion) core.TrustDecision {
				if atl.Uncertainty() <= m && math.Abs(atl.Belief()-atl.Disbelief()) <= delta {
					return core.TRUSTWORTHY
				} else {
					return core.NOT_TRUSTWORTHY
				}
			}
		case "Belief":
			k := .5 //TODO: default?
			paramTrustDeciderDelta, exists := params["TRUST_DECISION_K"]
			if exists {
				if value, err := strconv.ParseFloat(paramTrustDeciderDelta, 64); err == nil {
					if err == nil {
						k = value
					}
				}
			}
			trustDecider = func(proposition string, atl subjectivelogic.QueryableOpinion, rtl subjectivelogic.QueryableOpinion) core.TrustDecision {
				if atl.Belief() >= k {
					return core.TRUSTWORTHY
				} else {
					return core.NOT_TRUSTWORTHY
				}
			}
		}
	}

	return &TrustModelInstance{
		id:                  identifier,
		version:             0,
		template:            t.template,
		objects:             map[string]subjectivelogic.QueryableOpinion{},
		staticRTL:           &RTL,
		fusionOperator:      fusionOperator,
		discountingOperator: discountingOperator,
		trustDecision:       trustDecider,
	}, nil
}
