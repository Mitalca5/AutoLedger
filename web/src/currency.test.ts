import { describe, expect, it } from 'vitest'
import { currencySymbol, formatAmount, formatAxisAmount, formatMoney } from './currency'

describe('formatMoney', () => {
  it('divides cents into the main unit before formatting', () => {
    expect(formatMoney(154000, 'EUR')).toBe(new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR' }).format(1540))
  })

  it('formats a non-euro currency with its own symbol', () => {
    expect(formatMoney(100, 'USD')).toBe(new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'USD' }).format(1))
  })
})

describe('formatAmount', () => {
  it('respects maximumFractionDigits for a per-km rate', () => {
    const got = formatAmount(0.15, 'EUR', 3)
    expect(got).toContain('0,15')
  })
})

describe('currencySymbol', () => {
  it('extracts just the symbol for a chart label', () => {
    expect(currencySymbol('EUR')).toBe('€')
    expect(currencySymbol('USD')).toBe('$US')
  })
})

describe('formatAxisAmount', () => {
  it('never shows decimals on a small tick', () => {
    expect(formatAxisAmount(800, 'EUR')).not.toMatch(/[.,]/)
  })

  it('shortens thousands instead of printing 400 000,00', () => {
    const got = formatAxisAmount(400000, 'EUR')
    expect(got).toContain('400')
    expect(got).not.toMatch(/000/)
    expect(got).not.toMatch(/,00/)
  })
})
