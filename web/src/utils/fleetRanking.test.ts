import { describe, expect, it } from 'vitest'
import { cheapestPerKmIds, mostEconomicalIds, rankFleetVehicles } from './fleetRanking'

const v = (id: string, currency: string, perKm: number | null, comparable = true, per100 = 0) => ({
  vehicle_id: id,
  currency,
  comparable,
  full_cost_per_km: perKm,
  energy_cost_per_100km: per100,
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

describe('mostEconomicalIds', () => {
  it('picks the lowest energy cost per currency and ignores unknown ones', () => {
    const ids = mostEconomicalIds([v('a', 'EUR', null, true, 8), v('b', 'EUR', null, true, 5), v('c', 'CHF', null, true, 30), v('d', 'CHF', null, true, 0)])
    expect([...ids].sort()).toEqual(['b', 'c'])
  })
})
