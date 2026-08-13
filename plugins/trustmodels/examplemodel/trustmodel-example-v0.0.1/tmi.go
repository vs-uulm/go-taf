package trustmodel_example_v0_0_1

import (
	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/internal/util"
	"github.com/vs-uulm/go-taf/pkg/core"
	"github.com/vs-uulm/go-taf/pkg/trustdecision"
	"github.com/vs-uulm/go-taf/pkg/trustmodel/trustmodelstructure"
	"github.com/vs-uulm/go-taf/pkg/trustmodel/trustmodelupdate"
)

type TrustModelInstance struct {
	id      string
	version int

	template TrustModelTemplate
}

func (e *TrustModelInstance) Decide(atls map[string]subjectivelogic.QueryableOpinion) map[string]core.TrustDecision {
	rtls := e.RTLs()
	trustDecisions := make(map[string]core.TrustDecision, len(atls))
	for proposition, atlOpinion := range atls {
		rtlOpinion, exists := rtls[proposition]
		if !exists {
			//no RTL for proposition found
			trustDecisions[proposition] = core.UNDECIDABLE //If no RTL is found, we set trust decision to UNDECIDABLE as default
		} else {
			trustDecisions[proposition] = trustdecision.DecideByProjectedProbability(atlOpinion, rtlOpinion)
		}
	}
	return trustDecisions
}

func (e *TrustModelInstance) ID() string {
	return e.id
}

func (e *TrustModelInstance) Version() int {
	//TODO implement me
	panic("implement me")
}

func (e *TrustModelInstance) Fingerprint() uint32 {
	//TODO implement me
	panic("implement me")
}

func (e *TrustModelInstance) Structure() trustmodelstructure.TrustGraphStructure {
	//TODO implement me
	panic("implement me")
}

func (e *TrustModelInstance) Values() map[string][]trustmodelstructure.TrustRelationship {
	//TODO implement me
	panic("implement me")
}

func (e *TrustModelInstance) Template() core.TrustModelTemplate {
	return e.template
}

func (e *TrustModelInstance) TrustSourceQuantifiers() []core.TrustSourceQuantifier {
	return []core.TrustSourceQuantifier{}
}

func (e *TrustModelInstance) Update(update core.Update) bool {
	//TODO implement me
	switch update := update.(type) {
	case trustmodelupdate.UpdateAtomicTrustOpinion:
		//TODO
		util.UNUSED(update)
	default:
		//ignore
	}
	return true
}

func (e *TrustModelInstance) Initialize(params map[string]interface{}) {
	return
}

func (e *TrustModelInstance) Cleanup() {
	return
}
func (e *TrustModelInstance) RTLs() map[string]subjectivelogic.QueryableOpinion {
	return map[string]subjectivelogic.QueryableOpinion{}
}

func (e *TrustModelInstance) String() string {
	return core.TMIAsString(e)
}
