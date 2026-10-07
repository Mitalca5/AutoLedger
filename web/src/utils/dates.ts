import { intlLocale } from '@/i18n'

/** Today as YYYY-MM-DD (UTC), the format the date pickers and the API use. */
export const todayIso = (): string => new Date().toISOString().substring(0, 10)

/** The YYYY-MM-DD (UTC) day of a date or an ISO string. */
export const toIsoDay = (d: string | Date): string => new Date(d).toISOString().substring(0, 10)

/** A date as the local YYYY-MM-DDTHH:mm value a datetime-local input expects (toISOString would give UTC). */
export function toLocalDateTimeInput(d: string | Date = new Date()): string {
  const date = new Date(d)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

/** A "YYYY-MM" month key as a short month in the current language ("févr. 26"); anything else is returned untouched. */
export function formatMonthLabel(month: string): string {
  const m = /^(\d{4})-(\d{2})$/.exec(month)
  if (!m) return month
  return new Date(Date.UTC(Number(m[1]), Number(m[2]) - 1, 1)).toLocaleDateString(intlLocale(), { month: 'short', year: '2-digit', timeZone: 'UTC' })
}

/** Short date with the time in the current language, e.g. "02 mai, 08:30". */
export function formatDayTime(dateStr: string) {
  return new Date(dateStr).toLocaleDateString(intlLocale(), {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const day = (d: Date) => d.toLocaleDateString(intlLocale(), { day: '2-digit', month: 'short', year: 'numeric' })
const clock = (d: Date) => d.toLocaleTimeString(intlLocale(), { hour: '2-digit', minute: '2-digit' })

/** A session window: "02 mai 2026, 22:00 → 05:30", the end repeating its day only when it differs; just the start when the end is missing or not after it. */
export function formatChargeWindow(start: string, end?: string | null): string {
  const s = new Date(start)
  const head = `${day(s)}, ${clock(s)}`
  const e = end ? new Date(end) : null
  if (!e || Number.isNaN(e.getTime()) || e <= s) return head
  const sameDay = s.toDateString() === e.toDateString()
  return `${head} → ${sameDay ? '' : `${day(e)}, `}${clock(e)}`
}
