package trustmodelstructure

type DiscountOperator int

const (
	DefaultDiscount DiscountOperator = iota
	OppositeBeliefDiscount
	UncertaintyFavouringDiscount
	BaseRateSensitiveDiscount
	DisbeliefFavouringDiscount
)
