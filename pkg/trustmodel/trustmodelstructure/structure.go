package trustmodelstructure

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// A TrustGraphStructure defines the graph-structural properties of a trust model. It does not define scopes, as scopes are only defined for the values of a graph (i.e., trust opinions)
type TrustGraphStructure interface {
	Operator() FusionOperator
	DiscountOperator() DiscountOperator
	AdjacencyList() []AdjacencyListEntry
}

// A AdjacencyListEntry defines all outgoing edges of a source node by listing the corresponding target nodes of these edges.
type AdjacencyListEntry interface {
	SourceNode() string
	TargetNodes() []string
	// FusionWeight returns the weight used by weighted fusion operators (e.g., ProportionalFusion) for all opinions that
	// reach a fused edge via the edge SourceNode()->target, i.e., whose path starts with this edge. Defaults to 1.
	FusionWeight(target string) float64
}

type TrustGraphDTO struct {
	operator         FusionOperator
	discountOperator DiscountOperator
	adjacencyList    []AdjacencyListEntry
}

func NewTrustGraphDTO(operator FusionOperator, discountOperator DiscountOperator, entries []AdjacencyListEntry) *TrustGraphDTO {
	return &TrustGraphDTO{
		operator:         operator,
		discountOperator: discountOperator,
		adjacencyList:    entries,
	}
}

func (t *TrustGraphDTO) Operator() FusionOperator {
	return t.operator
}

func (t *TrustGraphDTO) DiscountOperator() DiscountOperator {
	return t.discountOperator
}

func (t *TrustGraphDTO) AdjacencyList() []AdjacencyListEntry {
	return t.adjacencyList
}

type AdjacencyEntryDTO struct {
	sourceNode  string
	targetNodes []string
	weights     map[string]float64
}

func NewAdjacencyEntryDTO(sourceNode string, targetNodes []string) *AdjacencyEntryDTO {
	return &AdjacencyEntryDTO{
		sourceNode:  sourceNode,
		targetNodes: targetNodes,
	}
}

// NewWeightedAdjacencyEntryDTO creates an adjacency entry whose target nodes are the keys of weightedTargets, each with the
// given fusion weight. Weights must be non-negative, but do not need to sum to 1, as they are normalized upon fusion.
func NewWeightedAdjacencyEntryDTO(sourceNode string, weightedTargets map[string]float64) *AdjacencyEntryDTO {
	targetNodes := make([]string, 0, len(weightedTargets))
	weights := make(map[string]float64, len(weightedTargets))
	for target, weight := range weightedTargets {
		targetNodes = append(targetNodes, target)
		weights[target] = weight
	}
	sort.Strings(targetNodes)
	return &AdjacencyEntryDTO{
		sourceNode:  sourceNode,
		targetNodes: targetNodes,
		weights:     weights,
	}
}

func (a *AdjacencyEntryDTO) SourceNode() string {
	return a.sourceNode
}

func (a *AdjacencyEntryDTO) TargetNodes() []string {
	return a.targetNodes
}

func (a *AdjacencyEntryDTO) FusionWeight(target string) float64 {
	if weight, exists := a.weights[target]; exists {
		return weight
	}
	return 1
}

func DumpStructure(structure TrustGraphStructure) string {
	result := []string{"++ Trust Graph Structure ++"}
	// result = append(result, "Operator: "+structure.Operator()) //TODO: fix
	for _, list := range structure.AdjacencyList() {
		result = append(result, list.SourceNode()+"==>"+fmt.Sprintf("%+v", list.TargetNodes()))
	}
	return strings.Join(result, "\n")
}

func (r *TrustGraphDTO) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Operator      string               `json:"operator"`
		AdjacencyList []AdjacencyListEntry `json:"adjacency_list"`
	}{
		Operator:      "", // TODO: r.Operator(),
		AdjacencyList: r.AdjacencyList(),
	})
}
func (r *AdjacencyEntryDTO) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		SourceNode    string             `json:"sourceNode"`
		TargetNodes   []string           `json:"targetNodes"`
		FusionWeights map[string]float64 `json:"fusionWeights,omitempty"`
	}{
		SourceNode:    r.sourceNode,
		TargetNodes:   r.targetNodes,
		FusionWeights: r.weights,
	})
}
