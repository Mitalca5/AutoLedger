import { describe, expect, it } from 'vitest'
import { comparablePriceHistory, isUnusualEnergyPrice } from './energyPriceWarning'

describe('relative energy price warnings', () => {
  it('warns strictly above three times the median, for any currency denomination', () => {
    for (const median of [0.25, 1.7, 1400]) {
      const reference = { count: 5, median }
      expect(isUnusualEnergyPrice(median * 2.99, reference)).toBe(false)
      expect(isUnusualEnergyPrice(median * 3, reference)).toBe(false)
      expect(isUnusualEnergyPrice(median * 3.01, reference)).toBe(true)
    }
  })

  it('requires five usable observations and a positive finite median', () => {
    expect(isUnusualEnergyPrice(1700, { count: 4, median: 1.7 })).toBe(false)
    for (const median of [null, 0, -1, NaN, Infinity]) {
      expect(isUnusualEnergyPrice(1700, { count: 5, median })).toBe(false)
    }
    expect(isUnusualEnergyPrice(1700, undefined)).toBe(false)
    expect(isUnusualEnergyPrice(0, { count: 5, median: 1.7 })).toBe(false)
    expect(isUnusualEnergyPrice(NaN, { count: 5, median: 1.7 })).toBe(false)
  })

  it('does not compare a saved projection with another currency', () => {
    const history = { currency: 'ARS', fuel: { count: 5, median: 1400 }, electricity: { count: 0, median: null } }
    expect(comparablePriceHistory(history, 'USD')).toBeNull()
    expect(comparablePriceHistory(history, 'ARS')).toBe(history)
    expect(comparablePriceHistory(null, 'ARS')).toBeNull()
  })
})
