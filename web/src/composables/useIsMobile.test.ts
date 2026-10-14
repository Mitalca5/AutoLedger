import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useIsMobile } from './useIsMobile'

// The suite runs without a DOM: the lifecycle hooks are captured so the test can mount and unmount by hand.
const hooks = vi.hoisted(() => ({ mounted: [] as (() => void)[], unmounted: [] as (() => void)[] }))
vi.mock('vue', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue')>()),
  onMounted: (cb: () => void) => hooks.mounted.push(cb),
  onUnmounted: (cb: () => void) => hooks.unmounted.push(cb),
}))

function fakeQuery(matches: boolean) {
  const listeners = new Set<() => void>()
  return {
    matches,
    addEventListener: (_: string, cb: () => void) => listeners.add(cb),
    removeEventListener: (_: string, cb: () => void) => listeners.delete(cb),
    set(value: boolean) {
      this.matches = value
      listeners.forEach((cb) => cb())
    },
    count: () => listeners.size,
  }
}

describe('useIsMobile', () => {
  beforeEach(() => {
    hooks.mounted = []
    hooks.unmounted = []
  })
  afterEach(() => vi.unstubAllGlobals())

  it('starts from the media query and follows its changes until unmounted', () => {
    const q = fakeQuery(true)
    vi.stubGlobal('window', { matchMedia: () => q })
    const isMobile = useIsMobile()
    expect(isMobile.value).toBe(true)

    hooks.mounted.forEach((cb) => cb())
    expect(q.count()).toBe(1)
    q.set(false)
    expect(isMobile.value).toBe(false)
    q.set(true)
    expect(isMobile.value).toBe(true)

    hooks.unmounted.forEach((cb) => cb())
    expect(q.count()).toBe(0)
  })

  it('is false where matchMedia does not exist', () => {
    vi.stubGlobal('window', {})
    const isMobile = useIsMobile()
    hooks.mounted.forEach((cb) => cb())
    hooks.unmounted.forEach((cb) => cb())
    expect(isMobile.value).toBe(false)
  })

  it('is false without a window', () => {
    vi.stubGlobal('window', undefined)
    expect(useIsMobile().value).toBe(false)
  })
})
