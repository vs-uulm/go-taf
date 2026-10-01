package leftturnassist_v0_1_0

import (
	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/core"
)

func createTrustSourceQuantifiers(params map[string]string) ([]core.TrustSourceQuantifier, error) {

	//quantifies the residuals of the Python-based MBD (see mbdEvidence) with the selected mechanism
	var mbdQuantify func(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion

	mbdTsqMechanism, exists := params["TRUST_QUANTIFICATION_MECHANISM_MBD"]
	if exists {
		switch mbdTsqMechanism {
		case "f1-optimized":
			mbdQuantify = func(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion {
				//TODO: implement
				return &FullUncertainty
			}
		case "analytic":
			mbdQuantify = func(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion {
				//TODO: implement
				return &FullUncertainty
			}
		case "b-spline":
			mbdQuantify = func(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion {
				//TODO: implement
				return &FullUncertainty
			}
		case "mlp":
			mbdQuantify = func(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion {
				//TODO: implement
				return &FullUncertainty
			}
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

	return []core.TrustSourceQuantifier{createCamQuantifier(params), tchQuantifier, mbdQuantifier}, nil
}

/*
createCamQuantifier creates the quantifier for the position opinion a vehicle V_x sends about itself in its CAMs. CAMs do
not pass through a trust source handler, so each TMI applies this quantifier itself when handling a RefreshCAM update
(see TrustModelInstance.Update); the evidence is the received opinion as V2X_POSITION_OPINION.
*/
func createCamQuantifier(params map[string]string) core.TrustSourceQuantifier {
	return core.TrustSourceQuantifier{
		Trustor:     "V_*",
		Trustee:     "C_*_*",
		Scope:       "C_*_*",
		TrustSource: core.V2X,
		Evidence:    []core.EvidenceType{core.V2X_POSITION_OPINION},
		Quantifier: func(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion {

			//TODO: implement, passes the received opinion through for now
			if opinion, ok := m[core.V2X_POSITION_OPINION].(subjectivelogic.QueryableOpinion); ok {
				return opinion
			}
			return &FullUncertainty
		},
	}
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

func init() {
}
