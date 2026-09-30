package leftturnassist_v0_1_0

import (
	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/core"
)

func createTrustSourceQuantifiers(params map[string]string) ([]core.TrustSourceQuantifier, error) {

	mbdTsqMechanism, exists := params["TRUST_QUANTIFICATION_MECHANISM_MBD"]
	if exists {
		switch mbdTsqMechanism {
		case "f1-optimized":
			//TODO
		case "analytic":
			//TODO
		case "b-spline":
			//TODO
		case "mlp":
			//TODO
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
		Evidence:    []core.EvidenceType{core.MBD_MISBEHAVIOR_REPORT},
		Quantifier: func(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion {

			//TODO: implement
			return &FullUncertainty
		},
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

	ntmQuantifier := core.TrustSourceQuantifier{
		Trustor:     "V_*",
		Trustee:     "C_*_*",
		Scope:       "C_*_*",
		TrustSource: core.NTM,
		Evidence:    []core.EvidenceType{core.NTM_REMOTE_OPINION},
		Quantifier: func(m map[core.EvidenceType]interface{}) subjectivelogic.QueryableOpinion {

			//TODO: check implementtion
			//Return first entry
			for _, opinion := range m {
				return opinion.(subjectivelogic.QueryableOpinion)
			}

			return &FullUncertainty
		},
	}

	return []core.TrustSourceQuantifier{ntmQuantifier, tchQuantifier, mbdQuantifier}, nil
}

func init() {
}
