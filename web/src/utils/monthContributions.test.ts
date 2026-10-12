import { describe, expect, it } from 'vitest'
import { contributionCategory, contributionRoute, recurrenceLabel } from './monthContributions'

describe('contributionCategory', () => {
  it('maps the drillable rows and ignores the others', () => {
    expect(contributionCategory('energy')).toBe('ENERGY')
    expect(contributionCategory('insurance')).toBe('INSURANCE')
    expect(contributionCategory('tires')).toBeNull()
    expect(contributionCategory('constructor')).toBeNull()
  })
})

describe('contributionRoute', () => {
  it('sends charging and fuel records to the energy page and the rest to expenses', () => {
    expect(contributionRoute('charge_logs')).toBe('/energy')
    expect(contributionRoute('fuel_logs')).toBe('/energy')
    expect(contributionRoute('maintenance_expenses')).toBe('/expenses')
  })
})

describe('recurrenceLabel', () => {
  it('uses dedicated wording for monthly and yearly intervals', () => {
    expect(recurrenceLabel(1).key).toMatch(/recurringMonthly$/)
    expect(recurrenceLabel(12).key).toMatch(/recurringYearly$/)
  })
  it('passes the interval for the other values', () => {
    expect(recurrenceLabel(3)).toEqual({ key: 'dashboard.monthDetailModal.contributions.recurring', params: { n: 3 } })
  })
})
