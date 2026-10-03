import { describe, expect, it } from 'vitest'
import { wrapFocusIndex } from './dialog'

describe('wrapFocusIndex', () => {
  it('lets the browser move the focus inside the panel', () => {
    expect(wrapFocusIndex(3, 1, false)).toBe(-1)
    expect(wrapFocusIndex(3, 1, true)).toBe(-1)
  })
  it('wraps from the last control to the first and back', () => {
    expect(wrapFocusIndex(3, 2, false)).toBe(0)
    expect(wrapFocusIndex(3, 0, true)).toBe(2)
  })
  it('enters the panel when the focus is outside it', () => {
    expect(wrapFocusIndex(3, -1, false)).toBe(0)
    expect(wrapFocusIndex(3, -1, true)).toBe(2)
  })
  it('does nothing without a focusable control', () => {
    expect(wrapFocusIndex(0, -1, false)).toBe(-1)
  })
})
