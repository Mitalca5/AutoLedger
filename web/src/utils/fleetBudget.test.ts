import { describe, expect, it } from 'vitest'
import { budgetUsage, parseBudgetInput } from './fleetBudget'

describe('budgetUsage', () => {
  it('is ok below the warning threshold', () => {
    expect(budgetUsage(100, 250)).toEqual({ percent: 40, status: 'ok', remaining: 150 })
  })
  it('warns from 80% up to the budget included', () => {
    expect(budgetUsage(200, 250).status).toBe('warning')
    expect(budgetUsage(250, 250).status).toBe('warning')
  })
  it('flags an overrun and keeps the real percentage', () => {
    expect(budgetUsage(300, 250)).toEqual({ percent: 120, status: 'over', remaining: -50 })
  })
})

describe('parseBudgetInput', () => {
  it('accepts dot and comma decimals', () => {
    expect(parseBudgetInput('250')).toBe(250)
    expect(parseBudgetInput(' 250,5 ')).toBe(250.5)
  })
  it('rounds to the cent', () => {
    expect(parseBudgetInput('10.456')).toBe(10.46)
  })
  it('refuses empty, zero, negative and non numeric input', () => {
    for (const bad of ['', '0', '-5', 'abc', 'Infinity']) expect(parseBudgetInput(bad)).toBeNull()
  })
})
