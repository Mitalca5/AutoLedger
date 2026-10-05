import { beforeEach, describe, expect, it } from 'vitest'
import { setLocale } from '@/i18n'
import { formatNumber, formatPercent } from './numbers'

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
