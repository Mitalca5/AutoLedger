/** Positive values mean savings against the combustion vehicle, over the whole scenario. */
export function comparisonSavings(reference: number, combustion: number, years: number, annualKm: number) {
  const amount = combustion - reference
  const distance = years * annualKm
  return {
    amount,
    percent: combustion > 0 ? amount / combustion * 100 : null,
    perMonth: years > 0 ? amount / (years * 12) : null,
    perKm: years > 0 && distance > 0 ? amount / distance : null,
  }
}
