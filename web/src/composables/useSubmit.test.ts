import { describe, expect, it } from 'vitest'
import { useSubmit } from './useSubmit'

describe('useSubmit', () => {
  it('ignores a call made while the first one is pending', async () => {
    const { pending, run } = useSubmit()
    let calls = 0
    let release!: () => void
    const first = run(() => {
      calls++
      return new Promise<string>((resolve) => (release = () => resolve('done')))
    })
    expect(pending.value).toBe(true)
    expect(await run(async () => { calls++; return 'second' })).toBeUndefined()
    release()
    expect(await first).toBe('done')
    expect(calls).toBe(1)
    expect(pending.value).toBe(false)
  })

  it('releases the guard after a failure', async () => {
    const { pending, run } = useSubmit()
    await expect(run(async () => { throw new Error('boom') })).rejects.toThrow('boom')
    expect(pending.value).toBe(false)
    expect(await run(async () => 'ok')).toBe('ok')
  })
})
