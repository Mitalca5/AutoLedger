import { describe, expect, it } from 'vitest'
import { effectScope, ref } from 'vue'
import { useEnergyPriceWarning } from './useEnergyPriceWarning'

describe('energy price confirmation', () => {
  it('allows keeping a high value, then asks again after the price or vehicle changes', () => {
    const scope = effectScope()
    const price = ref(1700)
    const reference = ref({ count: 5, median: 1.7 })
    const vehicle = ref('A')
    const warning = scope.run(() => useEnergyPriceWarning(() => price.value, () => reference.value, () => vehicle.value))!
    expect(warning.needsConfirmation.value).toBe(true)
    warning.confirm()
    expect(warning.needsConfirmation.value).toBe(false)
    expect(price.value).toBe(1700)
    price.value = 1800
    expect(warning.needsConfirmation.value).toBe(true)
    warning.confirm()
    vehicle.value = 'B'
    expect(warning.needsConfirmation.value).toBe(true)
    warning.confirm()
    reference.value = { count: 6, median: 2 }
    expect(warning.needsConfirmation.value).toBe(true)
    price.value = 2
    expect(warning.needsConfirmation.value).toBe(false)
    scope.stop()
  })
})
