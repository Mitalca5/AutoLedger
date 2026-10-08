import { describe, expect, it } from 'vitest'
import { nextTick, ref } from 'vue'
import { useChargeAmounts } from './useChargeAmounts'

describe('useChargeAmounts', () => {
  it('starts from the suggested price and prices the energy typed', async () => {
    const kwh = ref('')
    const cost = ref('')
    const a = useChargeAmounts(kwh, cost, 0.25)
    expect(a.price.value).toBe('0.25')
    kwh.value = '10'
    await nextTick()
    expect(cost.value).toBe('2.50')
  })

  it('derives the price from a typed cost and keeps it when the energy changes', async () => {
    const kwh = ref('10')
    const cost = ref('')
    const a = useChargeAmounts(kwh, cost, 0.25)
    cost.value = '5'
    a.onCostInput()
    expect(a.price.value).toBe('0.5')
    kwh.value = '20'
    await nextTick()
    expect(a.price.value).toBe('0.25')
    expect(cost.value).toBe('5')
  })

  it('derives the cost from a typed price, and clears it without energy', async () => {
    const kwh = ref('8')
    const cost = ref('')
    const a = useChargeAmounts(kwh, cost)
    a.price.value = '0.3'
    a.onPriceInput()
    expect(cost.value).toBe('2.40')
    kwh.value = ''
    await nextTick()
    expect(cost.value).toBe('')
  })

  it('starts from the price paid when editing a recorded charge, and Free zeroes both', () => {
    const a = useChargeAmounts(ref('10'), ref('4'), 0.9)
    expect(a.price.value).toBe('0.4')
    expect(a.driver.value).toBe('cost')
    const cost = ref('3')
    const b = useChargeAmounts(ref('10'), cost)
    b.setFree()
    expect([cost.value, b.price.value]).toEqual(['0', '0'])
  })

  it('clears an obsolete plan amount and its effective price', () => {
    const cost = ref('')
    const tariff = ref<number | null>(2)
    const a = useChargeAmounts(ref('10'), cost, undefined, tariff)
    expect([cost.value, a.price.value]).toEqual(['2.00', '0.2'])
    tariff.value = null
    expect([cost.value, a.price.value]).toEqual(['', ''])
  })

  it('falls back to the remembered suggestion, not the previous plan rate', () => {
    const kwh = ref('10')
    const cost = ref('')
    const tariff = ref<number | null>(2)
    const a = useChargeAmounts(kwh, cost, 0.3, tariff)
    kwh.value = '20'
    tariff.value = null
    expect([cost.value, a.price.value]).toEqual(['6.00', '0.3'])
  })

  it.each(['typed cost', 'typed price', 'calculator', 'free'] as const)('preserves an explicit %s through tariff updates and invalidation', async (source) => {
    const kwh = ref('10')
    const cost = ref('')
    const tariff = ref<number | null>(2)
    const a = useChargeAmounts(kwh, cost, undefined, tariff)
    if (source === 'typed cost') {
      cost.value = '7'
      a.onCostInput()
    } else if (source === 'typed price') {
      a.price.value = '0.7'
      a.onPriceInput()
    } else if (source === 'calculator') {
      a.setCost(7)
    } else {
      a.setFree()
    }
    const explicit = cost.value
    tariff.value = null
    tariff.value = 3
    expect(cost.value).toBe(explicit)
    kwh.value = '20'
    await nextTick()
    expect(Number(cost.value)).toBe(source === 'typed price' ? 14 : source === 'free' ? 0 : 7)
  })

  it('preserves an existing zero cost when editing a recorded charge', () => {
    const cost = ref('0')
    const tariff = ref<number | null>(4)
    const a = useChargeAmounts(ref('10'), cost, 0.3, tariff)
    tariff.value = null
    expect([cost.value, a.driver.value]).toEqual(['0', 'cost'])
  })

})
