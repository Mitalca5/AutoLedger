import { describe, expect, it } from 'vitest'
import { cheapestPerKmIds, rankFleetVehicles } from './fleetRanking'

const v = (id: string, currency: string, perKm: number | null, comparable = true) => ({
  vehicle_id: id,
  currency,
  comparable,
  full_cost_per_km: perKm,
})

describe('rankFleetVehicles', () => {
  it('ranks inside each currency and lists the fleet currency first', () => {
    const ranked = rankFleetVehicles([v('chf', 'CHF', 0.1), v('eur-b', 'EUR', 0.4), v('eur-a', 'EUR', 0.2)], 'EUR')
    expect(ranked.map((x) => x.vehicle_id)).toEqual(['eur-a', 'eur-b', 'chf'])
  })
  it('puts comparable vehicles first, then those without a cost, then the indicative ones', () => {
    const ranked = rankFleetVehicles([v('none', 'EUR', null), v('rough', 'EUR', 0.05, false), v('ok', 'EUR', 0.3)], 'EUR')
    expect(ranked.map((x) => x.vehicle_id)).toEqual(['ok', 'none', 'rough'])
  })
})

describe('cheapestPerKmIds', () => {
  it('flags one vehicle per currency, only when two can be told apart', () => {
    const ranked = rankFleetVehicles([v('a', 'EUR', 0.2), v('b', 'EUR', 0.4), v('chf', 'CHF', 0.1)], 'EUR')
    expect([...cheapestPerKmIds(ranked)]).toEqual(['a'])
  })
})
