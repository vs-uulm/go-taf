package leftturnassist_v0_1_0

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"sort"
	"strings"

	"github.com/vs-uulm/go-subjectivelogic/pkg/subjectivelogic"
	"github.com/vs-uulm/go-taf/pkg/core"
	"github.com/vs-uulm/go-taf/pkg/trustmodel/trustmodelstructure"
	"github.com/vs-uulm/go-taf/pkg/trustmodel/trustmodelupdate"
)

type TrustModelInstance struct {
	id       string
	version  int
	template TrustModelTemplate

	targetVehicleID string // ID x of vehicle V_{x} that is the source of the data

	tchOpinion subjectivelogic.QueryableOpinion // Opinion V_ego -> V_{sourceID}
	ntmOpinion subjectivelogic.QueryableOpinion // Opinion V_{sourceID} -> C_sourceID_{X}
	mbdOpinion subjectivelogic.QueryableOpinion // Opinion V_ego -> C_sourceID_{X}

	currentStructure   trustmodelstructure.TrustGraphStructure
	currentValues      map[string][]trustmodelstructure.TrustRelationship
	currentFingerprint uint32
	rtls               map[string]subjectivelogic.QueryableOpinion
	staticRTL          subjectivelogic.QueryableOpinion

	fusionOperator      trustmodelstructure.FusionOperator
	fusionWeightEgo     float64 // weight of V_ego's own opinions for ProportionalFusion, opinions via V_{targetVehicleID} get 1-fusionWeightEgo
	discountingOperator trustmodelstructure.DiscountOperator

	trustDecision func(proposition string, atl subjectivelogic.QueryableOpinion, rtl subjectivelogic.QueryableOpinion) core.TrustDecision
}

func (tmi *TrustModelInstance) Decide(proposition string, atl subjectivelogic.QueryableOpinion, rtl subjectivelogic.QueryableOpinion) core.TrustDecision {
	return tmi.trustDecision(proposition, atl, rtl)
}

func (e *TrustModelInstance) ID() string {
	return e.id
}

func (e *TrustModelInstance) Version() int {
	return e.version
}

func (e *TrustModelInstance) Fingerprint() uint32 {
	return e.currentFingerprint
}

func (e *TrustModelInstance) Template() core.TrustModelTemplate {
	return e.template
}

func (e *TrustModelInstance) Update(update core.Update) bool {
	oldVersion := e.Version()
	switch update := update.(type) {
	case trustmodelupdate.RefreshCAM:
		if update.SourceID() == e.targetVehicleID {
			camOpinion, err := subjectivelogic.NewOpinion(
				update.Opinion().Belief,
				update.Opinion().Disbelief,
				update.Opinion().Uncertainty,
				update.Opinion().BaseRate,
			)
			if err == nil {

				e.ntmOpinion = &camOpinion
				e.updateValues()
				e.incrementVersion()
			}
		}
	case trustmodelupdate.UpdateAtomicTrustOpinion:
		trustee := update.Trustee()
		if strings.HasPrefix(trustee, "V_") || strings.HasPrefix(trustee, "vehicle_") {
			id, err := parseVehicleIdentifier(trustee)
			if err == nil && id == e.targetVehicleID {
				e.tchOpinion = update.Opinion()
				e.updateValues()
				e.incrementVersion()
			}
		} else if strings.HasPrefix(trustee, "C_") {
			_, _, err := parseObjectIdentifier(trustee)
			if err == nil {
				e.mbdOpinion = update.Opinion()
				e.updateValues()
				e.incrementVersion()
			}
		}
	default:
		//ignore
	}
	return oldVersion != e.Version() //when version has changed, indicate to run TLEE
}

func (e *TrustModelInstance) incrementVersion() int {
	e.version = e.version + 1
	return e.version
}

/*
processTopologyUpdate reflects changes in the internal topology based upon the latest objects received in an update.
In case there are topology changes due to the update, the function returns true. Otherwise, if the topology is not affected, it returns false.
*/
func (e *TrustModelInstance) processTopologyUpdate(latestObjects []string) bool {
	return false
}

/*
updateFingerprint calculates the current fingerprint for the TMI.
Therefore, it takes the all the dynamic nodes, concatenates their
sorted string identifiers and calculates a hash value.
*/
func (e *TrustModelInstance) updateFingerprint() {
	nodes := make([]string, 0)
	nodes = append(nodes, e.targetVehicleID)

	sort.Strings(nodes)
	stringFingerprint := strings.Join(nodes, "")

	algorithm := fnv.New32a()
	_, err := algorithm.Write([]byte(stringFingerprint))
	if err == nil {
		e.currentFingerprint = algorithm.Sum32()
	}
}

/*
updateStructure updates the internally kept structure according to the latest topology.
*/
func (e *TrustModelInstance) updateStructure() {
	ego := vehicleIdentifier("ego")
	target := vehicleIdentifier(e.targetVehicleID)
	observation := objectIdentifier(e.targetVehicleID, e.targetVehicleID)

	egoToTarget := trustmodelstructure.NewAdjacencyEntryDTO(ego, []string{target})
	egoToObservation := trustmodelstructure.NewAdjacencyEntryDTO(ego, []string{observation})
	if e.fusionOperator == trustmodelstructure.ProportionalFusion {
		egoToTarget = trustmodelstructure.NewWeightedAdjacencyEntryDTO(ego, map[string]float64{target: 1 - e.fusionWeightEgo})
		egoToObservation = trustmodelstructure.NewWeightedAdjacencyEntryDTO(ego, map[string]float64{observation: e.fusionWeightEgo})
	}

	e.currentStructure = trustmodelstructure.NewTrustGraphDTO(e.fusionOperator, e.discountingOperator, []trustmodelstructure.AdjacencyListEntry{
		egoToTarget,
		egoToObservation,
		trustmodelstructure.NewAdjacencyEntryDTO(target, []string{observation}),
	})
}

/*
updateValues updates the internally kept values according to the latest state. Will also dynamically set RTL map to fixed RTL.
*/
func (e *TrustModelInstance) updateValues() {
	values := make(map[string][]trustmodelstructure.TrustRelationship)
	rtls := make(map[string]subjectivelogic.QueryableOpinion)

	ego := vehicleIdentifier("ego")
	target := vehicleIdentifier(e.targetVehicleID)
	observation := objectIdentifier(e.targetVehicleID, e.targetVehicleID)
	scope := observation

	//set values
	values[scope] = []trustmodelstructure.TrustRelationship{
		trustmodelstructure.NewTrustRelationshipDTO(ego, target, e.tchOpinion),
		trustmodelstructure.NewTrustRelationshipDTO(ego, observation, e.mbdOpinion),
		trustmodelstructure.NewTrustRelationshipDTO(target, observation, e.ntmOpinion),
	}

	//set RTL
	rtls[observation] = &RTL

	e.currentValues = values
	e.rtls = rtls
}

func (e *TrustModelInstance) Initialize(params map[string]interface{}) {
	//If a source ID has been defined, use it; otherwise, use ID of TMI
	sourceId, exists := params["SourceId"]
	if !exists {
		e.targetVehicleID = e.id
	} else {
		e.targetVehicleID = sourceId.(string)
	}

	e.version = 0
	e.currentFingerprint = 0
	e.rtls = map[string]subjectivelogic.QueryableOpinion{}
	e.tchOpinion = &FullUncertainty
	e.ntmOpinion = &FullUncertainty
	e.mbdOpinion = &FullUncertainty

	e.updateStructure()
	e.updateFingerprint()
	e.updateValues()
	return
}

func (e *TrustModelInstance) Cleanup() {
	//nothing to do here (yet)
	return
}

func (e *TrustModelInstance) Structure() trustmodelstructure.TrustGraphStructure {
	return e.currentStructure
}

func (e *TrustModelInstance) Values() map[string][]trustmodelstructure.TrustRelationship {

	/*
		//This code is a quick-n-dirty fix to set V-ego => V_X to full belief and V_X to C_X_? to the value of V-ego => V_X
			modifiedValues := make(map[string][]trustmodelstructure.TrustRelationship)
			for k, v := range e.currentValues {
				rels := make([]trustmodelstructure.TrustRelationship, 0)

				var egoToVehicle subjectivelogic.QueryableOpinion
				for _, rel := range v {
					if rel.Source() == "V_ego" && strings.HasPrefix(rel.Destination(), "V_") {
						egoToVehicle = rel.Opinion()
					}
				}

				for _, rel := range v {
					if rel.Source() == "V_ego" && strings.HasPrefix(rel.Destination(), "V_") {
						rels = append(rels, internaltrustmodelstructure.NewTrustRelationshipDTO(rel.Source(), rel.Destination(), &FullBelief))
					} else if strings.HasPrefix(rel.Source(), "V_") && strings.HasPrefix(rel.Destination(), "C_") {
						rels = append(rels, internaltrustmodelstructure.NewTrustRelationshipDTO(rel.Source(), rel.Destination(), egoToVehicle))
					} else {
						rels = append(rels, rel)
					}
				}
				modifiedValues[k] = rels
			}
			return modifiedValues
	*/
	return e.currentValues
}

func (e *TrustModelInstance) RTLs() map[string]subjectivelogic.QueryableOpinion {
	return e.rtls
}

/*
vehicleIdentifier is a helper function to turn a plain identifier into an identifier for vehicles used in the structure.
*/
func vehicleIdentifier(id string) string {
	return fmt.Sprintf("V_%s", id)
}

/*
objectIdentifier is a helper function to turn a plain identifier into an identifier for objects/observations used in the structure.
*/
func objectIdentifier(id string, source string) string {
	return fmt.Sprintf("C_%s_%s", source, id)
}

var objectIdentifierPattern = regexp.MustCompile(`^C_(\d+)_(\d+)$`)
var vehicleIdentifierPattern = regexp.MustCompile(`^(?:V|vehicle)_(\d+|ego).*$`)

/*
parseObjectIdentifier is a helper function to extract plain identifiers from an object identifier string.
*/
func parseObjectIdentifier(str string) (string, string, error) {
	res := objectIdentifierPattern.FindStringSubmatch(str)
	if res != nil && len(res) == 3 {
		return res[1], res[2], nil
	} else {
		return "", "", fmt.Errorf("Invalid object identifier '%s'", str)
	}
}

/*
parseVehicleIdentifier is a helper function to extract plain identifiers from a vehicle identifier string.
*/
func parseVehicleIdentifier(str string) (string, error) {
	res := vehicleIdentifierPattern.FindStringSubmatch(str)
	if res != nil && len(res) == 2 {
		return res[1], nil
	} else {
		return "", fmt.Errorf("Invalid vehicle identifier '%s'", str)
	}
}

func (e *TrustModelInstance) String() string {
	return core.TMIAsString(e)
}
