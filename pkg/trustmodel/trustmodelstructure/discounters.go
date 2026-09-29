package trustmodelstructure

type DiscountOperator int

const (
	DefaultDiscount = iota
	OppositeBeliefDiscount
	UncertaintyFavouringDiscount
	BaseRateSensitiveDiscount
	DisbeliefFavouringDiscount
)
