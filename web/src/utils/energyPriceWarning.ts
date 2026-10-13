export interface EnergyPriceReference {
  count: number
  median: number | null
}

export interface ComparisonPriceHistory {
  currency: string
  fuel: EnergyPriceReference
  electricity: EnergyPriceReference
}

export const MIN_PRICE_HISTORY = 5
export const HIGH_PRICE_MULTIPLIER = 3

// Zero medians (mostly free energy) cannot define a meaningful relative ceiling.
// Unknown costs never become zero observations; the API supplies only known prices.
export function isUnusualEnergyPrice(price: number, reference: EnergyPriceReference | undefined): boolean {
  return !!reference && reference.count >= MIN_PRICE_HISTORY && reference.median !== null &&
    Number.isFinite(reference.median) && reference.median > 0 && Number.isFinite(price) &&
    price > reference.median * HIGH_PRICE_MULTIPLIER
}

// History is advisory only, and amounts in different currencies must never be compared.
export function comparablePriceHistory(history: ComparisonPriceHistory | null, currency: string): ComparisonPriceHistory | null {
  return history?.currency === currency ? history : null
}
