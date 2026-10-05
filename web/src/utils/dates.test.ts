import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { setLocale } from '@/i18n'
import { formatDayTime, formatMonthLabel, todayIso, toIsoDay, toLocalDateTimeInput } from './dates'

describe('dates', () => {
  it('gives the UTC day of a date', () => {
    expect(toIsoDay('2026-05-02T23:30:00Z')).toBe('2026-05-02')
    expect(toIsoDay(new Date('2026-05-03T00:00:00Z'))).toBe('2026-05-03')
  })

  it('formats today as YYYY-MM-DD', () => {
    expect(todayIso()).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(todayIso()).toBe(new Date().toISOString().slice(0, 10))
  })

  describe('datetime-local value', () => {
    const tz = process.env.TZ
    beforeAll(() => {
      process.env.TZ = 'Europe/Paris'
    })
    afterAll(() => {
      process.env.TZ = tz
    })

    it('gives the local time, not the UTC one', () => {
      expect(toLocalDateTimeInput('2026-09-01T06:05:00Z')).toBe('2026-09-01T08:05')
      expect(toLocalDateTimeInput('2026-01-31T23:30:00Z')).toBe('2026-02-01T00:30')
    })

    it('reads back as the same instant', () => {
      const saved = '2026-09-01T06:05:00.000Z'
      expect(new Date(toLocalDateTimeInput(saved)).toISOString()).toBe(saved)
    })
  })

  it('formats a short date with the time', () => {
    const s = formatDayTime('2026-05-02T08:30:00')
    expect(s).toContain('02')
    expect(s).toContain('08:30')
  })
})

describe('formatMonthLabel', () => {
  it('renders a month key as a short localized month', () => {
    setLocale('fr')
    expect(formatMonthLabel('2026-02')).toMatch(/^févr\.? 26$/)
    setLocale('en')
    expect(formatMonthLabel('2026-02')).toMatch(/^Feb 26$/)
  })

  it('leaves anything that is not a month key untouched', () => {
    expect(formatMonthLabel('Total')).toBe('Total')
  })
})
