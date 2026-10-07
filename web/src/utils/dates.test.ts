import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { setLocale } from '@/i18n'
import { formatDayTime, formatMonthLabel, formatChargeWindow, todayIso, toIsoDay, toLocalDateTimeInput } from './dates'

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

describe('formatChargeWindow', () => {
  const at = (d: number, h: number, m = 0) => new Date(2026, 4, d, h, m).toISOString()

  it('shows the end as a clock time within the same day', () => {
    setLocale('en')
    expect(formatChargeWindow(at(2, 8, 5), at(2, 9, 40))).toMatch(/^02 May 2026, 0?8:05(?: AM)? → 0?9:40(?: AM)?$/)
  })

  it('repeats the end day when the session crosses midnight', () => {
    setLocale('en')
    expect(formatChargeWindow(at(1, 22), at(2, 5, 30))).toMatch(/→ 02 May 2026, /)
  })

  it('falls back to the start when the end is missing or not after it', () => {
    setLocale('en')
    const start = at(2, 8)
    expect(formatChargeWindow(start)).not.toContain('→')
    expect(formatChargeWindow(start, start)).not.toContain('→')
    expect(formatChargeWindow(start, 'nope')).not.toContain('→')
  })
})
