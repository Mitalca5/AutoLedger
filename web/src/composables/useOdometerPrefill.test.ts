import { describe, expect, it } from 'vitest'
import { canPrefillOdometer } from './useOdometerPrefill'

describe('canPrefillOdometer', () => {
  it('fills an empty or zero field', () => {
    expect(canPrefillOdometer('', null)).toBe(true)
    expect(canPrefillOdometer(0, null)).toBe(true)
    expect(canPrefillOdometer(null, null)).toBe(true)
  })
  it('fills a field that still holds the last filled value', () => {
    expect(canPrefillOdometer(12500, 12500)).toBe(true)
    expect(canPrefillOdometer('12500', 12500)).toBe(true)
  })
  it('keeps what the user typed', () => {
    expect(canPrefillOdometer(12501, 12500)).toBe(false)
    expect(canPrefillOdometer(12500, null)).toBe(false)
  })
})
