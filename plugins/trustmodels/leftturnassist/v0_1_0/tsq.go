package leftturnassist_v0_1_0

import (
	"fmt"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/core"
)

func createTrustSourceQuantifiers(params map[string]string) ([]core.TrustSourceQuantifier, error) {

	//quantifies the residuals of the Python-based MBD (see mbdEvidence) with the selected mechanism
	var mbdQuantify func(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion

	mbdTsqMechanism, exists := params["TRUST_QUANTIFICATION_MECHANISM_MBD"]
	if exists {
		switch mbdTsqMechanism {
		case "f1-optimized", "analytic":
			//Both use the analytic quantification approach and only differ in the parameters
			//TRUST_QUANTIFICATION_MECHANISM_MBD_PARAM: content of the parameters JSON file
			//(f1-optimized: parameters_f1_only.json, analytic: parameters_multi_objective.json)
			slParams, err := loadMbdSLParams(mbdParamJSON(params))
			if err != nil {
				return nil, fmt.Errorf("loading %s MBD quantification: %w", mbdTsqMechanism, err)
			}
			mbdQuantify = mbdFeatureQuantifier(slParams.computeTrustOpinion)
		case "b-spline":
			//TRUST_QUANTIFICATION_MECHANISM_MBD_PARAM: content of the B-spline model JSON file (bspline_model.json)
			bsplineModel, err := loadMbdBsplineModel(mbdParamJSON(params))
			if err != nil {
				return nil, fmt.Errorf("loading %s MBD quantification: %w", mbdTsqMechanism, err)
			}
			mbdQuantify = mbdFeatureQuantifier(bsplineModel.computeTrustOpinion)
		case "mlp":
			//TRUST_QUANTIFICATION_MECHANISM_MBD_PARAM: content of the MLP model JSON file (mlp_model.json)
			mlpModel, err := loadMbdMlpModel(mbdParamJSON(params))
			if err != nil {
				return nil, fmt.Errorf("loading %s MBD quantification: %w", mbdTsqMechanism, err)
			}
			mbdQuantify = mbdFeatureQuantifier(mlpModel.computeTrustOpinion)
		default:
			panic("Invalid TRUST_QUANTIFICATION_MECHANISM_MBD set: " + mbdTsqMechanism)
		}
	} else {
		panic("No value set for TRUST_QUANTIFICATION_MECHANISM_MBD")
	}

	mbdQuantifier := core.TrustSourceQuantifier{
		Trustor:     "V_ego",
		Trustee:     "C_*_*",
		Scope:       "C_*_*",
		TrustSource: core.MBD,
		Evidence:    mbdEvidence,
		Quantifier:  mbdQuantify,
	}

	tchQuantifier := core.TrustSourceQuantifier{
		Trustor:     "V_ego",
		Trustee:     "V_*",
		Scope:       "C_*_*",
		TrustSource: core.TCH,
		Evidence:    []core.EvidenceType{core.TCH_SECURE_BOOT, core.TCH_SECURE_OTA, core.TCH_ACCESS_CONTROL, core.TCH_APPLICATION_ISOLATION, core.TCH_CONTROL_FLOW_INTEGRITY, core.TCH_CONFIGURATION_INTEGRITY_VERIFICATION},
		Quantifier: func(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion {

			//TODO: implement
			return &FullUncertainty
		},
	}

	//quantifies the position opinion a vehicle V_x sends about itself in its CAMs. CAMs do not pass through a trust source
	//handler, so each TMI applies this quantifier itself when handling a RefreshCAM update (see TrustModelInstance.Update)
	camQuantifier := core.TrustSourceQuantifier{
		Trustor:     "V_*",
		Trustee:     "C_*_*",
		Scope:       "C_*_*",
		TrustSource: core.V2X,
		Evidence:    []core.EvidenceType{core.V2X_POSITION_OPINION},
		Quantifier: func(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion {

			//Return first entry
			for _, opinion := range m {
				return opinion.(subjectivelogic.QueryableOpinion)
			}
			return &FullUncertainty
		},
	}

	return []core.TrustSourceQuantifier{camQuantifier, tchQuantifier, mbdQuantifier}, nil
}

/*
mbdEvidence are the residuals between the prediction of the Python-based MBD and the received CAM, one per predicted
feature, each normalized to [0,1].
*/
var mbdEvidence = []core.EvidenceType{
	core.MBD_RELATIVE_POSITION_ERROR_X,
	core.MBD_RELATIVE_POSITION_ERROR_Y,
	core.MBD_SENDER_SPEED_ERROR_X,
	core.MBD_SENDER_SPEED_ERROR_Y,
	core.MBD_SENDER_ACCELERATION_ERROR_X,
	core.MBD_SENDER_ACCELERATION_ERROR_Y,
	core.MBD_DISTANCE_TO_ROAD_EDGE_ERROR,
	core.MBD_RECEIVER_TIME_ERROR,
	core.MBD_SENDER_HEADING_ERROR_SIN,
	core.MBD_SENDER_HEADING_ERROR_COS,
}

// mbdParamJSON returns the JSON content set in TRUST_QUANTIFICATION_MECHANISM_MBD_PARAM
func mbdParamJSON(params map[string]string) string {
	paramJSON, exists := params["TRUST_QUANTIFICATION_MECHANISM_MBD_PARAM"]
	if !exists {
		panic("No value set for TRUST_QUANTIFICATION_MECHANISM_MBD_PARAM")
	}
	return paramJSON
}

// mbdFeatureQuantifier creates a quantifier that derives the six MBD features from the evidence
// and maps them to an opinion using the given quantification approach
func mbdFeatureQuantifier(compute func([mbdNumFeatures]float64) mbdOpinion) func(map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion {
	return func(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion {
		features, err := mbdFeatures(m)
		if err != nil {
			return &FullUncertainty
		}
		op := compute(features)
		//Normalize, as clamping u (B-spline/MLP u_min) can violate b+d+u=1
		sum := op.B + op.D + op.U
		opinion, err := subjectivelogic.NewOpinion(op.B/sum, op.D/sum, op.U/sum, 0.5)
		if err != nil {
			return &FullUncertainty
		}
		return &opinion
	}
}

func init() {
}
