import { describe, expect, it } from 'vitest'
import { isActivationKey } from './clickable'

describe('isActivationKey', () => {
  it('accepts Enter and Space', () => {
    expect(isActivationKey('Enter')).toBe(true)
    expect(isActivationKey(' ')).toBe(true)
  })
  it('ignores every other key', () => {
    expect(isActivationKey('Tab')).toBe(false)
    expect(isActivationKey('Escape')).toBe(false)
    expect(isActivationKey('a')).toBe(false)
  })
})
