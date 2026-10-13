package models

// EnergyPriceReference describes known per-unit prices in the vehicle's currency.
// A nil median means no usable prices; zero is a valid median for free energy.
type EnergyPriceReference struct {
	Count  int      `json:"count"`
	Median *float64 `json:"median"`
}

type ComparisonPriceHistory struct {
	Currency    string               `json:"currency"`
	Fuel        EnergyPriceReference `json:"fuel"`
	Electricity EnergyPriceReference `json:"electricity"`
}
