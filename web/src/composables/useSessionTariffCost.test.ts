import { afterEach, describe, expect, it, vi } from 'vitest'
import { computed, effectScope, ref, type EffectScope } from 'vue'
import { api } from '@/services/api'
import { useSessionTariffCost } from './useSessionTariffCost'
import { useChargeAmounts } from './useChargeAmounts'
import type { SessionPriceRequest } from '@/utils/tariffSession'

const hooks = vi.hoisted(() => [] as (() => void)[])
vi.mock('vue', async (original) => ({ ...await original<typeof import('vue')>(), onBeforeUnmount: (hook: () => void) => hooks.push(hook) }))
vi.mock('@/services/api', () => ({ api: { calculateSessionCost: vi.fn() } }))

const scopes: EffectScope[] = []
const query = { vehicle_id: 'v', start_time: '2026-10-01T10:00:00Z', end_time: '2026-10-01T11:00:00Z', kwh: 10 }

function setup() {
  vi.useFakeTimers()
  const request = ref<SessionPriceRequest | null>({ ...query })
  const enabled = ref(true)
  const cost = ref('')
  const scope = effectScope()
  scopes.push(scope)
  const result = scope.run(() => {
    const tariff = useSessionTariffCost(() => request.value, enabled)
    const amounts = useChargeAmounts(computed(() => String(request.value?.kwh ?? '')), cost, undefined, computed(() => tariff.value?.cost ?? null))
    return { tariff, amounts }
  })!
  return { request, enabled, cost, ...result }
}

afterEach(() => {
  hooks.splice(0).forEach((hook) => hook())
  scopes.splice(0).forEach((scope) => scope.stop())
  vi.useRealTimers()
  vi.resetAllMocks()
})

describe('session tariff cost freshness', () => {
  it('invalidates the old amount immediately, including during debounce and the next request', async () => {
    vi.mocked(api.calculateSessionCost).mockResolvedValueOnce({ cost: 2, plan: 'Plan' }).mockResolvedValueOnce({ cost: 4, plan: 'Plan' })
    const s = setup()
    await vi.advanceTimersByTimeAsync(300)
    expect(s.cost.value).toBe('2.00')
    s.request.value = { ...query, kwh: 20 }
    expect([s.tariff.value, s.cost.value]).toEqual([null, ''])
    await vi.advanceTimersByTimeAsync(299)
    expect(s.cost.value).toBe('')
    await vi.advanceTimersByTimeAsync(1)
    expect(s.cost.value).toBe('4.00')
  })

  it.each(['no plan', 'error', 'invalid query', 'disabled'] as const)('does not retain an old amount after %s', async (reason) => {
    const calculate = vi.mocked(api.calculateSessionCost)
    calculate.mockResolvedValueOnce({ cost: 2, plan: 'Plan' })
    const s = setup()
    await vi.advanceTimersByTimeAsync(300)
    if (reason === 'disabled') s.enabled.value = false
    else if (reason === 'invalid query') s.request.value = null
    else {
      if (reason === 'error') calculate.mockRejectedValueOnce(new Error('Unavailable'))
      else calculate.mockResolvedValueOnce({ cost: 0, plan: null })
      s.request.value = { ...query, kwh: 20 }
    }
    await vi.advanceTimersByTimeAsync(300)
    expect([s.tariff.value, s.cost.value]).toEqual([null, ''])
  })

  it('ignores a late result for a previous query', async () => {
    let resolveOld!: (value: { cost: number; plan: string }) => void
    vi.mocked(api.calculateSessionCost)
      .mockReturnValueOnce(new Promise((resolve) => { resolveOld = resolve }))
      .mockResolvedValueOnce({ cost: 4, plan: 'Current plan' })
    const s = setup()
    await vi.advanceTimersByTimeAsync(300)
    s.request.value = { ...query, kwh: 20 }
    await vi.advanceTimersByTimeAsync(300)
    resolveOld({ cost: 2, plan: 'Old plan' })
    await Promise.resolve()
    expect(s.tariff.value).toEqual({ cost: 4, plan: 'Current plan' })
    expect(s.cost.value).toBe('4.00')
  })

  it('keeps a calculator amount when an in-flight tariff request completes', async () => {
    let resolve!: (value: { cost: number; plan: string }) => void
    vi.mocked(api.calculateSessionCost).mockReturnValueOnce(new Promise((r) => { resolve = r }))
    const s = setup()
    await vi.advanceTimersByTimeAsync(300)
    s.amounts.setCost(7)
    resolve({ cost: 2, plan: 'Plan' })
    await Promise.resolve()
    expect(s.cost.value).toBe('7.00')
  })

  it('does not update state after unmounting with a request in flight', async () => {
    let resolve!: (value: { cost: number; plan: string }) => void
    vi.mocked(api.calculateSessionCost).mockReturnValueOnce(new Promise((r) => { resolve = r }))
    const s = setup()
    await vi.advanceTimersByTimeAsync(300)
    hooks.splice(0).forEach((hook) => hook())
    resolve({ cost: 2, plan: 'Plan' })
    await Promise.resolve()
    expect(s.tariff.value).toBeNull()
  })
})
