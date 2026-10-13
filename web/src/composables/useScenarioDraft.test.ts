import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'
import { useScenarioDraft } from './useScenarioDraft'

type Inputs = { km: number; price: number }

function setup(overrides: { preview?: (payload: unknown) => Promise<any>; blocked?: () => boolean; isValid?: (d: Inputs) => boolean } = {}) {
  const base = ref<Inputs>({ km: 10000, price: 1.5 })
  const preview = vi.fn(overrides.preview ?? (async (payload: unknown) => ({ echo: payload })))
  const scope = effectScope()
  const draft = scope.run(() =>
    useScenarioDraft<Inputs>({
      base: () => base.value,
      isValid: overrides.isValid ?? ((d) => d.km > 0),
      payload: (d) => ({ ...d }),
      preview,
      blocked: overrides.blocked,
      debounceMs: 250,
    }),
  )!
  return { base, preview, draft, scope }
}

describe('scenario draft', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  it('sends nothing while the draft equals the saved scenario', async () => {
    const { draft, preview, scope } = setup()
    expect(draft.dirty.value).toBe(false)
    draft.draft.km = 10000
    await nextTick()
    await vi.advanceTimersByTimeAsync(500)
    expect(preview).not.toHaveBeenCalled()
    expect(draft.preview.value).toBeNull()
    scope.stop()
  })

  it('debounces edits and keeps the answer of the latest one', async () => {
    const { draft, preview, scope } = setup()
    draft.draft.km = 12000
    await nextTick()
    await vi.advanceTimersByTimeAsync(100)
    draft.draft.km = 15000
    await nextTick()
    await vi.advanceTimersByTimeAsync(250)
    expect(preview).toHaveBeenCalledTimes(1)
    expect(draft.dirty.value).toBe(true)
    expect(draft.preview.value).toEqual({ echo: { km: 15000, price: 1.5 } })
    expect(draft.loading.value).toBe(false)
    scope.stop()
  })

  it('discards an answer that arrives after a newer edit', async () => {
    let finishFirst!: (v: any) => void
    const preview = vi.fn()
      .mockImplementationOnce(() => new Promise((resolve) => { finishFirst = resolve }))
      .mockImplementationOnce(async () => ({ echo: 'second' }))
    const { draft, scope } = setup({ preview })
    draft.draft.km = 12000
    await nextTick()
    await vi.advanceTimersByTimeAsync(250)
    draft.draft.km = 13000
    await nextTick()
    await vi.advanceTimersByTimeAsync(250)
    expect(draft.preview.value).toEqual({ echo: 'second' })
    finishFirst({ echo: 'first' })
    await vi.advanceTimersByTimeAsync(0)
    expect(draft.preview.value).toEqual({ echo: 'second' })
    scope.stop()
  })

  it('does not send an out-of-range draft and keeps the last answer', async () => {
    const { draft, preview, scope } = setup()
    draft.draft.km = 12000
    await nextTick()
    await vi.advanceTimersByTimeAsync(250)
    expect(preview).toHaveBeenCalledTimes(1)
    draft.draft.km = -5
    await nextTick()
    await vi.advanceTimersByTimeAsync(500)
    expect(draft.valid.value).toBe(false)
    expect(preview).toHaveBeenCalledTimes(1)
    expect(draft.preview.value).toEqual({ echo: { km: 12000, price: 1.5 } })
    scope.stop()
  })

  it('drops the answer and sends nothing while blocked, then evaluates once released', async () => {
    const blocked = ref(false)
    const { draft, preview, scope } = setup({ blocked: () => blocked.value })
    draft.draft.price = 2
    await nextTick()
    await vi.advanceTimersByTimeAsync(250)
    expect(draft.preview.value).not.toBeNull()
    blocked.value = true
    await nextTick()
    expect(draft.preview.value).toBeNull()
    await vi.advanceTimersByTimeAsync(500)
    expect(preview).toHaveBeenCalledTimes(1)
    blocked.value = false
    await nextTick()
    await vi.advanceTimersByTimeAsync(250)
    expect(preview).toHaveBeenCalledTimes(2)
    scope.stop()
  })

  it('clears the answer and returns to the saved values on reset or when the saved scenario changes', async () => {
    const { base, draft, scope } = setup()
    draft.draft.km = 12000
    await nextTick()
    await vi.advanceTimersByTimeAsync(250)
    expect(draft.preview.value).not.toBeNull()
    draft.reset()
    await nextTick()
    expect(draft.draft).toEqual({ km: 10000, price: 1.5 })
    expect(draft.preview.value).toBeNull()
    draft.draft.km = 11000
    base.value = { km: 20000, price: 2 }
    await nextTick()
    expect(draft.draft).toEqual({ km: 20000, price: 2 })
    scope.stop()
  })

  it('flags a failed evaluation without keeping an answer', async () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => {})
    const { draft, scope } = setup({ preview: async () => { throw new Error('offline') } })
    draft.draft.km = 12000
    await nextTick()
    await vi.advanceTimersByTimeAsync(250)
    expect(draft.failed.value).toBe(true)
    expect(draft.preview.value).toBeNull()
    error.mockRestore()
    scope.stop()
  })
})
