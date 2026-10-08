import type { TariffPlan } from '@/services/api'

export interface BandForm {
  name: string
  rate: number | null
}

export interface RuleForm {
  days: number[]
  start: string
  end: string
  band: string
}

export interface PlanForm {
  name: string
  planType: 'FLAT' | 'BANDS'
  currency: string
  flatRate: number | null
  bands: BandForm[]
  rules: RuleForm[]
  defaultBand: string
  standingCharge: number | null
  validFrom: string
  validTo: string
  isDefault: boolean
}

/** Weekdays in the order a week is read, as the API numbers them (0 is Sunday). */
export const WEEK_ORDER = [1, 2, 3, 4, 5, 6, 0] as const

const HHMM = /^(?:[01]\d|2[0-3]):[0-5]\d$|^24:00$/

export function emptyPlanForm(currency: string): PlanForm {
  return {
    name: '',
    planType: 'BANDS',
    currency,
    flatRate: null,
    bands: [{ name: '', rate: null }],
    rules: [],
    defaultBand: '',
    standingCharge: null,
    validFrom: '',
    validTo: '',
    isDefault: false,
  }
}

export function planToForm(plan: TariffPlan): PlanForm {
  const flat = plan.plan_type === 'FLAT'
  const bands = (plan.bands ?? []).map((b) => ({ name: b.name, rate: b.rate_cents }))
  return {
    name: plan.name,
    planType: flat ? 'FLAT' : 'BANDS',
    currency: plan.currency,
    flatRate: plan.flat_rate_cents ?? null,
    bands: flat || bands.length ? bands : [{ name: '', rate: null }],
    rules: (plan.rules ?? []).map((r) => ({ days: [...(r.days ?? [])], start: r.start, end: r.end, band: r.band })),
    defaultBand: plan.default_band ?? '',
    standingCharge: plan.standing_charge_cents ?? null,
    validFrom: plan.valid_from ?? '',
    validTo: plan.valid_to ?? '',
    isDefault: plan.is_default,
  }
}

/** A new version of a tariff: same contract and prices, to be given its own validity dates. */
export function nextVersionForm(plan: TariffPlan): PlanForm {
  const form = planToForm(plan)
  form.validFrom = plan.valid_to ? addDays(plan.valid_to, 1) : ''
  form.validTo = ''
  form.isDefault = false
  return form
}

function addDays(day: string, n: number): string {
  const d = new Date(`${day}T00:00:00Z`)
  d.setUTCDate(d.getUTCDate() + n)
  return d.toISOString().slice(0, 10)
}

export type PlanProblem =
  | 'name'
  | 'flatRate'
  | 'bandsRequired'
  | 'bandName'
  | 'bandDuplicate'
  | 'bandRate'
  | 'defaultBand'
  | 'ruleBand'
  | 'ruleTime'
  | 'ruleDays'
  | 'validity'

/** The first thing the API would refuse in the form, or null. Mirrors the server's checks. */
export function planProblem(form: PlanForm): PlanProblem | null {
  if (!form.name.trim()) return 'name'
  if (form.planType === 'FLAT') {
    return form.flatRate === null || form.flatRate < 0 ? 'flatRate' : validityProblem(form)
  }
  if (form.bands.length === 0) return 'bandsRequired'
  const names = new Set<string>()
  for (const b of form.bands) {
    const name = b.name.trim()
    if (!name) return 'bandName'
    if (names.has(name)) return 'bandDuplicate'
    names.add(name)
    if (b.rate === null || b.rate < 0) return 'bandRate'
  }
  if (form.defaultBand && !names.has(form.defaultBand)) return 'defaultBand'
  for (const r of form.rules) {
    if (!names.has(r.band)) return 'ruleBand'
    if (!HHMM.test(r.start) || !HHMM.test(r.end)) return 'ruleTime'
    if (r.days.some((d) => !Number.isInteger(d) || d < 0 || d > 6)) return 'ruleDays'
  }
  return validityProblem(form)
}

function validityProblem(form: PlanForm): PlanProblem | null {
  return form.validFrom && form.validTo && form.validTo < form.validFrom ? 'validity' : null
}

/** The request body for the form; call it once planProblem is null. */
export function formToPayload(form: PlanForm): Partial<TariffPlan> {
  const base = {
    name: form.name.trim(),
    plan_type: form.planType,
    currency: form.currency,
    standing_charge_cents: form.standingCharge,
    valid_from: form.validFrom || null,
    valid_to: form.validTo || null,
    is_default: form.isDefault,
  }
  if (form.planType === 'FLAT') return { ...base, flat_rate_cents: form.flatRate }
  const bands = form.bands.map((b) => ({ name: b.name.trim(), rate_cents: b.rate ?? 0 }))
  return {
    ...base,
    bands,
    rules: form.rules.map((r) => ({ days: r.days.length === 7 ? [] : [...r.days].sort((a, b) => a - b), start: r.start, end: r.end, band: r.band })),
    default_band: form.defaultBand || bands[0]?.name || '',
  }
}

/** Renames a band in the rules and the default band that point at it, so a typo fix keeps the grid intact. */
export function renameBand(form: PlanForm, index: number, name: string): void {
  const old = form.bands[index]?.name
  if (old === undefined) return
  form.bands[index].name = name
  if (old === '') return
  for (const r of form.rules) if (r.band === old) r.band = name
  if (form.defaultBand === old) form.defaultBand = name
}

/** Removes a band and the rules that used it; the default band falls back to the first one left. */
export function removeBand(form: PlanForm, index: number): void {
  const [gone] = form.bands.splice(index, 1)
  if (!gone) return
  form.rules = form.rules.filter((r) => r.band !== gone.name)
  if (form.defaultBand === gone.name) form.defaultBand = ''
}

/** Plans that share a name are the versions of one tariff: grouped, newest validity first (the API already sorts them). */
export function groupVersions(plans: TariffPlan[]): { name: string; versions: TariffPlan[] }[] {
  const groups = new Map<string, { name: string; versions: TariffPlan[] }>()
  for (const p of plans) {
    const key = p.name.trim().toLowerCase()
    const g = groups.get(key)
    if (g) g.versions.push(p)
    else groups.set(key, { name: p.name, versions: [p] })
  }
  return [...groups.values()]
}
