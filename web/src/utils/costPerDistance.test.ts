import { describe, expect, it } from 'vitest'
import { formatCostPerDistance } from './costPerDistance'

describe('formatCostPerDistance', () => {
  it('shows a dash when there is no cost to divide', () => {
    expect(formatCostPerDistance(0, 'EUR')).toBe('—')
    expect(formatCostPerDistance(undefined, 'EUR')).toBe('—')
    expect(formatCostPerDistance(Number.NaN, 'EUR')).toBe('—')
    expect(formatCostPerDistance(-1, 'EUR', 3, true)).toBe('—')
  })

  it('formats a positive figure, with the unit on request', () => {
    expect(formatCostPerDistance(0.0525, 'EUR', 4)).toMatch(/0,0525/)
    expect(formatCostPerDistance(0.0525, 'EUR', 3, true)).toMatch(/0,053.*\/km/)
  })
})
