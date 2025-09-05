package trustmodelstructure

type FusionOperator int

const (
	AveragingFusion FusionOperator = iota
	ConstraintFusion
	CumulativeFusion
	WeightedFusion
	ConsensusAndCompromiseFusion
	NoFusion
)
