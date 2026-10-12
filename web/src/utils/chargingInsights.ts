import type { PlaceCategory } from './chargingPlaces'

export interface PlaceStatsGroup {
  key: string
  name?: string
  category?: string
  sessions: number
  kwh_added: number
  cost: number
  priced_kwh: number
  price_per_kwh: number | null
  free_sessions: number
  unknown_sessions: number
  unknown_kwh: number
}

export interface PlaceStatsMonth {
  month: string
  kwh_added: number
  cost: number
  priced_kwh: number
  price_per_kwh: number | null
  groups: { key: string; kwh_added: number; cost: number; priced_kwh: number }[]
}

export interface HomeSavings {
  reason?: 'no_home' | 'no_away'
  home_price_per_kwh: number | null
  home_priced_kwh: number
  away_sessions: number
  away_priced_kwh: number
  away_cost: number
  hypothetical_cost: number
  savings: number
  unclassified_kwh: number
}

export interface PlaceStats {
  months: number
  summary: {
    sessions: number
    kwh_added: number
    cost: number
    priced_kwh: number
    price_per_kwh: number | null
    free_sessions: number
    unknown_sessions: number
    unknown_kwh: number
    zero_kwh_sessions: number
  }
  groups: PlaceStatsGroup[]
  series: PlaceStatsMonth[]
  home_savings: HomeSavings
}

export const PLACE_PERIODS = [6, 12, 24, 0] as const

export const UNCLASSIFIED_COLOR = '#64748b'
export const OUTSIDE_COLOR = '#38bdf8'
/** Category of the charges reporting a location linked to no place once a home place exists. */
export const OUTSIDE_CATEGORY = 'OUTSIDE'
const CATEGORY_COLORS: Record<PlaceCategory, string[]> = {
  HOME: ['#34d399', '#10b981', '#6ee7b7'],
  WORK: ['#60a5fa', '#3b82f6', '#93c5fd'],
  PUBLIC: ['#fbbf24', '#f59e0b', '#fcd34d'],
  OTHER: ['#a78bfa', '#8b5cf6', '#c4b5fd'],
}

/** A stable colour per group: its category's hue, a shade per place of that category. */
export function groupColors(groups: PlaceStatsGroup[]): Record<string, string> {
  const used: Record<string, number> = {}
  const out: Record<string, string> = {}
  for (const g of groups) {
    const category = g.category as PlaceCategory
    if (g.category === OUTSIDE_CATEGORY) {
      out[g.key] = OUTSIDE_COLOR
      continue
    }
    if (!g.key || !CATEGORY_COLORS[category]) {
      out[g.key] = UNCLASSIFIED_COLOR
      continue
    }
    const shades = CATEGORY_COLORS[category]
    const n = used[category] ?? 0
    used[category] = n + 1
    out[g.key] = shades[n % shades.length]
  }
  return out
}

/** Label of a group: the place name, the translated "outside", or the translated "unclassified". */
export function groupLabel(g: PlaceStatsGroup, unclassified: string, outside = ''): string {
  if (g.category === OUTSIDE_CATEGORY) return outside || unclassified
  return g.key ? g.name || unclassified : unclassified
}

export type BreakdownMode = 'energy' | 'cost'

/** Donut items of the groups by energy (kWh) or by recorded cost. */
export function donutItems(groups: PlaceStatsGroup[], mode: BreakdownMode, colors: Record<string, string>, unclassified: string, outside = '') {
  return groups.map((g) => ({
    label: groupLabel(g, unclassified, outside),
    color: colors[g.key],
    amount: mode === 'energy' ? g.kwh_added : g.cost,
  }))
}

/** One stacked-bar dataset per group, one value per month, in kWh or in recorded cost. */
export function monthlyDatasets(stats: PlaceStats, colors: Record<string, string>, unclassified: string, mode: BreakdownMode, outside = '') {
  return stats.groups.map((g) => ({
    key: g.key,
    label: groupLabel(g, unclassified, outside),
    color: colors[g.key],
    data: stats.series.map((m) => {
      const x = m.groups.find((y) => y.key === g.key)
      return x ? (mode === 'energy' ? x.kwh_added : x.cost) : 0
    }),
  }))
}

/** Average price per month; a month without any cost known is a gap, not a zero. */
export function monthlyPrices(stats: PlaceStats): (number | null)[] {
  return stats.series.map((m) => m.price_per_kwh)
}

export type SavingsView =
  | { state: 'none'; reasonKey: string }
  | { state: 'ok'; amount: number; better: boolean; awayKwh: number; awayCost: number; hypotheticalCost: number; homePrice: number; unclassifiedKwh: number }

/** What the "if everything were charged at home" figure shows. A negative saving means charging away was cheaper. */
export function savingsView(h: HomeSavings): SavingsView {
  if (h.reason === 'no_home' || h.home_price_per_kwh === null) {
    return { state: 'none', reasonKey: 'expenses.chargingInsights.savings.noHome' }
  }
  if (h.reason === 'no_away') {
    return { state: 'none', reasonKey: 'expenses.chargingInsights.savings.noAway' }
  }
  return {
    state: 'ok',
    amount: Math.abs(h.savings),
    better: h.savings >= 0,
    awayKwh: h.away_priced_kwh,
    awayCost: h.away_cost,
    hypotheticalCost: h.hypothetical_cost,
    homePrice: h.home_price_per_kwh,
    unclassifiedKwh: h.unclassified_kwh,
  }
}

/** Energy outside any classified place, as a share (0..1) of the period's energy. */
export function unclassifiedShare(stats: PlaceStats): number {
  const total = stats.summary.kwh_added
  if (total <= 0) return 0
  const un = stats.groups.find((g) => !g.key)
  return un ? un.kwh_added / total : 0
}

export interface PlaceCategoryRow {
  /** HOME, WORK, PUBLIC, OTHER, OUTSIDE, or '' for the charges no place could be assigned to. */
  category: string
  sessions: number
  kwh: number
  sharePct: number
  /** Cost over the priced energy only, as in the place view; null when no charge of the category has a cost. */
  pricePerKwh: number | null
  unknownSessions: number
}

/** The place groups folded by category, the busiest first; unclassified charges stay on their own row. */
export function placeCategoryRows(stats: PlaceStats): PlaceCategoryRow[] {
  const total = stats.summary.kwh_added
  const byCategory = new Map<string, { sessions: number; kwh: number; cost: number; pricedKwh: number; unknown: number }>()
  for (const g of stats.groups) {
    const category = g.category === OUTSIDE_CATEGORY ? OUTSIDE_CATEGORY : g.key ? g.category || 'OTHER' : ''
    const row = byCategory.get(category) ?? { sessions: 0, kwh: 0, cost: 0, pricedKwh: 0, unknown: 0 }
    row.sessions += g.sessions
    row.kwh += g.kwh_added
    row.cost += g.cost
    row.pricedKwh += g.priced_kwh
    row.unknown += g.unknown_sessions
    byCategory.set(category, row)
  }
  return [...byCategory.entries()]
    .map(([category, r]) => ({
      category,
      sessions: r.sessions,
      kwh: r.kwh,
      sharePct: total > 0 ? (r.kwh / total) * 100 : 0,
      pricePerKwh: r.pricedKwh > 0 ? r.cost / r.pricedKwh : null,
      unknownSessions: r.unknown,
    }))
    .filter((r) => r.kwh > 0)
    .sort((a, b) => b.kwh - a.kwh)
}
