import { describe, expect, it } from 'vitest'
import { breakEvenDelta, breakEvenReasonKey } from './comparisonBreakEven'

describe('break-even reason', () => {
  it('maps the API reasons to message keys and ignores anything else', () => {
    expect(breakEvenReasonKey('period_too_short')).toBe('comparison.breakEven.reason.period_too_short')
    expect(breakEvenReasonKey('no_saving')).toBe('comparison.breakEven.reason.no_saving')
    expect(breakEvenReasonKey('reversed')).toBe('comparison.breakEven.reason.reversed')
    expect(breakEvenReasonKey(undefined)).toBeNull()
    expect(breakEvenReasonKey('')).toBeNull()
    expect(breakEvenReasonKey('unknown')).toBeNull()
  })
})

describe('break-even delta', () => {
  it('subtracts two reached break-evens, in tenths of a year', () => {
    expect(breakEvenDelta(6.4, 4.4)).toEqual({ kind: 'years', years: -2 })
    expect(breakEvenDelta(4.4, 6.9)).toEqual({ kind: 'years', years: 2.5 })
    expect(breakEvenDelta(3, 3)).toEqual({ kind: 'years', years: 0 })
    expect(breakEvenDelta(0, 0)).toEqual({ kind: 'years', years: 0 })
  })

  it('says when the break-even appears or disappears', () => {
    expect(breakEvenDelta(null, 4.4)).toEqual({ kind: 'reached' })
    expect(breakEvenDelta(4.4, null)).toEqual({ kind: 'lost' })
    expect(breakEvenDelta(null, null)).toEqual({ kind: 'none' })
  })
})
