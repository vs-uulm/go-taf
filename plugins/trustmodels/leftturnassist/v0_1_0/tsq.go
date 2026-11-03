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
				panic(fmt.Sprintf("Could not load %s MBD quantification from TRUST_QUANTIFICATION_MECHANISM_MBD_PARAM: %v", mbdTsqMechanism, err))
			}
			mbdQuantify = mbdFeatureQuantifier(slParams.computeTrustOpinion)
		case "b-spline":
			//TRUST_QUANTIFICATION_MECHANISM_MBD_PARAM: content of the B-spline model JSON file (bspline_model.json)
			bsplineModel, err := loadMbdBsplineModel(mbdParamJSON(params))
			if err != nil {
				panic(fmt.Sprintf("Could not load %s MBD quantification from TRUST_QUANTIFICATION_MECHANISM_MBD_PARAM: %v", mbdTsqMechanism, err))
			}
			mbdQuantify = mbdFeatureQuantifier(bsplineModel.computeTrustOpinion)
		case "mlp":
			//TRUST_QUANTIFICATION_MECHANISM_MBD_PARAM: content of the MLP model JSON file (mlp_model.json)
			mlpModel, err := loadMbdMlpModel(mbdParamJSON(params))
			if err != nil {
				panic(fmt.Sprintf("Could not load %s MBD quantification from TRUST_QUANTIFICATION_MECHANISM_MBD_PARAM: %v", mbdTsqMechanism, err))
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
		Evidence:    tchEvidence,
		Quantifier:  quantifyTCH,
	}

	//quantifies the position opinion a vehicle V_x sends about itself in its CAMs. CAMs do not pass through a trust source
	//handler, so each TMI applies this quantifier itself when handling a UpdateCamOpinion update (see TrustModelInstance.Update)
	camQuantifier := core.TrustSourceQuantifier{
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

/*
tchEvidence are the claims attested by the TCH that protect the chain from the firmware start to the signed position
message of a vehicle. Each claim has the same weight.
*/
var tchEvidence = []core.EvidenceType{
	core.TCH_SECURE_BOOT,
	core.TCH_KEY_PROTECTION,
	core.TCH_CONTROL_FLOW_INTEGRITY,
	core.TCH_CONFIGURATION_INTEGRITY_VERIFICATION,
	core.TCH_COMMUNICATION_PROTECTION,
	core.TCH_TAMPER_PROOF_CHECK,
}

// tchBaseClaims are the claims whose failure invalidates all remaining claims, as the firmware or the attestation
// keys may be compromised
var tchBaseClaims = map[core.EvidenceType]bool{
	core.TCH_SECURE_BOOT:    true,
	core.TCH_KEY_PROTECTION: true,
}

// tchInvalidatingClaims are claims whose violation also devalues the remaining claims by the given factor, as
// those attest a platform that is demonstrably manipulated; the graded form of a base claim
var tchInvalidatingClaims = map[core.EvidenceType]float64{
	core.TCH_TAMPER_PROOF_CHECK: 0.8,
}

// tchMaxCertainty limits belief + disbelief of the TCH opinion; the remainder is uncertainty, as a report is a
// finite observation and cannot establish that the attested claims cover the compromise, that the measured state
// still holds, or that the root of trust is itself intact
const tchMaxCertainty = 0.9

/*
quantifyTCH maps the appraisals of the attested claims to an opinion:

	 1 (claim verified)          -> belief += w
	 0 (attestation failed)      -> disbelief += w; for a base claim: belief = 0, disbelief = tchMaxCertainty,
	                                uncertainty = 1 - tchMaxCertainty; for an invalidating claim with factor f:
	                                disbelief = f*tchMaxCertainty, the remaining claims' belief and disbelief scaled
	                                to share (1-f)*tchMaxCertainty
	-1 (control not implemented) -> w remains uncertainty
	-2 or no evidence            -> w remains uncertainty

with w = tchMaxCertainty/len(tchEvidence). The meaning of the appraisals corresponds to the IMA trust model
(trustmodel-ima-standalone-v0.0.1): base claims correspond to output weight 2, all other claims to output weight 1.
*/
func quantifyTCH(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion {
	weight := tchMaxCertainty / float64(len(tchEvidence))
	belief := 0.0
	disbelief := 0.0
	invalidation := 0.0
	invalidatingWeight := 0.0

	for _, claim := range tchEvidence {
		appraisal, ok := m[claim].(int)
		if !ok {
			continue
		}
		switch appraisal {
		case 1:
			belief += weight
		case 0:
			if tchBaseClaims[claim] {
				opinion, _ := subjectivelogic.NewOpinion(0, tchMaxCertainty, 1-tchMaxCertainty, 0.5)
				return &opinion
			}
			if factor, invalidating := tchInvalidatingClaims[claim]; invalidating {
				invalidation = max(invalidation, factor)
				invalidatingWeight += weight
				continue
			}
			disbelief += weight
		}
		//-1 and any other value leave the claim's weight as uncertainty: no evidence in either direction
	}

	if invalidation > 0 {
		//the remaining claims attest a platform that is demonstrably manipulated, so they share only (1-f) of the
		//certainty, in proportion to their own belief and disbelief
		scale := 0.0
		if remaining := tchMaxCertainty - invalidatingWeight; remaining > 0 {
			scale = (1 - invalidation) * tchMaxCertainty / remaining
		}
		belief *= scale
		disbelief = invalidation*tchMaxCertainty + disbelief*scale
	}

	opinion, err := subjectivelogic.NewOpinion(belief, disbelief, max(0, 1-belief-disbelief), 0.5)
	if err != nil {
		return &FullUncertainty
	}
	return &opinion
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
