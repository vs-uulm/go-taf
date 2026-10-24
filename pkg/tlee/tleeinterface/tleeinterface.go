package tleeinterface

import (
	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/trustmodel/trustmodelstructure"
)

/*
The TLEE interface indicates that a struct can be used as a TLEE by exposing the required RunTLEE() function.
*/
type TLEE interface {

	/*
	   The RunTLEE function provided by the TLEE to be used by the TAF.
	*/
	RunTLEE(trustmodelID string, version int, fingerprint uint32, structure trustmodelstructure.TrustGraphStructure, values map[string][]trustmodelstructure.TrustRelationship) (map[string]subjectivelogic.QueryableOpinion, error)
}
