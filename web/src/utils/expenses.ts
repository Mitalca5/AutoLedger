import { intlLocale, t } from '@/i18n'

// A curated shortlist for the currency pickers (a vehicle's own currency, or a manual expense's
// foreign currency); the backend accepts any ISO-4217-shaped code, so this is a convenience.
export const CURRENCIES = ['EUR', 'USD', 'GBP', 'CHF', 'CAD', 'AUD', 'JPY']

/** Label of an expense category in the current language. */
export const categoryLabel = (category: string): string =>
  ['MAINTENANCE', 'REPAIR', 'INSURANCE', 'SUBSCRIPTION', 'TAX', 'FINANCING', 'ACCESSORY', 'OTHER'].includes(category)
    ? t(`expenses.categories.${category}`)
    : category

export interface ReminderPreset {
  title: string
  category: string
  interval_km: number | ''
  interval_months: number | ''
  scheduled_month?: number
  scheduled_day?: number
  lead_km: number
  lead_days: number
}

export const reminderPresets = (): ReminderPreset[] => [
  {
    title: t('expenses.presets.tireRotation'),
    category: 'TIRES',
    interval_km: 10000,
    interval_months: 12,
    lead_km: 1000,
    lead_days: 15,
  },
  {
    title: t('expenses.presets.cabinFilter'),
    category: 'MAINTENANCE',
    interval_km: 40000,
    interval_months: 24,
    lead_km: 2000,
    lead_days: 30,
  },
  {
    title: t('expenses.presets.brakeFluid'),
    category: 'MAINTENANCE',
    interval_km: '',
    interval_months: 24,
    lead_km: 0,
    lead_days: 30,
  },
  {
    title: t('expenses.presets.inspection'),
    category: 'MAINTENANCE',
    interval_km: '',
    interval_months: 24,
    lead_km: 0,
    lead_days: 30,
  },
  {
    title: t('expenses.presets.wipers'),
    category: 'MAINTENANCE',
    interval_km: '',
    interval_months: 12,
    lead_km: 0,
    lead_days: 15,
  },
  {
    title: t('expenses.presets.calipers'),
    category: 'MAINTENANCE',
    interval_km: 20000,
    interval_months: 12,
    lead_km: 1000,
    lead_days: 15,
  },
  {
    title: t('expenses.presets.winterTires'),
    category: 'TIRES',
    interval_km: '',
    interval_months: '',
    scheduled_month: 11,
    scheduled_day: 1,
    lead_km: 0,
    lead_days: 15,
  },
  {
    title: t('expenses.presets.summerTires'),
    category: 'TIRES',
    interval_km: '',
    interval_months: '',
    scheduled_month: 4,
    scheduled_day: 1,
    lead_km: 0,
    lead_days: 15,
  },
]

/** The next occurrence of the given month/day on or after `from`, as YYYY-MM-DD. */
export function nextOccurrenceDate(month: number, day: number, from = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  const thisYear = from.getFullYear()
  const todayKey = `${thisYear}-${pad(from.getMonth() + 1)}-${pad(from.getDate())}`
  const candidate = `${thisYear}-${pad(month)}-${pad(day)}`
  return candidate >= todayKey ? candidate : `${thisYear + 1}-${pad(month)}-${pad(day)}`
}

/** datetime-local inputs expect local time, not UTC. */
// A reminder only has a due point when it carries a mileage or a calendar interval
export const hasReminderSchedule = (r: {
  interval_km?: number | null
  interval_months?: number | null
  scheduled_date?: string | null
}): boolean => (r.interval_km ?? 0) > 0 || (r.interval_months ?? 0) > 0 || !!r.scheduled_date

interface ReminderDays {
  remaining_days: number
  scheduled_date?: string | null
}

/** Whether the calendar due point has passed: a fixed date only does the day after, an interval already on its due day. */
export const reminderDaysOver = (r: ReminderDays): boolean => (r.scheduled_date ? r.remaining_days < 0 : r.remaining_days <= 0)

/** Remaining (or elapsed) calendar time of a reminder as a short phrase; a fixed date reached today reads "today". */
export const reminderDaysLabel = (r: ReminderDays): string =>
  r.scheduled_date && r.remaining_days === 0
    ? t('expenses.remindersPanel.dueToday')
    : reminderDaysOver(r)
      ? t('expenses.remindersPanel.daysOver', { days: Math.abs(r.remaining_days) })
      : t('expenses.remindersPanel.daysLeft', { days: r.remaining_days })

export function toLocalDateTimeInput(d: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** Currency fields of an expense payload: a foreign currency (anything but the vehicle's own) carries its rate to it. */
export function currencyPayload(form: { currency: string; fx_rate: string }, baseCurrency: string) {
  if (form.currency === baseCurrency) return { currency: baseCurrency, fx_rate: null }
  return { currency: form.currency, fx_rate: form.fx_rate ? Number(form.fx_rate) : null }
}

export function formatFileSize(bytes: number): string {
  const sizes = t('shell.appDropzone.byteUnits').split(',')
  if (!bytes || bytes <= 0) return `0 ${sizes[0]}`
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

export function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString(intlLocale(), {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  })
}

/** A calendar day stored as midnight UTC, shown as that same day whatever the timezone; the year is optional. */
export function formatCalendarDay(dateStr: string, withYear = true) {
  const text = new Date(dateStr.substring(0, 10)).toLocaleDateString(intlLocale(), {
    day: 'numeric',
    month: 'long',
    ...(withYear ? { year: 'numeric' } : {}),
    timeZone: 'UTC',
  })
  return intlLocale().startsWith('fr') ? text.replace(/^1 /, '1er ') : text
}

/** Drives the user picked for a toll that are older than the drives the modal loaded. */
export const countUnlistedDrives = (selectedIds: string[], listedDrives: { id: string }[]): number =>
  selectedIds.filter((id) => !listedDrives.some((d) => d.id === id)).length

/** The earlier maintenance a new one can close: amortized, dated on or before the form date, not the one being edited. */
export function findCloseCandidate(maintenances: any[] | null | undefined, editingId: string | null, formDate: string) {
  if (!maintenances || !maintenances.length) return null
  return (
    maintenances.find(
      (m) =>
        m.id !== editingId &&
        m.amortization_mode &&
        m.amortization_mode !== 'NONE' &&
        new Date(m.date).toISOString().substring(0, 10) <= formDate
    ) || null
  )
}

interface MaintenanceLike {
  category: string
  date: string
  amount: number | string
  currency?: string | null
  fx_rate?: number | string | null
}

export interface MaintenanceFilter {
  category: string
  year: string
}

/** Maintenance and fixed expenses matching a category and a year; an empty value means "any". */
export function filterMaintenance<T extends MaintenanceLike>(items: T[], filter: MaintenanceFilter): T[] {
  return items.filter(
    (m) => (!filter.category || m.category === filter.category) && (!filter.year || new Date(m.date).getUTCFullYear() === Number(filter.year))
  )
}

/** Years that hold at least one expense, most recent first. */
export function maintenanceYears(items: MaintenanceLike[]): number[] {
  return [...new Set(items.map((m) => new Date(m.date).getUTCFullYear()))].sort((a, b) => b - a)
}

/** Sum of the expenses in the vehicle's currency; a foreign-currency amount is converted with its own rate. */
export function maintenanceTotal(items: MaintenanceLike[], baseCurrency: string): number {
  return items.reduce((sum, m) => {
    const amount = Number(m.amount) || 0
    if (!m.currency || m.currency === baseCurrency) return sum + amount
    return sum + amount * (Number(m.fx_rate) || 0)
  }, 0)
}

/** How a new expense of a category weighs on the cost per km: only a lasting purchase is spread over the distance. */
export function defaultAmortizationMode(category: string): string {
  return category === 'ACCESSORY' ? 'DISTANCE' : 'NONE'
}

/** The most recent distinct descriptions, to suggest when typing a new one. */
export function recentDescriptions(items: { description?: string | null; date: string }[], limit = 10): string[] {
  const seen = new Set<string>()
  for (const m of [...items].sort((a, b) => b.date.localeCompare(a.date))) {
    const text = m.description?.trim()
    if (text) seen.add(text)
    if (seen.size >= limit) break
  }
  return [...seen]
}

interface ChargeLike {
  date: string
  kwh_added: number
  cost: number | null
  currency?: string | null
  fx_rate?: number | string | null
}

export interface ChargeMonth<T extends ChargeLike> {
  key: string
  date: string
  charges: T[]
  kwh: number
  cost: number
  withoutCost: number
  /** Average price per kWh over the charges that have a cost; null when none does. */
  pricePerKwh: number | null
}

/** Charges grouped by calendar month (local time, as their dates are shown), in the order they come. Costs are in the vehicle's currency. */
export function groupChargesByMonth<T extends ChargeLike>(charges: T[], baseCurrency: string): ChargeMonth<T>[] {
  const months = new Map<string, ChargeMonth<T>>()
  const pricedKwh = new Map<string, number>()
  for (const c of charges) {
    const d = new Date(c.date)
    const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
    let month = months.get(key)
    if (!month) {
      month = { key, date: c.date, charges: [], kwh: 0, cost: 0, withoutCost: 0, pricePerKwh: null }
      months.set(key, month)
    }
    month.charges.push(c)
    month.kwh += Number(c.kwh_added) || 0
    if (c.cost === null) {
      month.withoutCost++
      continue
    }
    const base = !c.currency || c.currency === baseCurrency ? c.cost : c.cost * (Number(c.fx_rate) || 0)
    month.cost += base
    if (c.kwh_added > 0) pricedKwh.set(key, (pricedKwh.get(key) ?? 0) + c.kwh_added)
  }
  for (const month of months.values()) {
    const priced = pricedKwh.get(month.key) ?? 0
    month.pricePerKwh = priced > 0 ? month.cost / priced : null
  }
  return [...months.values()]
}
