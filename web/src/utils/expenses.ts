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
]

/** datetime-local inputs expect local time, not UTC. */
// A reminder only has a due point when it carries a mileage or a calendar interval
export const hasReminderSchedule = (r: { interval_km?: number | null; interval_months?: number | null }): boolean =>
  (r.interval_km ?? 0) > 0 || (r.interval_months ?? 0) > 0

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
