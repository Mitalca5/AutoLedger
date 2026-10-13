import { describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'
import { useComparisonPriceHistory } from './useComparisonPriceHistory'

const api = vi.hoisted(() => ({ getComparisonPriceHistory: vi.fn() }))
vi.mock('@/services/api', () => ({ api }))

describe('comparison price history', () => {
  it('discards older responses, clears a previous vehicle and ignores currency mismatches', async () => {
    let finishFirst!: (value: any) => void
    let finishSecond!: (value: any) => void
    api.getComparisonPriceHistory.mockReset()
      .mockImplementationOnce(() => new Promise(resolve => { finishFirst = resolve }))
      .mockImplementationOnce(() => new Promise(resolve => { finishSecond = resolve }))
    const vehicle = ref<string | undefined>('A')
    const scope = effectScope()
    const history = scope.run(() => useComparisonPriceHistory(() => vehicle.value, () => 'DIESEL', () => 'ARS'))!
    const result = { currency: 'ARS', fuel: { count: 5, median: 1400 }, electricity: { count: 0, median: null } }
    vehicle.value = 'B'
    await nextTick()
    finishSecond(result)
    await nextTick()
    expect(history.value).toEqual(result)
    finishFirst({ ...result, currency: 'USD' })
    await nextTick()
    expect(history.value).toEqual(result)
    vehicle.value = undefined
    await nextTick()
    expect(history.value).toBeNull()
    scope.stop()
  })

  it('leaves the advisory unavailable after a request fails', async () => {
    api.getComparisonPriceHistory.mockReset().mockRejectedValue(new Error('offline'))
    const scope = effectScope()
    const history = scope.run(() => useComparisonPriceHistory(() => 'A', () => 'DIESEL', () => 'ARS'))!
    await nextTick()
    expect(history.value).toBeNull()
    scope.stop()
  })
})
