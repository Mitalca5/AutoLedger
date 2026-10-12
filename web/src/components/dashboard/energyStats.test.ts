import { describe, expect, it } from 'vitest'
import { mergeAcDcClasses, type ChargeClass } from './energyStats'

const cls = (over: Partial<ChargeClass>): ChargeClass => ({ class: 'AC', sessions: 0, kwh_added: 0, energy_cost: 0, priced_kwh: 0, sessions_without_cost: 0, ...over })

describe('mergeAcDcClasses', () => {
  it('merges the domestic socket into AC, keeps DC separate', () => {
    const slow = cls({ class: 'SLOW', sessions: 10, kwh_added: 99, priced_kwh: 99, energy_cost: 6.08, price_per_kwh: 0.132, charge_efficiency: 0.87 })
    const ac = cls({ class: 'AC', sessions: 75, kwh_added: 1452, priced_kwh: 1452, energy_cost: 8.2 * 75, price_per_kwh: 0.139, charge_efficiency: 0.95 })
    const dc = cls({ class: 'DC', sessions: 30, kwh_added: 832, priced_kwh: 832, energy_cost: 832 * 0.273, price_per_kwh: 0.273, charge_efficiency: 0.93 })
    const unknown = cls({ class: 'UNKNOWN', sessions: 5, kwh_added: 0, energy_cost: 0 })

    const { classes, unknownSessions } = mergeAcDcClasses([slow, ac, dc, unknown])

    expect(classes.map((c) => c.class)).toEqual(['AC', 'DC'])
    const mergedAc = classes[0]
    expect(mergedAc.sessions).toBe(85)
    expect(mergedAc.kwh_added).toBeCloseTo(1551, 5)
    expect(mergedAc.energy_cost).toBeCloseTo(slow.energy_cost + ac.energy_cost, 5)
    expect(mergedAc.price_per_kwh).toBeCloseTo(mergedAc.energy_cost / 1551, 5)
    // Weighted by kWh: mostly the AC (wallbox) efficiency, slightly pulled down by the slow one
    expect(mergedAc.charge_efficiency).toBeGreaterThan(0.94)
    expect(mergedAc.charge_efficiency).toBeLessThan(0.95)
    expect(classes[1]).toEqual(dc)
    expect(unknownSessions).toBe(5)
  })

  it('keeps a class missing entirely out of the result, without crashing', () => {
    const dc = cls({ class: 'DC', sessions: 3, kwh_added: 50, priced_kwh: 50, energy_cost: 10 })
    expect(mergeAcDcClasses([dc]).classes).toEqual([{ ...dc, class: 'DC' }])
    expect(mergeAcDcClasses([]).classes).toEqual([])
    expect(mergeAcDcClasses([]).unknownSessions).toBe(0)
  })

  it('does not divide by zero when a merged class has no kWh', () => {
    const slow = cls({ class: 'SLOW', sessions: 1, kwh_added: 0, energy_cost: 0 })
    const merged = mergeAcDcClasses([slow]).classes[0]
    expect(merged.price_per_kwh).toBeUndefined()
    expect(merged.charge_efficiency).toBeUndefined()
    expect(merged.cost_per_full_charge).toBeUndefined()
  })

  it('prices AC over the kWh that have a cost, so a session without a cost does not lower the price', () => {
    // One 50 kWh session paid 10 EUR (0,20 EUR/kWh) and one 50 kWh session without a cost.
    const ac = cls({ class: 'AC', sessions: 2, kwh_added: 100, priced_kwh: 50, sessions_without_cost: 1, energy_cost: 10, price_per_kwh: 0.2 })
    const merged = mergeAcDcClasses([ac]).classes[0]
    expect(merged.price_per_kwh).toBeCloseTo(0.2, 6)
    expect(merged.sessions_without_cost).toBe(1)
  })

  it('merges the socket and AC prices over their priced kWh only', () => {
    const slow = cls({ class: 'SLOW', sessions: 2, kwh_added: 40, priced_kwh: 20, sessions_without_cost: 1, energy_cost: 4, price_per_kwh: 0.2 })
    const ac = cls({ class: 'AC', sessions: 1, kwh_added: 30, priced_kwh: 30, energy_cost: 9, price_per_kwh: 0.3 })
    const merged = mergeAcDcClasses([slow, ac]).classes[0]
    expect(merged.price_per_kwh).toBeCloseTo(13 / 50, 6)
    expect(merged.priced_kwh).toBe(50)
  })

  it('keeps a known free class at zero and an all-unpriced class unavailable', () => {
    const free = cls({ class: 'AC', sessions: 1, kwh_added: 20, priced_kwh: 20, energy_cost: 0, price_per_kwh: 0 })
    expect(mergeAcDcClasses([free]).classes[0].price_per_kwh).toBe(0)
    const unpriced = cls({ class: 'AC', sessions: 1, kwh_added: 20, priced_kwh: 0, sessions_without_cost: 1, energy_cost: 0 })
    expect(mergeAcDcClasses([unpriced]).classes[0].price_per_kwh).toBeUndefined()
  })

  it('weights the full-charge cost by the state-of-charge points behind it, not by sessions', () => {
    const slow = cls({ class: 'SLOW', sessions: 10, kwh_added: 100, priced_kwh: 100, energy_cost: 10, soc_swing_priced: 100, cost_per_full_charge: 10 })
    const ac = cls({ class: 'AC', sessions: 1, kwh_added: 60, priced_kwh: 60, energy_cost: 12, soc_swing_priced: 60, cost_per_full_charge: 20 })
    const merged = mergeAcDcClasses([slow, ac]).classes[0]
    // 22 EUR over 160 points = 13,75 EUR for 100 points; a session-weighted mean would give 10,9
    expect(merged.cost_per_full_charge).toBeCloseTo(13.75, 6)
  })
})
