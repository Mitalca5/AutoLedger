// Projection currency is saved explicitly. A tracked scenario may also be read from its immutable vehicle currency.
export function comparisonCurrency(scenario: any, vehicles: { id: string; currency: string }[]): string | undefined {
  return scenario?.currency || (scenario?.vehicle_id ? vehicles.find(v => v.id === scenario.vehicle_id)?.currency : undefined)
}
export function canCompareCurrencies(scenarios: any[], vehicles: { id: string; currency: string }[]): boolean {
  const currencies = scenarios.map(s => comparisonCurrency(s, vehicles))
  return currencies.length > 0 && currencies.every(c => !!c && c === currencies[0])
}
