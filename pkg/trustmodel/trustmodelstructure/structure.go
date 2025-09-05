package trustmodelstructure

import (
	"encoding/json"
	"fmt"
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
}

func NewAdjacencyEntryDTO(sourceNode string, targetNodes []string) *AdjacencyEntryDTO {
	return &AdjacencyEntryDTO{
		sourceNode:  sourceNode,
		targetNodes: targetNodes,
	}
}

func (a *AdjacencyEntryDTO) SourceNode() string {
	return a.sourceNode
}

func (a *AdjacencyEntryDTO) TargetNodes() []string {
	return a.targetNodes
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
		SourceNode  string   `json:"sourceNode"`
		TargetNodes []string `json:"targetNodes"`
	}{
		SourceNode:  r.sourceNode,
		TargetNodes: r.targetNodes,
	})
}
