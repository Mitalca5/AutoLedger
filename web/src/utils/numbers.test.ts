import { beforeEach, describe, expect, it } from 'vitest'
import { setLocale } from '@/i18n'
import { formatDecimalInput, formatNumber, formatPercent, parseDecimal } from './numbers'

const plain = (s: string) => s.replace(/[  ]/g, ' ')

describe('formatNumber', () => {
  it('uses a decimal comma and groups thousands in French', () => {
    setLocale('fr')
    expect(formatNumber(39.25, 2)).toBe('39,25')
    expect(plain(formatNumber(1234.5, 1))).toBe('1 234,5')
    expect(formatNumber(0.22764, 4)).toBe('0,2276')
  })

  it('uses a decimal point in English', () => {
    setLocale('en')
    expect(formatNumber(39.25, 2)).toBe('39.25')
    expect(formatNumber(1234.5, 1)).toBe('1,234.5')
  })
})

describe('formatPercent', () => {
  beforeEach(() => setLocale('fr'))

  it('spaces the percent sign the French way', () => {
    expect(plain(formatPercent(12.5))).toBe('12,5 %')
    expect(plain(formatPercent(88, 0))).toBe('88 %')
  })
})

describe('parseDecimal', () => {
  it('reads a comma or a point as the decimal separator', () => {
    expect(parseDecimal('16,5')).toBe(16.5)
    expect(parseDecimal('16.5')).toBe(16.5)
    expect(parseDecimal('-0,25')).toBe(-0.25)
    expect(parseDecimal(',5')).toBe(0.5)
  })

  it('ignores spaces and a thousands separator when the other sign is the decimal one', () => {
    expect(parseDecimal('1 234,5')).toBe(1234.5)
    expect(parseDecimal('1.234,5')).toBe(1234.5)
    expect(parseDecimal('1,234.5')).toBe(1234.5)
  })

  it('refuses anything that is not a number', () => {
    for (const raw of ['', ' ', '-', 'abc', '1,2,3x', '1e5', '--1']) expect(parseDecimal(raw)).toBeNaN()
  })
})

describe('formatDecimalInput', () => {
  it('writes the language\'s decimal separator without grouping', () => {
    setLocale('fr')
    expect(formatDecimalInput(1234.5)).toBe('1234,5')
    expect(formatDecimalInput(0.2276)).toBe('0,2276')
    setLocale('en')
    expect(formatDecimalInput(1234.5)).toBe('1234.5')
  })
})
