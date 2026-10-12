interface RankableVehicle {
  vehicle_id: string
  currency: string
  comparable: boolean
  full_cost_per_km: number | null
  energy_cost_per_100km: number
}

// Amounts in two currencies are never put in one order: vehicles are grouped by currency (the fleet currency first,
// then by first appearance) and ranked inside their group.
export function rankFleetVehicles<T extends RankableVehicle>(vehicles: T[], fleetCurrency: string): T[] {
  const order: string[] = [fleetCurrency]
  for (const v of vehicles) if (!order.includes(v.currency)) order.push(v.currency)
  return [...vehicles].sort((a, b) => {
    if (a.currency !== b.currency) return order.indexOf(a.currency) - order.indexOf(b.currency)
    if (a.comparable !== b.comparable) return a.comparable ? -1 : 1
    // A vehicle without a cost per km yet goes last, never first as if it cost nothing
    if (a.full_cost_per_km == null || b.full_cost_per_km == null) return (a.full_cost_per_km == null ? 1 : 0) - (b.full_cost_per_km == null ? 1 : 0)
    return a.full_cost_per_km - b.full_cost_per_km
  })
}

// The cheapest vehicle per km of each currency that has at least two comparable vehicles to tell apart.
export function cheapestPerKmIds(ranked: RankableVehicle[]): Set<string> {
  const ids = new Set<string>()
  const seen = new Map<string, number>()
  for (const v of ranked) {
    if (!v.comparable) continue
    const n = (seen.get(v.currency) ?? 0) + 1
    seen.set(v.currency, n)
    if (n === 1) ids.add(v.vehicle_id)
  }
  for (const v of ranked) if ((seen.get(v.currency) ?? 0) < 2) ids.delete(v.vehicle_id)
  return ids
}

// The vehicle with the lowest energy cost per 100 km inside each currency.
export function mostEconomicalIds(vehicles: RankableVehicle[]): Set<string> {
  const best = new Map<string, RankableVehicle>()
  for (const v of vehicles) {
    if (!(v.energy_cost_per_100km > 0)) continue
    const cur = best.get(v.currency)
    if (!cur || v.energy_cost_per_100km < cur.energy_cost_per_100km) best.set(v.currency, v)
  }
  return new Set([...best.values()].map((v) => v.vehicle_id))
}
